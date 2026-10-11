package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

// Contratos de la feature 037 (cierre de auditoría defensiva) contra el
// binario real, con HOME y almacén aislados por prueba.

const (
	priv037 = "SECRETO_037"
	cred037 = "AKIAIOSFODNN7EXAMPLE" // clave de ejemplo de la documentación de AWS
)

type bench037 struct {
	t        *testing.T
	bin      string
	project  string
	dataHome string
	env      []string
}

func newBench037(t *testing.T) *bench037 {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	mustMkdir(t, project)
	if out, err := exec.Command("git", "-C", project, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	// Sin grafo externo: su refresco en segundo plano escribe en .memory
	// después de que mem termina y rompería la limpieza del TempDir.
	mustWrite(t, filepath.Join(project, ".memory", "settings.json"), `{"code_graph_disabled":true}`)
	env := append(os.Environ(),
		"HOME="+filepath.Join(root, "home"),
		"USERPROFILE="+filepath.Join(root, "home"),
		"GOMEMORY_DATA_HOME="+filepath.Join(root, "data"),
		"GOMEMORY_NO_UPDATE_CHECK=1",
	)
	if coverDir := os.Getenv("GOMEMORY_COVERAGE_DIR"); coverDir != "" {
		env = append(env, "GOCOVERDIR="+coverDir)
	}
	return &bench037{t: t, bin: buildLifecycleBinary(t), project: project, dataHome: filepath.Join(root, "data"), env: env}
}

// run ejecuta mem en el proyecto con stdin cerrado salvo que se indique.
func (b *bench037) run(stdin string, args ...string) (stdout, stderr string, code int) {
	b.t.Helper()
	cmd := exec.Command(b.bin, args...)
	cmd.Dir = b.project
	cmd.Env = b.env
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		code = exit.ExitCode()
	default:
		b.t.Fatalf("lanzar mem %v: %v", args, err)
	}
	return out.String(), errOut.String(), code
}

func assertNoProtected037(t *testing.T, where, got string) {
	t.Helper()
	for _, gone := range []string{priv037, cred037, "anidado"} {
		if strings.Contains(got, gone) {
			t.Fatalf("%s revela %q:\n%s", where, gone, got)
		}
	}
}

// TestAudit037_SessionEndProtegeResumen cubre US1.1 por consola: la respuesta
// inmediata de `session end -s` y el contexto posterior no revelan contenido
// protegido y conservan el texto público.
func TestAudit037_SessionEndProtegeResumen(t *testing.T) {
	b := newBench037(t)
	if _, errOut, code := b.run("", "session", "start"); code != 0 {
		t.Fatalf("session start: %d %s", code, errOut)
	}
	summary := "resumen público <private>" + priv037 + " <private>anidado</private></private> clave " + cred037 + " <private>sin cierre"
	out, errOut, code := b.run("", "session", "end", "-s", summary)
	if code != 0 {
		t.Fatalf("session end: %d %s", code, errOut)
	}
	assertNoProtected037(t, "session end stdout", out)
	assertNoProtected037(t, "session end stderr", errOut)

	ctx, _, code := b.run("", "context")
	if code != 0 {
		t.Fatalf("context: %d", code)
	}
	assertNoProtected037(t, "context", ctx)
	if !strings.Contains(ctx, "resumen público") {
		t.Fatalf("context perdió el resumen público:\n%s", ctx)
	}

	list, _, _ := b.run("", "session", "list")
	assertNoProtected037(t, "session list", list)
}

// dataDir devuelve GOMEMORY_DATA_HOME del banco.
func (b *bench037) dataDir() string { return b.dataHome }

// openStore abre la base del proyecto del banco directamente (el
// proceso de prueba tiene otro GOMEMORY_DATA_HOME, así que no usa Open).
func (b *bench037) openStore() (*sql.DB, string) {
	b.t.Helper()
	key := persistence.ProjectKey(b.project)
	path := filepath.Join(b.dataDir(), "projects", key, persistence.DbName)
	if _, err := os.Stat(path); err != nil {
		found, _ := filepath.Glob(filepath.Join(b.dataDir(), "projects", "*", "*"))
		b.t.Fatalf("base del banco no encontrada en %s: %v (hay %v)", path, err, found)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		b.t.Fatal(err)
	}
	b.t.Cleanup(func() { _ = db.Close() })
	return db, key
}

func sanitizeJSON(t *testing.T, out string) domain.SanitizeReport {
	t.Helper()
	var r domain.SanitizeReport
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("informe JSON inválido: %v\n%s", err, out)
	}
	return r
}

// TestAudit037_SanitizeHistorico cubre US2 sobre el binario real: vista
// previa sin efectos, aplicación que preserva identidad y fechas y limpia
// snapshots y contexto, segunda aplicación sin cambios, e interrupciones
// antes y después del commit que dejan pending_cleanup y salida 1 hasta un
// reintento satisfactorio.
func TestAudit037_SanitizeHistorico(t *testing.T) {
	b := newBench037(t)
	if _, errOut, code := b.run("", "session", "start"); code != 0 {
		t.Fatalf("session start: %d %s", code, errOut)
	}
	db, key := b.openStore()
	protected := "público <private>" + priv037 + "</private> " + cred037
	gz := gzipString(t, "salida <private>"+priv037+" sin cierre")
	for _, s := range []struct {
		q    string
		args []any
	}{
		{`INSERT INTO sessions (id, project, summary, created_at, ended_at) VALUES ('hist-s', ?, ?, '2025-01-01 00:00:00', '2025-01-01 02:00:00')`, []any{key, protected}},
		{`INSERT INTO memory_relations (id, project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at) VALUES (77, ?, 1, 2, 'related', 0.42, ?, '2024-04-04 04:04:04')`, []any{key, protected}},
		{`INSERT INTO channel_activity (project, agent, scope, kind, last_error, last_error_at) VALUES (?, 'opencode', 'user', 'turn_reminder', ?, '2025-03-03 00:00:00')`, []any{key, protected}},
		{`INSERT INTO compression_originals (ref, hash, project, content_gz, raw_bytes, content_type, compressor, created_at, last_access_at, expires_at) VALUES ('privref00001', 'hh', ?, ?, 10, 'log', 'log', 'x', 'x', '2099-01-01')`, []any{key, gz}},
		// Remanente de la regex anterior: se reporta, no se modifica, y no
		// debe impedir que el saneamiento complete.
		{`INSERT INTO memories (id, project, type, title, content) VALUES (900, ?, 'decision', 'remanente', 'texto Z</private> fin')`, []any{key}},
	} {
		if _, err := db.Exec(s.q, s.args...); err != nil {
			t.Fatalf("fixture %q: %v", s.q, err)
		}
	}
	backup := filepath.Join(b.dataDir(), "backups", key, "20250101-000000.000000000.json")
	mustWrite(t, backup, `{"version":2,"exported_at":"x","source_project":"p","memories":[],"relations":[{"ref_a":1,"ref_b":2,"relation":"related","confidence":0.4,"reasoning":"`+protected+`","created_at":"2024-01-01 00:00:00"}]}`)
	contextFile := filepath.Join(b.project, ".memory", "context.md")
	mustWrite(t, contextFile, "## Sesiones\n- "+protected+"\n")

	raw := func(q string) string {
		t.Helper()
		var v sql.NullString
		if err := db.QueryRow(q).Scan(&v); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return v.String
	}

	out, errOut, code := b.run("", "sanitize", "--json")
	if code != 0 {
		t.Fatalf("preview: %d %s", code, errOut)
	}
	assertNoProtected037(t, "preview", out+errOut)
	if r := sanitizeJSON(t, out); r.Status != domain.SanitizeStatusPreview || len(r.Categories) != 5 {
		t.Fatalf("preview: %+v", r)
	}
	if raw(`SELECT summary FROM sessions WHERE id = 'hist-s'`) != protected {
		t.Fatal("preview modificó filas")
	}

	// Interrupción ANTES del commit: otra conexión retiene la escritura.
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), `BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	out, errOut, code = b.run("", "sanitize", "--apply", "--json", "--timeout", "20s")
	if code != 1 || !strings.Contains(errOut, domain.SanitizeCodeBusy) {
		t.Fatalf("apply con escritor: code=%d stderr=%s", code, errOut)
	}
	if r := sanitizeJSON(t, out); r.Status != domain.SanitizeStatusPendingCleanup {
		t.Fatalf("apply con escritor: %+v", r)
	}
	_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
	_ = conn.Close()
	if raw(`SELECT summary FROM sessions WHERE id = 'hist-s'`) != protected {
		t.Fatal("apply interrumpido confirmó cambios")
	}

	// Interrupción DESPUÉS del commit: un lector impide el checkpoint.
	reader, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := reader.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	out, errOut, code = b.run("", "sanitize", "--apply", "--json", "--timeout", "20s")
	_ = reader.Rollback()
	if code != 1 || !strings.Contains(errOut, domain.SanitizeCodeBusy) {
		t.Fatalf("apply con lector: code=%d stderr=%s", code, errOut)
	}
	if r := sanitizeJSON(t, out); r.Status != domain.SanitizeStatusPendingCleanup || r.RowsChanged != 4 {
		t.Fatalf("apply con lector: %+v", r)
	}

	// Reintento sin clientes: completa y cierra los informes anteriores.
	_ = db.Close()
	out, errOut, code = b.run("", "sanitize", "--apply", "--json")
	if code != 0 {
		t.Fatalf("reintento: %d %s", code, errOut)
	}
	if r := sanitizeJSON(t, out); r.Status != domain.SanitizeStatusCompleted {
		t.Fatalf("reintento: %+v", r)
	}
	db, _ = b.openStore()
	assertNoProtected037(t, "summary", raw(`SELECT summary FROM sessions WHERE id = 'hist-s'`))
	assertNoProtected037(t, "reasoning", raw(`SELECT reasoning FROM memory_relations WHERE id = 77`))
	assertNoProtected037(t, "last_error", raw(`SELECT last_error FROM channel_activity WHERE kind = 'turn_reminder'`))
	if got := raw(`SELECT created_at || '|' || ended_at FROM sessions WHERE id = 'hist-s'`); got != "2025-01-01 00:00:00|2025-01-01 02:00:00" {
		t.Fatalf("fechas alteradas: %s", got)
	}
	if got := raw(`SELECT confidence || '|' || created_at FROM memory_relations WHERE id = 77`); got != "0.42|2024-04-04 04:04:04" {
		t.Fatalf("relación alterada: %s", got)
	}
	if raw(`SELECT COUNT(*) FROM compression_originals WHERE ref = 'privref00001'`) != "0" {
		t.Fatal("original protegido no eliminado")
	}
	for _, path := range []string{backup, contextFile} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		assertNoProtected037(t, filepath.Base(path), string(data))
	}
	reports, _ := filepath.Glob(filepath.Join(b.dataDir(), "maintenance", "privacy", "*.json"))
	if len(reports) != 3 {
		t.Fatalf("informes = %d, quiero 3", len(reports))
	}
	for _, p := range reports {
		data, _ := os.ReadFile(p)
		assertNoProtected037(t, "informe", string(data))
		if r := sanitizeJSON(t, string(data)); r.Status != domain.SanitizeStatusCompleted {
			t.Fatalf("informe %s quedó %s", filepath.Base(p), r.Status)
		}
	}

	out, errOut, code = b.run("", "sanitize", "--apply", "--json")
	if r := sanitizeJSON(t, out); code != 0 || r.RowsChanged != 0 || r.Status != domain.SanitizeStatusCompleted {
		t.Fatalf("segundo apply: code=%d %+v %s", code, r, errOut)
	}
}

func gzipString(t *testing.T, s string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestAudit037_WrapDistingueArranqueFallido cubre US6 (H-07) con stdin
// cerrado: un comando que no arranca sale 1 con WRAP_START_FAILED y sin
// diálogo; un hijo iniciado conserva su salida y su código.
func TestAudit037_WrapDistingueArranqueFallido(t *testing.T) {
	b := newBench037(t)
	noExec := filepath.Join(b.project, "sin-permiso.sh")
	mustWrite(t, noExec, "#!/bin/sh\necho no\n")
	if err := os.Chmod(noExec, 0o644); err != nil {
		t.Fatal(err)
	}
	const dialog = "¿Guardar?"

	for _, c := range []struct {
		name string
		args []string
	}{
		{"inexistente", []string{"wrap", "-s=false", "comando-inexistente-037", "--token=" + priv037}},
		{"sin permiso", []string{"wrap", "-s=false", noExec}},
	} {
		out, errOut, code := b.run("", c.args...)
		if code != 1 || !strings.Contains(errOut, "WRAP_START_FAILED") {
			t.Fatalf("%s: code=%d stderr=%q", c.name, code, errOut)
		}
		if strings.Contains(errOut, dialog) || strings.Contains(out+errOut, priv037) {
			t.Fatalf("%s: diálogo o argumentos crudos en la salida: %q", c.name, errOut)
		}
	}

	for _, c := range []struct {
		name     string
		script   string
		wantCode int
		wantOut  string
	}{
		{"hijo 0", "echo hola", 0, "hola"},
		{"hijo 7", "echo fallo >&2; exit 7", 7, ""},
		{"hijo 1", "exit 1", 1, ""},
	} {
		out, errOut, code := b.run("", "wrap", "-s=false", "sh", "-c", c.script)
		if code != c.wantCode || !strings.Contains(out, c.wantOut) {
			t.Fatalf("%s: code=%d stdout=%q", c.name, code, out)
		}
		if strings.Contains(errOut, "WRAP_START_FAILED") || !strings.Contains(errOut, dialog) {
			t.Fatalf("%s: un hijo iniciado conserva el diálogo y no es fallo de arranque: %q", c.name, errOut)
		}
	}

	if _, _, code := b.run("", "wrap", "-s=false", "sh", "-c", "kill -TERM $$"); code == 0 {
		t.Fatal("un hijo terminado por señal no puede convertirse en éxito")
	}
}
