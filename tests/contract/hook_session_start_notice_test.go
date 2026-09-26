package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type sessionStartOutput struct {
	SystemMessage      string `json:"systemMessage"`
	HookSpecificOutput struct {
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

func runSessionStartClaude(t *testing.T, s *lifecycleSandbox, p string) (lifecycleResult, sessionStartOutput) {
	t.Helper()
	s.Setenv("CLAUDE_PROJECT_DIR", p)
	defer s.Setenv("CLAUDE_PROJECT_DIR", "")
	res := s.RunInput("", p, "{}", "hook", "session-start")
	var out sessionStartOutput
	if strings.HasPrefix(strings.TrimSpace(res.Stdout), "{") {
		if err := json.Unmarshal([]byte(res.Stdout), &out); err != nil {
			t.Fatalf("salida JSON inválida: %v\n%s", err, res.Stdout)
		}
	}
	return res, out
}

// FR-003, FR-004a: al iniciar sesión se retira la copia de gomemory del
// proyecto y la persona ve el aviso una sola vez.
func TestSessionStart_RetiraLaCopiaYAvisaUnaVez(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("p1")
	copia := filepath.Join(p, exeName("mem"))
	copyExecutable(t, buildLifecycleBinary(t), copia)

	res, out := runSessionStartClaude(t, s, p)
	if res.ExitCode != 0 {
		t.Fatalf("session-start: code=%d\n%s", res.ExitCode, res.Stderr)
	}
	if _, err := os.Stat(copia); !os.IsNotExist(err) {
		t.Fatal("la copia debía retirarse")
	}
	// FindRoot resuelve enlaces (en macOS /var → /private/var).
	realP, _ := filepath.EvalSymlinks(p)
	if !strings.Contains(out.SystemMessage, "Se retiró "+filepath.Join(realP, exeName("mem"))) ||
		!strings.Contains(out.SystemMessage, s.GlobalBin) {
		t.Errorf("aviso de retirada inesperado: %q", out.SystemMessage)
	}

	_, out2 := runSessionStartClaude(t, s, p)
	if strings.Contains(out2.SystemMessage, "Se retiró") {
		t.Errorf("el aviso solo se muestra una vez: %q", out2.SystemMessage)
	}
}

// FR-004: si la retirada falla, el hook no se rompe ni se retrasa.
func TestSessionStart_RetiradaFallidaNoRompeElHook(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permisos de directorio de Unix")
	}
	s := newLifecycleSandbox(t)
	p := s.Project("p2")
	copia := filepath.Join(p, exeName("mem"))
	copyExecutable(t, buildLifecycleBinary(t), copia)
	// Inicializa .memory antes de bloquear la escritura en el proyecto.
	if r := s.RunInput("", p, "{}", "hook", "session-start"); r.ExitCode != 0 {
		t.Fatalf("preparación: %s", r.Stderr)
	}
	copyExecutable(t, buildLifecycleBinary(t), copia)
	if err := os.Chmod(p, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o755) })

	res, out := runSessionStartClaude(t, s, p)
	if res.ExitCode != 0 {
		t.Fatalf("el hook debe terminar con 0 aunque la retirada falle: %d\n%s", res.ExitCode, res.Stderr)
	}
	if _, err := os.Stat(copia); err != nil {
		t.Fatal("la copia sigue (no se pudo retirar)")
	}
	if strings.Contains(out.SystemMessage, "Se retiró") {
		t.Errorf("no puede anunciarse una retirada que no ocurrió: %q", out.SystemMessage)
	}
}

// Contrato: sin avisos, session-start sigue emitiendo el contexto en texto
// plano, también en Claude Code.
func TestSessionStart_SinAvisosTextoPlano(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("p3")
	res, _ := runSessionStartClaude(t, s, p)
	if res.ExitCode != 0 || strings.HasPrefix(strings.TrimSpace(res.Stdout), "{") {
		t.Fatalf("sin avisos la salida no cambia de formato: code=%d\n%s", res.ExitCode, res.Stdout)
	}
}

// writeUpdateCache siembra la caché de versiones del sandbox.
func writeUpdateCache(t *testing.T, s *lifecycleSandbox, latest string, checkedAt time.Time) string {
	t.Helper()
	path := filepath.Join(s.Data, "update-check.json")
	mustWrite(t, path, fmt.Sprintf(`{"latest":%q,"checked_at":%q,"etag":""}`, latest, checkedAt.UTC().Format(time.RFC3339)))
	return path
}

// apiServer cuenta las consultas; delay simula una API lenta o inalcanzable.
func apiServer(t *testing.T, tag string, delay time.Duration) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
		if _, err := fmt.Fprintf(w, `{"tag_name":%q}`, tag); err != nil {
			t.Errorf("escribir release: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// FR-030: con una versión nueva en caché, la persona la ve una vez por sesión.
func TestSessionStart_AvisoDeVersionUnaVez(t *testing.T) {
	s := newLifecycleSandbox(t)
	srv, _ := apiServer(t, "v99.0.0", 0)
	s.Setenv("GOMEMORY_NO_UPDATE_CHECK", "")
	s.Setenv("GOMEMORY_RELEASE_API_BASE", srv.URL)
	p := s.Project("p1")
	writeUpdateCache(t, s, "v99.0.0", time.Now())

	_, out := runSessionStartClaude(t, s, p)
	if !strings.Contains(out.SystemMessage, "gomemory v99.0.0 disponible") || !strings.Contains(out.SystemMessage, "mem update") {
		t.Fatalf("falta el aviso: %q", out.SystemMessage)
	}
	_, out2 := runSessionStartClaude(t, s, p)
	if strings.Contains(out2.SystemMessage, "disponible") {
		t.Errorf("en la misma sesión no se repite: %q", out2.SystemMessage)
	}
}

// FR-031, SC-008: desactivado no hay aviso, ni consulta, ni escritura.
func TestSessionStart_AvisoDesactivadoSinRed(t *testing.T) {
	for _, via := range []string{"env", "ajuste"} {
		t.Run(via, func(t *testing.T) {
			s := newLifecycleSandbox(t)
			srv, hits := apiServer(t, "v99.0.0", 0)
			s.Setenv("GOMEMORY_RELEASE_API_BASE", srv.URL)
			p := s.Project("p1")
			if via == "env" {
				s.Setenv("GOMEMORY_NO_UPDATE_CHECK", "1")
			} else {
				s.Setenv("GOMEMORY_NO_UPDATE_CHECK", "")
				mustWrite(t, filepath.Join(p, ".memory", "settings.json"), `{"update_check_disabled":true}`)
			}
			cache := writeUpdateCache(t, s, "v99.0.0", time.Now().Add(-72*time.Hour))
			antes := readAll(t, cache)

			_, out := runSessionStartClaude(t, s, p)
			time.Sleep(1500 * time.Millisecond) // margen para un proceso de fondo
			if strings.Contains(out.SystemMessage, "disponible") {
				t.Errorf("desactivado no avisa: %q", out.SystemMessage)
			}
			if hits.Load() != 0 {
				t.Errorf("desactivado no consulta la red: %d consultas", hits.Load())
			}
			if readAll(t, cache) != antes {
				t.Error("desactivado no escribe la caché")
			}
		})
	}
}

// Una caché corrupta no produce aviso ni rompe el hook.
func TestSessionStart_CacheCorrupta(t *testing.T) {
	s := newLifecycleSandbox(t)
	srv, _ := apiServer(t, "v99.0.0", 0)
	s.Setenv("GOMEMORY_NO_UPDATE_CHECK", "")
	s.Setenv("GOMEMORY_RELEASE_API_BASE", srv.URL)
	p := s.Project("p1")
	mustWrite(t, filepath.Join(s.Data, "update-check.json"), "{roto")
	res, out := runSessionStartClaude(t, s, p)
	if res.ExitCode != 0 || strings.Contains(out.SystemMessage, "disponible") {
		t.Fatalf("code=%d aviso=%q", res.ExitCode, out.SystemMessage)
	}
	// Una caché corrupta cuenta como vencida: se espera a que la consulta de
	// fondo la reescriba, para que no escriba durante la limpieza de TempDir
	// (memoria 140: la carrera del refresco en segundo plano).
	cache := filepath.Join(s.Data, "update-check.json")
	for deadline := time.Now().Add(8 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if strings.Contains(readAll(t, cache), "v99.0.0") {
			return
		}
	}
	t.Fatal("la consulta de fondo no reescribió la caché corrupta")
}

// FR-029, SC-006: con la caché vencida la consulta va en segundo plano; el
// hook no espera a una API lenta, y la caché se renueva después.
func TestSessionStart_ConsultaEnSegundoPlano(t *testing.T) {
	s := newLifecycleSandbox(t)
	srv, hits := apiServer(t, "v99.1.0", 1500*time.Millisecond)
	s.Setenv("GOMEMORY_NO_UPDATE_CHECK", "")
	s.Setenv("GOMEMORY_RELEASE_API_BASE", srv.URL)
	p := s.Project("p1")
	cache := writeUpdateCache(t, s, "v2.0.0", time.Now().Add(-72*time.Hour))

	start := time.Now()
	res, _ := runSessionStartClaude(t, s, p)
	if res.ExitCode != 0 {
		t.Fatalf("session-start: %d %s", res.ExitCode, res.Stderr)
	}
	if d := time.Since(start); d > 1200*time.Millisecond {
		t.Errorf("el hook esperó a la API: %v", d)
	}
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(readAll(t, cache), "v99.1.0") {
			if hits.Load() < 1 {
				t.Error("la caché cambió sin consultar")
			}
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("la consulta de fondo no renovó la caché:\n%s", readAll(t, cache))
}
