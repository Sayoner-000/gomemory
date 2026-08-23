package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

func correrAtajo(t *testing.T, deps *Deps, alias string, args ...string) (string, string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	err := runPinnedShortcut(deps, alias, args, &out, &errb)
	return out.String(), errb.String(), err
}

func sembrarUno(t *testing.T, deps *Deps, alias, contenido string) {
	t.Helper()
	doc, ok := domain.PinnedDocByAlias(alias)
	if !ok {
		t.Fatalf("alias %q no está en el catálogo", alias)
	}
	seeder, topics, ok := seedDeps(deps)
	if !ok {
		t.Fatal("el repositorio debe exponer siembra y consulta")
	}
	if _, err := usecases.ImportPinnedDoc(seeder, topics, deps.Project,
		doc.TopicKey, doc.Type, doc.Title, contenido); err != nil {
		t.Fatalf("sembrar %s: %v", alias, err)
	}
}

// FR-024: una sola invocación devuelve el documento vigente.
func TestConstitution_SirveLaVersionGuardada(t *testing.T) {
	deps := depsDocs(t)
	sembrarUno(t, deps, "constitution", "CONSTITUCIÓN DEL EQUIPO\nlínea dos\n")

	out, errb, err := correrAtajo(t, deps, "constitution")
	if err != nil {
		t.Fatalf("constitution: %v", err)
	}
	if out != "CONSTITUCIÓN DEL EQUIPO\nlínea dos\n" {
		t.Errorf("stdout = %q", out)
	}
	if errb != "" {
		t.Errorf("con el documento presente no debe advertirse nada: %q", errb)
	}
}

// FR-025: sin memoria, el aviso deja claro que lo mostrado no es del proyecto —
// y viaja por stderr para no contaminar una redirección.
func TestConstitution_SinMemoriaAdviertePorStderr(t *testing.T) {
	deps := depsDocs(t)

	out, errb, err := correrAtajo(t, deps, "constitution")
	if err != nil {
		// Sin plantilla embebida en el binario de test no hay nada que servir:
		// es un error legítimo y explícito, no una salida vacía silenciosa.
		if !strings.Contains(err.Error(), "no hay contenido") {
			t.Fatalf("error inesperado: %v", err)
		}
		return
	}
	if !strings.Contains(errb, "⚠️") {
		t.Errorf("debe advertirse que el contenido no viene de la memoria, stderr = %q", errb)
	}
	if strings.Contains(out, "⚠️") {
		t.Error("el aviso no puede ir a stdout: rompería `mem constitution > archivo.md`")
	}
}

// El atajo sirve la versión EDITADA, no la plantilla.
func TestConstitution_SirveLaVersionEditada(t *testing.T) {
	deps := depsDocs(t)
	sembrarUno(t, deps, "constitution", "ORIGINAL\n")
	sembrarUno(t, deps, "constitution", "EDITADA POR EL EQUIPO\n")

	out, _, err := correrAtajo(t, deps, "constitution")
	if err != nil {
		t.Fatalf("constitution: %v", err)
	}
	if out != "EDITADA POR EL EQUIPO\n" {
		t.Errorf("stdout = %q, esperaba la versión editada", out)
	}
}

// FR-026: --sync refleja en spec-kit cuando existe...
func TestConstitution_SyncEscribeEnSpeckit(t *testing.T) {
	deps := depsDocs(t)
	sembrarUno(t, deps, "constitution", "CONSTITUCIÓN VIGENTE\n")
	if err := os.MkdirAll(filepath.Join(deps.Root, ".specify"), 0o755); err != nil {
		t.Fatalf("preparar .specify: %v", err)
	}

	_, errb, err := correrAtajo(t, deps, "constitution", "--sync")
	if err != nil {
		t.Fatalf("--sync: %v", err)
	}
	datos, err := os.ReadFile(filepath.Join(deps.Root, ".specify", "memory", "constitution.md"))
	if err != nil {
		t.Fatalf("no se escribió el archivo de spec-kit: %v", err)
	}
	if string(datos) != "CONSTITUCIÓN VIGENTE\n" {
		t.Errorf("contenido sincronizado = %q", datos)
	}
	if !strings.Contains(errb, "actualizado") {
		t.Errorf("debe informarse la sincronización: %q", errb)
	}
}

// ...y NUNCA crea la estructura cuando el proyecto no usa spec-kit.
func TestConstitution_SyncNoCreaSpeckit(t *testing.T) {
	deps := depsDocs(t)
	sembrarUno(t, deps, "constitution", "CONSTITUCIÓN\n")

	_, errb, err := correrAtajo(t, deps, "constitution", "--sync")
	if err != nil {
		t.Fatalf("--sync: %v", err)
	}
	if _, err := os.Stat(filepath.Join(deps.Root, ".specify")); err == nil {
		t.Error("--sync creó .specify en un proyecto que no usa spec-kit")
	}
	if !strings.Contains(errb, "no usa spec-kit") {
		t.Errorf("debe informarse que no aplica: %q", errb)
	}
}

// El atajo de reglas comparte toda la maquinaria: si uno funciona, el otro
// también, y no hay una segunda resolución que pueda divergir.
func TestRules_AtajoEquivalente(t *testing.T) {
	deps := depsDocs(t)
	sembrarUno(t, deps, "rules", "REGLAS DEL EQUIPO\n")

	out, _, err := correrAtajo(t, deps, "rules")
	if err != nil {
		t.Fatalf("rules: %v", err)
	}
	if out != "REGLAS DEL EQUIPO\n" {
		t.Errorf("stdout = %q", out)
	}

	viaDocs, _, err := correrDocs(t, deps, "show", "rules")
	if err != nil {
		t.Fatalf("docs show: %v", err)
	}
	if viaDocs != out {
		t.Errorf("el atajo y `docs show` deben devolver lo mismo:\n%q\n%q", out, viaDocs)
	}
}
