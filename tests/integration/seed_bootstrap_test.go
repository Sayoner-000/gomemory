package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestSeedsAparecenSinInstall cubre FR-004 y el escenario §8 del quickstart.
//
// Desde v1.9 el MCP se registra en ámbito global y muchos proyectos no ejecutan
// `mem install` jamás. Si la siembra viviera solo en el instalador, la promesa
// de que las reglas y la constitución "se agregan solas" sería falsa para la
// mayoría de proyectos. Este test arranca el binario como lo haría un agente y
// comprueba que las semillas existen sin haber instalado nada.
func TestSeedsAparecenSinInstall(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t) // sin install, sin init, sin .memory previo

	arrancarMCPUnaVez(t, bin, target)

	ctxOut := runMem(t, bin, target, "context")

	if !strings.Contains(ctxOut, "## Reglas de trabajo (memoria fijada)") {
		t.Errorf("las reglas no llegaron al contexto sin instalación previa.\nSalida:\n%s", ctxOut)
	}
	// El preámbulo real termina con la sección de gestión de tareas: si aparece,
	// el contenido llegó entero y no recortado a 200 caracteres.
	if !strings.Contains(ctxOut, "Captura Lecciones") {
		t.Error("las reglas llegaron recortadas: falta el final del preámbulo")
	}

	// La constitución se siembra pero NO se emite íntegra: se consulta bajo
	// demanda, para no gastar 635 líneas de contexto en cada sesión.
	if !strings.Contains(ctxOut, "Constitución del proyecto") {
		t.Errorf("la semilla de constitución no aparece en el contexto.\nSalida:\n%s", ctxOut)
	}
}

// TestSiembraEsIdempotenteEntreArranques: repetir la operación no duplica ni
// reescribe. Es lo que permite sembrar en cada arranque del servidor MCP sin
// coste ni riesgo.
func TestSiembraEsIdempotenteEntreArranques(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)

	for i := 0; i < 3; i++ {
		arrancarMCPUnaVez(t, bin, target)
	}

	lista := runMem(t, bin, target, "list", "-n", "50")
	if n := strings.Count(lista, "Reglas de trabajo del proyecto"); n != 1 {
		t.Errorf("la semilla de reglas aparece %d veces tras 3 arranques; esperaba 1", n)
	}
	if n := strings.Count(lista, "Constitución del proyecto"); n != 1 {
		t.Errorf("la semilla de constitución aparece %d veces tras 3 arranques; esperaba 1", n)
	}
}

// arrancarMCPUnaVez levanta el servidor MCP, completa el handshake y lo cierra.
// Es la ruta real por la que un agente provoca la siembra sin haber ejecutado
// `mem install` nunca (FR-004).
func arrancarMCPUnaVez(t *testing.T, bin, dir string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.Command(bin, "mcp", "--root", dir)
	cmd.Dir = dir
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("arrancar mem mcp: %v", err)
	}
	if _, err := session.ListTools(ctx, nil); err != nil {
		t.Fatalf("handshake MCP: %v", err)
	}
	session.Close()
}

func runMem(t *testing.T, bin, dir string, args ...string) string {
	t.Helper()
	// --root no es un flag global: lo consume cada subcomando y `context` no lo
	// declara. La raíz se resuelve desde el directorio de trabajo.
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mem %s falló: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
