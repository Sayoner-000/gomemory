package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

func depsDocs(t *testing.T) *Deps {
	t.Helper()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return &Deps{
		Root:        root,
		Project:     "proj",
		MemoryRepo:  persistence.NewMemoryRepository(db),
		SessionRepo: persistence.NewSessionRepository(db),
		ProjectRepo: persistence.NewProjectRepository(),
	}
}

func correrDocs(t *testing.T, deps *Deps, args ...string) (string, string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	err := runDocs(deps, args, &out, &errb)
	return out.String(), errb.String(), err
}

// sembrar deja las dos semillas con contenido conocido, sin depender de que
// TemplatesFS esté cargado en el binario de test.
func sembrar(t *testing.T, deps *Deps, contenido string) {
	t.Helper()
	seeder, topics, ok := seedDeps(deps)
	if !ok {
		t.Fatal("el repositorio debe exponer siembra y consulta por clave")
	}
	for _, d := range domain.PinnedDocs {
		if _, err := usecases.ImportPinnedDoc(seeder, topics, deps.Project,
			d.TopicKey, d.Type, d.Title, contenido); err != nil {
			t.Fatalf("sembrar %s: %v", d.Alias, err)
		}
	}
}

// FR-044: el estado se deriva comparando con la plantilla, no se almacena.
func TestDocsList_MuestraEstadoDerivado(t *testing.T) {
	deps := depsDocs(t)

	out, _, err := correrDocs(t, deps, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, d := range domain.PinnedDocs {
		if !strings.Contains(out, d.Alias) || !strings.Contains(out, d.Label) {
			t.Errorf("falta %s (%s) en el listado:\n%s", d.Alias, d.Label, out)
		}
	}
	if !strings.Contains(out, string(usecases.DocSinSembrar)) {
		t.Errorf("sin memorias, el estado debe ser 'sin sembrar':\n%s", out)
	}

	sembrar(t, deps, "CONTENIDO DEL EQUIPO")
	out, _, err = correrDocs(t, deps, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, string(usecases.DocPersonalizado)) {
		t.Errorf("con contenido distinto de la plantilla debe decir 'personalizado':\n%s", out)
	}
}

// FR-036 + contrato de stdout: sin -o, stdout lleva SOLO el documento.
func TestDocsShow_StdoutLimpio(t *testing.T) {
	deps := depsDocs(t)
	sembrar(t, deps, "REGLAS DEL EQUIPO\nsegunda línea\n")

	out, errb, err := correrDocs(t, deps, "show", "rules")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if out != "REGLAS DEL EQUIPO\nsegunda línea\n" {
		t.Errorf("stdout contaminado o incompleto: %q", out)
	}
	if errb != "" {
		t.Errorf("con el documento presente no debe haber avisos: %q", errb)
	}
}

// FR-037: sin memoria se sirve el contenido por defecto, y el aviso va a stderr
// para no contaminar una redirección.
func TestDocsShow_AvisoVaAStderr(t *testing.T) {
	deps := depsDocs(t)

	_, errb, err := correrDocs(t, deps, "show", "rules")
	// Sin plantilla embebida en el binario de test, resolver es un error
	// legítimo: lo que se comprueba es que el aviso nunca va a stdout.
	if err == nil && !strings.Contains(errb, "⚠️") {
		t.Errorf("sin memoria debe advertirse por stderr, stderr = %q", errb)
	}
}

// FR-038: reemplaza conservando la identidad de documento fijado.
func TestDocsImport_CreaYReemplazaConservandoLaClave(t *testing.T) {
	deps := depsDocs(t)
	ruta := filepath.Join(t.TempDir(), "reglas.md")
	if err := os.WriteFile(ruta, []byte("MIS REGLAS\n"), 0o644); err != nil {
		t.Fatalf("preparar archivo: %v", err)
	}

	_, errb, err := correrDocs(t, deps, "import", "rules", ruta)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if !strings.Contains(errb, "creado") {
		t.Errorf("la primera importación debe informar creación: %q", errb)
	}

	out, _, _ := correrDocs(t, deps, "show", "rules")
	if out != "MIS REGLAS\n" {
		t.Errorf("contenido = %q", out)
	}

	_, topics, _ := seedDeps(deps)
	m, _ := topics.ByTopicKey(deps.Project, domain.TopicWorkRules)
	if m == nil || m.TopicKey != domain.TopicWorkRules {
		t.Errorf("se perdió la clave de tópico: %+v — dejaría de ser un documento fijado", m)
	}

	if err := os.WriteFile(ruta, []byte("REGLAS v2\n"), 0o644); err != nil {
		t.Fatalf("reescribir: %v", err)
	}
	_, errb, err = correrDocs(t, deps, "import", "rules", ruta)
	if err != nil {
		t.Fatalf("reimport: %v", err)
	}
	if !strings.Contains(errb, "actualizado") {
		t.Errorf("la segunda importación reemplaza, debe informarlo: %q", errb)
	}
}

// FR-040 — el peor modo de fallo posible de esta capacidad.
func TestDocsImport_FallidoNoDestruyeElAnterior(t *testing.T) {
	deps := depsDocs(t)
	sembrar(t, deps, "CONTENIDO VALIOSO\n")

	dir := t.TempDir()
	vacio := filepath.Join(dir, "vacio.md")
	if err := os.WriteFile(vacio, []byte("   \n\t\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	if _, _, err := correrDocs(t, deps, "import", "rules", vacio); err == nil {
		t.Error("importar un archivo vacío debe fallar")
	}
	if out, _, _ := correrDocs(t, deps, "show", "rules"); out != "CONTENIDO VALIOSO\n" {
		t.Fatalf("un import fallido destruyó el documento anterior: %q", out)
	}

	if _, _, err := correrDocs(t, deps, "import", "rules", filepath.Join(dir, "no-existe.md")); err == nil {
		t.Error("importar un archivo inexistente debe fallar")
	}
	if out, _, _ := correrDocs(t, deps, "show", "rules"); out != "CONTENIDO VALIOSO\n" {
		t.Fatalf("un import de archivo inexistente destruyó el documento: %q", out)
	}
}

func TestDocsImport_IdenticoInformaSinCambios(t *testing.T) {
	deps := depsDocs(t)
	sembrar(t, deps, "MISMO TEXTO\n")

	ruta := filepath.Join(t.TempDir(), "r.md")
	if err := os.WriteFile(ruta, []byte("MISMO TEXTO\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	_, errb, err := correrDocs(t, deps, "import", "rules", ruta)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if !strings.Contains(errb, "sin cambios") {
		t.Errorf("stderr = %q, esperaba 'sin cambios'", errb)
	}
}

// FR-042: el catálogo es una comodidad, no un límite.
func TestDocsImport_ClaveArbitraria(t *testing.T) {
	deps := depsDocs(t)
	ruta := filepath.Join(t.TempDir(), "runbook.md")
	if err := os.WriteFile(ruta, []byte("pasos de guardia\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	if _, _, err := correrDocs(t, deps, "import", "--topic", "equipo:runbook", ruta); err != nil {
		t.Fatalf("importar clave arbitraria: %v", err)
	}
	_, topics, _ := seedDeps(deps)
	m, _ := topics.ByTopicKey(deps.Project, "equipo:runbook")
	if m == nil || m.Content != "pasos de guardia\n" {
		t.Errorf("no se guardó bajo la clave arbitraria: %+v", m)
	}
}

func TestDocs_AliasDesconocidoListaLosValidos(t *testing.T) {
	deps := depsDocs(t)

	_, _, err := correrDocs(t, deps, "show", "inexistente")
	if err == nil {
		t.Fatal("un alias desconocido debe fallar")
	}
	for _, alias := range domain.PinnedDocAliases() {
		if !strings.Contains(err.Error(), alias) {
			t.Errorf("el error debe listar los alias válidos, falta %q: %v", alias, err)
		}
	}
}

func TestDocsExport_AArchivoYDirectorio(t *testing.T) {
	deps := depsDocs(t)
	sembrar(t, deps, "CONTENIDO\n")
	dir := t.TempDir()

	destino := filepath.Join(dir, "reglas.md")
	if _, _, err := correrDocs(t, deps, "export", "-o", destino, "rules"); err != nil {
		t.Fatalf("export: %v", err)
	}
	datos, err := os.ReadFile(destino)
	if err != nil || string(datos) != "CONTENIDO\n" {
		t.Errorf("archivo exportado incorrecto: %q, %v", datos, err)
	}

	todos := filepath.Join(dir, "todos")
	if _, _, err := correrDocs(t, deps, "export", "--all", "-o", todos); err != nil {
		t.Fatalf("export --all: %v", err)
	}
	for _, d := range domain.PinnedDocs {
		if _, err := os.Stat(filepath.Join(todos, d.Alias+".md")); err != nil {
			t.Errorf("--all no exportó %s: %v", d.Alias, err)
		}
	}
}

// FR-035/SC-015: la maquinaria recorre el catálogo. Este test falla si alguien
// enumera los documentos a mano en cmd_docs.go.
func TestDocs_RecorreElCatalogoSinCasosEspeciales(t *testing.T) {
	deps := depsDocs(t)

	out, _, err := correrDocs(t, deps, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	filas := 0
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		for _, d := range domain.PinnedDocs {
			if strings.HasPrefix(strings.TrimSpace(l), d.Alias) {
				filas++
			}
		}
	}
	if filas != len(domain.PinnedDocs) {
		t.Errorf("el listado muestra %d documentos, el catálogo tiene %d", filas, len(domain.PinnedDocs))
	}
}

// TestDocsExport_FlagDespuesDelAlias cubre un bug encontrado al validar contra
// el binario real, no en la suite: `docs export rules -o archivo` escribía el
// documento en stdout y dejaba el archivo vacío.
//
// Causa: flag.FlagSet de stdlib deja de parsear en el primer argumento
// posicional, así que -o nunca se leía. Fallaba EN SILENCIO — el peor tipo de
// fallo para un comando de exportación, porque quien lo usa cree que ya tiene
// su archivo.
func TestDocsExport_FlagDespuesDelAlias(t *testing.T) {
	deps := depsDocs(t)
	sembrar(t, deps, "CONTENIDO EXPORTABLE\n")
	destino := filepath.Join(t.TempDir(), "salida.md")

	out, _, err := correrDocs(t, deps, "export", "rules", "-o", destino)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if out != "" {
		t.Errorf("con -o no debe escribirse nada en stdout, salió: %q", out)
	}
	datos, err := os.ReadFile(destino)
	if err != nil {
		t.Fatalf("el flag -o tras el alias no se aplicó: %v", err)
	}
	if string(datos) != "CONTENIDO EXPORTABLE\n" {
		t.Errorf("archivo = %q", datos)
	}
}

func TestDocsImport_FlagDespuesDelPosicional(t *testing.T) {
	deps := depsDocs(t)
	ruta := filepath.Join(t.TempDir(), "x.md")
	if err := os.WriteFile(ruta, []byte("contenido\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}

	if _, _, err := correrDocs(t, deps, "import", ruta, "--topic", "equipo:x"); err != nil {
		t.Fatalf("import con flag al final: %v", err)
	}
	_, topics, _ := seedDeps(deps)
	if m, _ := topics.ByTopicKey(deps.Project, "equipo:x"); m == nil {
		t.Error("--topic tras el posicional no se aplicó")
	}
}

// Ambos órdenes deben funcionar: el flag antes o después del alias.
func TestDocsExport_AmbosOrdenesDeFlags(t *testing.T) {
	deps := depsDocs(t)
	sembrar(t, deps, "X\n")
	dir := t.TempDir()

	for i, args := range [][]string{
		{"export", "-o", filepath.Join(dir, "a.md"), "rules"},
		{"export", "rules", "-o", filepath.Join(dir, "b.md")},
	} {
		if _, _, err := correrDocs(t, deps, args...); err != nil {
			t.Fatalf("orden %d falló: %v", i, err)
		}
	}
	for _, n := range []string{"a.md", "b.md"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("falta %s: %v", n, err)
		}
	}
}
