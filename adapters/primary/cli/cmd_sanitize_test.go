package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

// fakeSanitizer registra la llamada y devuelve un informe/errores fijos.
type fakeSanitizer struct {
	calls    []string
	project  string
	deadline time.Duration
	report   domain.SanitizeReport
	err      error
}

func (f *fakeSanitizer) record(ctx context.Context, op, project string) (domain.SanitizeReport, error) {
	f.calls = append(f.calls, op)
	f.project = project
	if d, ok := ctx.Deadline(); ok {
		f.deadline = time.Until(d).Round(time.Second)
	}
	return f.report, f.err
}

func (f *fakeSanitizer) Preview(ctx context.Context, project string) (domain.SanitizeReport, error) {
	return f.record(ctx, "preview", project)
}

func (f *fakeSanitizer) Apply(ctx context.Context, project string) (domain.SanitizeReport, error) {
	return f.record(ctx, "apply", project)
}

func sanitizeDeps(s *fakeSanitizer) *Deps {
	return &Deps{ProjectRepo: &fakeProjectRepo{root: "/tmp/demo"}, Sanitizer: s}
}

// captureStderr redirige os.Stderr mientras corre fn.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		done <- sb.String()
	}()
	defer func() {
		os.Stderr = orig
	}()
	fn()
	_ = w.Close()
	return <-done
}

type sanitizeRun struct {
	code           int
	stdout, stderr string
}

func runSanitize(t *testing.T, deps *Deps, args ...string) sanitizeRun {
	t.Helper()
	var res sanitizeRun
	res.stderr = captureStderr(t, func() {
		res.stdout = captureStdout(t, func() {
			res.code = runExpectingExit(t, func() { CmdSanitize(deps, args) })
		})
	})
	return res
}

func previewReport() domain.SanitizeReport {
	r := domain.NewSanitizeReport("demo", domain.SanitizeModePreview)
	r.Categories = []domain.SanitizeCategoryCount{{Category: domain.SanitizeSession, AffectedCount: 1, RecordIDs: []string{"s1"}}}
	return r
}

// TestCmdSanitize_PreviewPorDefecto cubre FR-006: sin --apply no se aplica.
func TestCmdSanitize_PreviewPorDefecto(t *testing.T) {
	s := &fakeSanitizer{report: previewReport()}
	res := runSanitize(t, sanitizeDeps(s))
	if res.code > 0 || len(s.calls) != 1 || s.calls[0] != "preview" || s.project != "demo" {
		t.Fatalf("preview: code=%d calls=%v project=%q stderr=%s", res.code, s.calls, s.project, res.stderr)
	}
	if !strings.Contains(res.stdout, "session") || !strings.Contains(res.stdout, "mem sanitize --apply") {
		t.Fatalf("salida de vista previa:\n%s", res.stdout)
	}
	if s.deadline != 60*time.Second {
		t.Fatalf("timeout por defecto = %v, quiero 60s", s.deadline)
	}
}

// TestCmdSanitize_ApplyJSONUnObjeto cubre el contrato JSON: un único objeto
// en stdout, nada en stderr si termina bien, timeout respetado.
func TestCmdSanitize_ApplyJSONUnObjeto(t *testing.T) {
	r := previewReport()
	r.Mode, r.Status, r.RunID, r.RowsChanged = domain.SanitizeModeApply, domain.SanitizeStatusCompleted, "run-1", 1
	s := &fakeSanitizer{report: r}
	res := runSanitize(t, sanitizeDeps(s), "--apply", "--json", "--timeout", "2m")
	if res.code > 0 || len(s.calls) != 1 || s.calls[0] != "apply" || s.deadline != 2*time.Minute {
		t.Fatalf("apply: code=%d calls=%v deadline=%v", res.code, s.calls, s.deadline)
	}
	dec := json.NewDecoder(strings.NewReader(res.stdout))
	var got domain.SanitizeReport
	if err := dec.Decode(&got); err != nil || got.RunID != "run-1" || got.Status != domain.SanitizeStatusCompleted {
		t.Fatalf("JSON inválido: %v\n%s", err, res.stdout)
	}
	if dec.More() {
		t.Fatalf("stdout tiene más de un objeto:\n%s", res.stdout)
	}
	if strings.TrimSpace(res.stderr) != "" {
		t.Fatalf("stderr con éxito: %q", res.stderr)
	}
}

// TestCmdSanitize_ArgumentosInvalidosSalen2: no se inicia el saneamiento.
func TestCmdSanitize_ArgumentosInvalidosSalen2(t *testing.T) {
	for _, args := range [][]string{{"--timeout", "0s"}, {"--timeout", "-1m"}, {"--timeout", "abc"}, {"extra"}} {
		s := &fakeSanitizer{report: previewReport()}
		res := runSanitize(t, sanitizeDeps(s), args...)
		if res.code != 2 || len(s.calls) != 0 {
			t.Fatalf("%v: code=%d calls=%v", args, res.code, s.calls)
		}
	}
}

// TestCmdSanitize_PendienteSale1SinCausaCruda cubre los códigos de
// diagnóstico: busy/pending/timeout salen 1, con el informe en stdout y un
// diagnóstico depurado en stderr.
func TestCmdSanitize_PendienteSale1SinCausaCruda(t *testing.T) {
	for _, code := range []string{domain.SanitizeCodeBusy, domain.SanitizeCodePendingCleanup, domain.SanitizeCodeTimeout, domain.SanitizeCodeFailed} {
		r := previewReport()
		r.Mode, r.Status, r.RunID = domain.SanitizeModeApply, domain.SanitizeStatusPendingCleanup, "run-2"
		r.Pending = []domain.SanitizePending{{Code: code, Action: "Repite mem sanitize --apply."}}
		s := &fakeSanitizer{report: r, err: domain.NewSanitizeError(code, errors.New("driver dice SECRETO_037"))}
		res := runSanitize(t, sanitizeDeps(s), "--apply", "--json")
		if res.code != 1 {
			t.Fatalf("%s: code=%d", code, res.code)
		}
		var got domain.SanitizeReport
		if err := json.Unmarshal([]byte(res.stdout), &got); err != nil || got.Status != domain.SanitizeStatusPendingCleanup {
			t.Fatalf("%s: informe en stdout: %v\n%s", code, err, res.stdout)
		}
		if !strings.Contains(res.stderr, code) || strings.Contains(res.stderr+res.stdout, "SECRETO_037") {
			t.Fatalf("%s: diagnóstico: %q", code, res.stderr)
		}
	}
}

// TestCmdSanitize_SinServicioFalla: sin almacén disponible no hay éxito.
func TestCmdSanitize_SinServicioFalla(t *testing.T) {
	res := runSanitize(t, &Deps{ProjectRepo: &fakeProjectRepo{root: "/tmp/demo"}}, "--json")
	if res.code != 1 || !strings.Contains(res.stderr, domain.SanitizeCodeFailed) {
		t.Fatalf("sin Sanitizer: code=%d stderr=%q", res.code, res.stderr)
	}
}
