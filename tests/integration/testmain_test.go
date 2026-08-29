package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestMain sandboxea para toda la suite de este paquete las dos cosas que los
// tests escriben fuera del árbol del proyecto: el store global de gomemory y el
// directorio personal de quien ejecuta.
//
// El store (GOMEMORY_DATA_HOME) ya estaba aislado. El HOME no, y eso hacía que
// `go test ./...` modificara el `~/.codex/config.toml` REAL de quien corría la
// suite: varios tests lanzan el binario como subproceso —`exec.Command(bin,
// "install", target)`— fijando cmd.Dir pero no cmd.Env, así que el hijo heredaba
// el HOME del proceso de test. Un `t.Setenv("HOME", ...)` dentro de un test
// protege al código in-process, pero NO a un subproceso al que no se le pasa un
// Env explícito.
//
// No es que el binario se portara mal: `mem install` cablea Codex en ámbito de
// usuario a propósito, porque Codex no tiene equivalente por proyecto (ver
// setupCodex). El defecto era ejercer esa ruta contra el HOME de verdad.
//
// Se aísla aquí y no en cada llamada por la misma razón que el store: son más
// de cuarenta puntos de lanzamiento y basta que uno nuevo se olvide para que la
// contaminación vuelva sin que nada avise.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "gomemory-test-integration-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("GOMEMORY_DATA_HOME", dir)

	home, err := os.MkdirTemp("", "gomemory-test-home-integration-*")
	if err != nil {
		panic(err)
	}
	anclarCachesDeGo()
	os.Setenv("HOME", home)
	os.Setenv("USERPROFILE", home) // Windows

	code := m.Run()

	os.RemoveAll(dir)
	os.RemoveAll(home)
	os.Exit(code)
}

// anclarCachesDeGo fija GOCACHE y GOMODCACHE a sus rutas reales ANTES de mover
// el HOME. El toolchain las deriva del directorio personal, así que sin esto
// los tests que compilan el binario lo harían contra cachés vacías dentro del
// HOME temporal: cada ejecución recompilaría el mundo y borraría el resultado
// al terminar. Best-effort — si `go env` no responde, se sigue igual.
func anclarCachesDeGo() {
	out, err := exec.Command("go", "env", "GOCACHE", "GOMODCACHE").Output()
	if err != nil {
		return
	}
	valores := strings.Fields(strings.TrimSpace(string(out)))
	if len(valores) != 2 {
		return
	}
	os.Setenv("GOCACHE", valores[0])
	os.Setenv("GOMODCACHE", valores[1])
}
