package cli

import (
	"errors"
	"os"
	"strings"
	"testing"

	"mem/application/ports"
)

// fakeSettingsRepo evita tocar disco para leer el gate de la feature.
type fakeSettingsRepo struct {
	atomicPlanDisabled bool
}

func (f fakeSettingsRepo) Read(string) ports.SettingsData {
	return ports.SettingsData{AtomicPlanDisabled: f.atomicPlanDisabled}
}
func (f fakeSettingsRepo) Write(string, ports.SettingsData) error { return nil }
func (f fakeSettingsRepo) ApplyAutoApprove(string, ports.SettingsData) {}

// planCtxStub es el doble del constructor de contexto para las pruebas del
// comando: permite ejercer la rama degradada sin necesidad de un proyecto sin
// memoria en disco.
type planCtxStub struct {
	out string
	err error
}

func (s planCtxStub) Build() (string, error) { return s.out, s.err }
func (s planCtxStub) WriteFile() error       { return nil }

// captureStdout ejecuta fn redirigiendo la salida estándar y devuelve lo escrito.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				sb.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		done <- sb.String()
	}()

	fn()

	w.Close()
	os.Stdout = orig
	return <-done
}

// TestCmdPlanContext_RamaCompleta verifica el camino feliz: método y contexto.
func TestCmdPlanContext_RamaCompleta(t *testing.T) {
	deps := &Deps{
		Root:           t.TempDir(),
		SettingsRepo:   fakeSettingsRepo{},
		ContextBuilder: planCtxStub{out: "# Memoria del Proyecto\n\nhistorial"},
	}
	SetPlanMethod("# Método\n\ncontenido del método")
	defer SetPlanMethod("")

	out := captureStdout(t, func() { CmdPlanContext(deps, nil) })

	if !strings.Contains(out, "contenido del método") {
		t.Error("falta el método en la salida")
	}
	if !strings.Contains(out, "historial") {
		t.Error("falta el contexto en la salida")
	}
}

// TestCmdPlanContext_RamaDegradada cubre FR-034: si el contexto falla, el
// comando NO debe abortar ni emitir error — debe entregar el método igual. Esta
// es la garantía de que un modo plan nunca queda interrumpido.
func TestCmdPlanContext_RamaDegradada_NoAborta(t *testing.T) {
	deps := &Deps{
		Root:           t.TempDir(),
		SettingsRepo:   fakeSettingsRepo{},
		ContextBuilder: planCtxStub{err: errors.New("memoria no inicializada")},
	}
	SetPlanMethod("# Método\n\ncontenido del método")
	defer SetPlanMethod("")

	out := captureStdout(t, func() { CmdPlanContext(deps, nil) })

	if !strings.Contains(out, "contenido del método") {
		t.Error("con el contexto caído el método debe seguir emitiéndose")
	}
	if strings.Contains(out, "historial") {
		t.Error("no debe emitirse contexto cuando su construcción falla")
	}
}

// TestCmdPlanContext_RamaSilenciada cubre FR-032: apagado ⇒ salida vacía.
func TestCmdPlanContext_RamaSilenciada_SalidaVacia(t *testing.T) {
	deps := &Deps{
		Root:           t.TempDir(),
		SettingsRepo:   fakeSettingsRepo{atomicPlanDisabled: true},
		ContextBuilder: planCtxStub{out: "# Memoria del Proyecto\n\nhistorial"},
	}
	SetPlanMethod("# Método\n\ncontenido del método")
	defer SetPlanMethod("")

	out := captureStdout(t, func() { CmdPlanContext(deps, nil) })

	if strings.TrimSpace(out) != "" {
		t.Errorf("con la funcionalidad apagada la salida debe ser vacía, se obtuvo %q", out)
	}
}
