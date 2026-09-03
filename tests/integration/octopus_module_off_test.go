package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// T030 — INV-AAR-019 y SC-001 extremo a extremo: con el módulo apagado, el
// sistema es INDISTINGUIBLE de uno sin esta funcionalidad.
//
// Se ejecuta contra el binario real y contra la base real, no contra funciones
// internas: "apagado" es una promesa sobre lo que el usuario observa, no sobre
// qué ramas del código se ejecutan.
func TestOctopusModuloApagado(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "mem-off-test")
	build := exec.Command("go", "build", "-o", bin, "./infrastructure")
	build.Dir = repoRootIntegration(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("compilar binario: %v\n%s", err, out)
	}

	// HOME propio: el store global del proceso hijo queda aislado, con una sola
	// carpeta de proyecto.
	env := storeAislado(t)

	dir := t.TempDir()
	memDir := filepath.Join(dir, ".memory")
	if err := os.MkdirAll(memDir, 0o700); err != nil {
		t.Fatalf("crear .memory: %v", err)
	}
	// Deliberadamente EXPLÍCITO en false, no ausente: ambos deben significar lo
	// mismo, y el caso explícito es el que un usuario escribiría al apagarlo.
	escribirArchivo(t, filepath.Join(memDir, "settings.json"), `{"octopus_enabled": false}`)

	correr := func(args ...string) (string, error) {
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	t.Run("los subcomandos responden desactivado y fallan", func(t *testing.T) {
		for _, sub := range []string{"route", "plan", "status", "usage", "history"} {
			out, err := correr("octopus", sub, "algo")
			if err == nil {
				t.Errorf("mem octopus %s debería terminar con código distinto de cero", sub)
			}
			if !strings.Contains(out, "desactivado") {
				t.Errorf("mem octopus %s debería decir que el módulo está desactivado:\n%s", sub, out)
			}
			// Y decir CÓMO activarlo: informar sin dar salida deja al usuario buscando.
			if !strings.Contains(out, "Configuración") {
				t.Errorf("mem octopus %s debería indicar cómo activarlo:\n%s", sub, out)
			}
		}
	})

	t.Run("ninguna emisión de contexto menciona Octopus", func(t *testing.T) {
		for _, args := range [][]string{
			{"context"},
			{"plan-context"},
			{"hook", "user-prompt-submit"},
		} {
			out, err := correr(args...)
			if err != nil {
				t.Fatalf("mem %s: %v\n%s", strings.Join(args, " "), err, out)
			}
			if strings.Contains(strings.ToLower(out), "octopus") {
				t.Errorf("mem %s menciona Octopus con el módulo apagado:\n%s", strings.Join(args, " "), out)
			}
			// Control: si la salida viniera vacía, la ausencia no probaría nada.
			if strings.TrimSpace(out) == "" {
				t.Errorf("mem %s no emitió nada: la prueba no está midiendo lo que cree", strings.Join(args, " "))
			}
		}
	})

	t.Run("la ayuda sí lo menciona: el interruptor debe ser descubrible", func(t *testing.T) {
		// Excepción deliberada y única. Si nada nombrara la funcionalidad, nadie
		// podría encontrarla para encenderla. La ayuda no viaja al contexto del
		// agente en cada sesión, que es lo que SC-001 protege.
		out, _ := correr("help")
		if !strings.Contains(strings.ToLower(out), "octopus") {
			t.Error("la ayuda debería nombrar el módulo para que sea descubrible")
		}
	})

	t.Run("no se escribe ninguna fila de telemetría", func(t *testing.T) {
		// Intentar usarlo por todas las vías antes de mirar la base.
		correr("octopus", "route", "investigar algo", "--class", "investigation")
		correr("octopus", "plan")
		correr("octopus", "status")

		db := baseDelStore(t, env)
		if db == nil {
			// La base ni siquiera existe. gomemory la crea en la primera
			// escritura, así que su ausencia demuestra que nada escribió: es la
			// evidencia más fuerte de huella cero, no un caso sin comprobar.
			return
		}
		defer db.Close()

		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM octopus_executions`).Scan(&n); err != nil {
			t.Fatalf("consultar la tabla: %v", err)
		}
		if n != 0 {
			t.Errorf("con el módulo apagado no debe escribirse ninguna fila, hay %d", n)
		}
	})

	t.Run("encender el módulo sí escribe: el control de la prueba anterior", func(t *testing.T) {
		// Sin esta comprobación, "cero filas" podría deberse a que la telemetría
		// no funciona en absoluto, no a que el módulo esté apagado.
		envEncendido := storeAislado(t)

		encendido := t.TempDir()
		memDir := filepath.Join(encendido, ".memory")
		if err := os.MkdirAll(memDir, 0o700); err != nil {
			t.Fatalf("crear .memory: %v", err)
		}
		escribirArchivo(t, filepath.Join(memDir, "settings.json"), `{"octopus_enabled": true}`)

		cmd := exec.Command(bin, "octopus", "route", "investigar la expiración",
			"--class", "investigation", "--read-only", "--context-tokens", "2200")
		cmd.Dir = encendido
		cmd.Env = envEncendido
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("octopus route con el módulo encendido: %v\n%s", err, out)
		}

		db := baseDelStore(t, envEncendido)
		if db == nil {
			t.Fatal("con el módulo encendido la decisión debería haberse registrado")
		}
		defer db.Close()

		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM octopus_executions`).Scan(&n); err != nil {
			t.Fatalf("consultar la tabla: %v", err)
		}
		if n == 0 {
			t.Error("con el módulo encendido debe registrarse la decisión")
		}
	})
}
