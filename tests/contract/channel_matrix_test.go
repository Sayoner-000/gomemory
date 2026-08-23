package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mem/adapters/primary/cli"
	"mem/adapters/secondary/persistence"
	"mem/domain"
)

// inventario lista los archivos bajo dir, en orden estable. Un directorio
// ausente produce inventario vacío, no error: es un estado válido.
func inventario(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		out = append(out, strings.TrimPrefix(p, dir))
		return nil
	})
	sort.Strings(out)
	return out
}

// TestC5_ActividadDeProyectoNoModificaElEntornoDeLaPersona es el contrato que
// faltaba.
//
// Durante el desarrollo de esta misma feature, añadir a la desinstalación una
// operación sobre una ruta derivada del directorio de la persona convirtió
// pruebas inofensivas en destructivas, y llegó a eliminar el complemento
// instalado en la máquina real de quien ejecutó la batería.
//
// El contrato ejerce la actividad sobre un proyecto temporal con el entorno de
// la persona redirigido, y falla si ese entorno resulta modificado.
func TestC5_ActividadDeProyectoNoModificaElEntornoDeLaPersona(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Artefactos de ámbito de usuario que la actividad NO debe tocar.
	for _, rel := range [][]string{
		{".config", "opencode", "plugins", "gomemory.ts"},
		{".claude", "settings.json"},
		{".codex", "config.toml"},
	} {
		ruta := filepath.Join(append([]string{home}, rel...)...)
		if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
			t.Fatalf("preparar %s: %v", ruta, err)
		}
		if err := os.WriteFile(ruta, []byte("contenido de la persona"), 0o644); err != nil {
			t.Fatalf("escribir %s: %v", ruta, err)
		}
	}

	antes := inventario(t, home)

	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "mem"), []byte("fake"), 0o755); err != nil {
		t.Fatalf("preparar binario falso: %v", err)
	}
	deps := &cli.Deps{ProjectRepo: persistence.NewProjectRepository()}
	cli.CmdUninstall(deps, []string{target, "--yes"})

	despues := inventario(t, home)

	if len(antes) != len(despues) {
		t.Fatalf("la desinstalación modificó el entorno de la persona:\nantes:   %v\ndespués: %v", antes, despues)
	}
	for i := range antes {
		if antes[i] != despues[i] {
			t.Errorf("el entorno de la persona cambió en %q", antes[i])
		}
	}
}

// TestC4_ActividadesDeclaranAlcance comprueba que toda actividad que escribe
// declara un ámbito, y que solo la de lectura recorre los dos.
func TestC4_ActividadesDeclaranAlcance(t *testing.T) {
	for _, act := range domain.LifecycleActivities {
		if act.ReadOnly {
			continue
		}
		if act.Scope != domain.ScopeProject && act.Scope != domain.ScopeUser {
			t.Errorf("la actividad %q escribe y no declara un ámbito válido", act.Name)
		}
	}
}

// TestC3_SimetriaDeInventario comprueba sobre la matriz que el conjunto de
// artefactos de proyecto que se escriben coincide con el que se retira.
func TestC3_SimetriaDeInventario(t *testing.T) {
	retiradas := map[string]bool{}
	for _, c := range domain.CellsForActivity(domain.ActivityUninstall) {
		retiradas[c.Key()] = true
	}
	for _, c := range domain.CellsForActivity(domain.ActivityInstall) {
		if !retiradas[c.Key()] {
			t.Errorf("%s: se instala y no se desinstala", c)
		}
	}
}

// TestC8_SeleccionDeAgentesRespetada cubre el contrato C8: una actividad que
// recibe una selección de agentes produce artefactos únicamente de esos.
//
// Antes de esta feature, pedir el registro de ámbito global de un agente dejaba
// tres archivos de otro. La cadena causal era un bucle que recorría el registro
// de capacidades pero fijaba un nombre de agente e ignoraba la selección
// recibida: creaba el directorio de ese agente, y una función posterior —que sí
// era simétrica y solo escribía donde el directorio ya existía— lo encontraba
// recién creado y añadía dos archivos más.
func TestC8_SeleccionDeAgentesRespetada(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cli.RunGlobalScopeSetupForTest([]string{"codex"})

	for _, ajeno := range []string{".claude", ".config/opencode"} {
		ruta := filepath.Join(home, filepath.FromSlash(ajeno))
		if _, err := os.Stat(ruta); err == nil {
			listado := inventario(t, ruta)
			t.Errorf("se pidió únicamente un agente y se creó %q con %d archivo(s): %v", ajeno, len(listado), listado)
		}
	}
}
