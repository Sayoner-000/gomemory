package usecases

import (
	"context"
	"mem/application/ports"
	"mem/domain"
	"testing"
)

type shieldReaderFake struct{ calls int }

func (f *shieldReaderFake) Fingerprint(context.Context) string { return "hash" }
func (f *shieldReaderFake) Files(context.Context, int, int) ([]ports.AgentConfigFile, []string) {
	f.calls++
	return []ports.AgentConfigFile{{Path: "settings.json", Content: []byte(`{"permissions":{"allow":["Bash"]}}`)}}, []string{"large.json"}
}

type shieldStateFake struct{ report domain.ShieldReport }

func (f *shieldStateFake) LastHash(context.Context) string { return f.report.Hash }
func (f *shieldStateFake) Save(_ context.Context, r domain.ShieldReport) error {
	f.report = r
	return nil
}
func (f *shieldStateFake) Last(context.Context) (domain.ShieldReport, bool) {
	return f.report, f.report.Hash != ""
}
func TestShieldOncePerFingerprint(t *testing.T) {
	r, s := &shieldReaderFake{}, &shieldStateFake{}
	runner := ShieldScanner{Reader: r, State: s}
	for i := range 2 {
		report, scanned, err := runner.Scan(context.Background(), domain.ExpandSignals{})
		if err != nil || scanned != (i == 0) || len(report.Findings) != 1 || len(report.Skipped) != 1 {
			t.Fatalf("%+v %t %v", report, scanned, err)
		}
	}
	if r.calls != 1 {
		t.Fatal(r.calls)
	}
}

type shieldRewrittenReader struct{}

func (shieldRewrittenReader) Fingerprint(context.Context) string { return "hash" }
func (shieldRewrittenReader) Files(context.Context, int, int) ([]ports.AgentConfigFile, []string) {
	return []ports.AgentConfigFile{{Path: "config.toml", Rewritten: true,
		Content: []byte("{\n\"sandbox_mode\":\"danger-full-access\",\n\"x\":\"ghp_" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"}")}}, nil
}

// H-05: sobre contenido transformado por el lector no se inventan líneas.
func TestShieldRewrittenContentHasNoLines(t *testing.T) {
	r := ShieldScan(context.Background(), shieldRewrittenReader{}, nil)
	if len(r.Findings) == 0 {
		t.Fatal("se esperaban hallazgos")
	}
	for _, f := range r.Findings {
		if f.Line != 0 {
			t.Errorf("línea %d sobre contenido reescrito: %+v", f.Line, f)
		}
	}
}

// H-03: el informe que se guarda y se devuelve ya va depurado.
func TestShieldScanSanitizesReport(t *testing.T) {
	s := &shieldStateFake{}
	r := ShieldScan(context.Background(), shieldEscReader{}, s)
	for _, f := range append(r.Findings, s.report.Findings...) {
		if containsControl(f.File) {
			t.Errorf("ruta sin depurar: %q", f.File)
		}
	}
	for _, p := range append(r.Skipped, s.report.Skipped...) {
		if containsControl(p) {
			t.Errorf("omitido sin depurar: %q", p)
		}
	}
}

type shieldEscReader struct{}

func (shieldEscReader) Fingerprint(context.Context) string { return "hash" }
func (shieldEscReader) Files(context.Context, int, int) ([]ports.AgentConfigFile, []string) {
	return []ports.AgentConfigFile{{Path: "a\x1b[31mb.json", Content: []byte(`{"permissions":{"allow":["Bash"]}}`)}}, []string{"x\x1b]0;t\x07.json"}
}

func containsControl(s string) bool {
	for _, r := range s {
		if r == 0x1b || r == 0x07 {
			return true
		}
	}
	return false
}
