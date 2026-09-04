package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/domain"
)

func modeloDocs(t *testing.T) model {
	t.Helper()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return model{
		screen:       screenDocs,
		memRepo:      persistence.NewMemoryRepository(db),
		relRepo:      persistence.NewRelationRepository(db),
		settingsRepo: tuiSettingsStub{data: &ports.SettingsData{}},
		root:         root,
		project:      "proj",
		docPath:      textinput.New(),
		docTemplates: map[string]string{
			"agent-preamble.md":           "PLANTILLA DE REGLAS\n",
			"speckit-constitution-gen.md": "PLANTILLA DE CONSTITUCIÓN\n",
		},
		width:  100,
		height: 40,
		ready:  true,
	}
}

func teclaDocs(t *testing.T, m model, k string) model {
	t.Helper()
	next, _ := m.updateDocs(tea.KeyPressMsg{Code: rune(k[0]), Text: k})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("updateDocs no devolvió el modelo")
	}
	return got
}

// FR-035/SC-015: las filas se GENERAN del catálogo. Si alguien las enumera a
// mano, añadir un documento nuevo dejaría de reflejarse y este test lo detecta.
func TestConfigView_FilasDeDocumentosSeGeneranDelCatalogo(t *testing.T) {
	m := modeloDocs(t)
	m.screen = screenConfig

	vista := m.configView()
	for _, d := range domain.PinnedDocs {
		if !strings.Contains(vista, "Actualizar "+d.Label) {
			t.Errorf("falta la fila de %q en el menú de configuración:\n%s", d.Label, vista)
		}
	}
	if !strings.Contains(vista, string(docEstadoEsperadoSinSembrar)) {
		t.Errorf("las filas deben mostrar el estado del documento:\n%s", vista)
	}
}

// La convención de tui.go es explícita: las filas nuevas van al FINAL, para no
// invalidar las constantes que los tests existentes referencian por nombre.
func TestConfigRows_DocumentosVanAlFinal(t *testing.T) {
	if configRowDocsBase != configRowPlanGuard+1 {
		t.Errorf("las filas de documentos deben empezar justo después de configRowPlanGuard")
	}
	// Feature 027: la última fila del menú ya no es la de documentos sino el
	// interruptor de Octopus AAR, añadido al final según la misma convención.
	// La intención de la aserción no cambia: el menú se dimensiona por nombre,
	// nunca por literales.
	if configRowOctopus != configRowDocsBase+len(domain.PinnedDocs) {
		t.Errorf("configRowOctopus = %d, esperaba %d", configRowOctopus, configRowDocsBase+len(domain.PinnedDocs))
	}
	if configOptions != configRowOctopus+1 {
		t.Errorf("configOptions = %d, esperaba %d", configOptions, configRowOctopus+1)
	}
	if configRowAtomicPlan != 6 || configRowPlanGuard <= configRowAtomicPlan {
		t.Error("las filas preexistentes se desplazaron: los tests que las referencian por nombre quedarían inválidos")
	}
}

func TestDocsView_MuestraLasCuatroOperaciones(t *testing.T) {
	m := modeloDocs(t)

	vista := m.docsView()
	for _, op := range []string{"Ver contenido", "Exportar a archivo", "Importar desde archivo", "Restaurar contenido por defecto"} {
		if !strings.Contains(vista, op) {
			t.Errorf("falta la operación %q en la pantalla:\n%s", op, vista)
		}
	}
}

func TestDocsView_DesplazaContenidoLargo(t *testing.T) {
	lineas := make([]string, 60)
	for i := range lineas {
		lineas[i] = fmt.Sprintf("línea de documento %02d", i)
	}

	m := modeloDocs(t)
	m.height = 18
	m.docVista = strings.Join(lineas, "\n")

	primera := ansi.Strip(m.docsView())
	if !strings.Contains(primera, "línea de documento 00") || strings.Contains(primera, "línea de documento 59") {
		t.Fatalf("la vista inicial no está acotada al comienzo:\n%s", primera)
	}

	next, _ := m.updateDocs(keyMsg("G"))
	m = next.(model)
	ultima := ansi.Strip(m.docsView())
	if !strings.Contains(ultima, "línea de documento 59") || !strings.Contains(ultima, "más arriba") {
		t.Fatalf("end no desplazó el documento hasta el final:\n%s", ultima)
	}

	next, _ = m.updateDocs(keyMsg("g"))
	m = next.(model)
	if m.docScroll != 0 {
		t.Fatalf("g debe volver al inicio, docScroll=%d", m.docScroll)
	}
}

func TestDocs_ExportarEImportar(t *testing.T) {
	m := modeloDocs(t)
	dir := t.TempDir()
	destino := filepath.Join(dir, "reglas.md")

	// Exportar: con el documento sin sembrar, se exporta la plantilla.
	m = teclaDocs(t, m, "e")
	if m.docPendiente != docActionExportar {
		t.Fatal("la tecla e debe pedir una ruta de exportación")
	}
	m.docPath.SetValue(destino)
	next, _ := m.updateDocsRuta(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	m = next.(model)
	if m.docErr != "" {
		t.Fatalf("exportar falló: %s", m.docErr)
	}
	datos, err := os.ReadFile(destino)
	if err != nil || string(datos) != "PLANTILLA DE REGLAS\n" {
		t.Fatalf("archivo exportado incorrecto: %q, %v", datos, err)
	}

	// Editar e importar de vuelta.
	if err := os.WriteFile(destino, []byte("REGLAS DEL EQUIPO\n"), 0o644); err != nil {
		t.Fatalf("editar: %v", err)
	}
	m = teclaDocs(t, m, "i")
	m.docPath.SetValue(destino)
	next, _ = m.updateDocsRuta(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	m = next.(model)
	if m.docErr != "" {
		t.Fatalf("importar falló: %s", m.docErr)
	}

	topics, _ := m.docsTopics()
	got, _ := topics.ByTopicKey("proj", domain.TopicWorkRules)
	if got == nil || got.Content != "REGLAS DEL EQUIPO\n" {
		t.Errorf("el import no se guardó: %+v", got)
	}
	if st := m.docEstado(domain.PinnedDocs[0]); st.State != "personalizado" {
		t.Errorf("tras importar, el estado debe ser personalizado, es %q", st.State)
	}
}

// FR-040 en la TUI: un import fallido muestra el error sin perder el documento.
func TestDocs_ImportFallidoNoDestruyeNiSaleDeLaPantalla(t *testing.T) {
	m := modeloDocs(t)
	dir := t.TempDir()
	bueno := filepath.Join(dir, "bueno.md")
	if err := os.WriteFile(bueno, []byte("CONTENIDO VALIOSO\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	m = teclaDocs(t, m, "i")
	m.docPath.SetValue(bueno)
	next, _ := m.updateDocsRuta(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	m = next.(model)

	vacio := filepath.Join(dir, "vacio.md")
	if err := os.WriteFile(vacio, []byte("  \n"), 0o644); err != nil {
		t.Fatalf("preparar vacío: %v", err)
	}
	m = teclaDocs(t, m, "i")
	m.docPath.SetValue(vacio)
	next, _ = m.updateDocsRuta(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	m = next.(model)

	if m.docErr == "" {
		t.Error("un import vacío debe mostrar el motivo en pantalla")
	}
	if m.screen != screenDocs {
		t.Error("un import fallido no debe sacar de la pantalla")
	}
	topics, _ := m.docsTopics()
	got, _ := topics.ByTopicKey("proj", domain.TopicWorkRules)
	if got == nil || got.Content != "CONTENIDO VALIOSO\n" {
		t.Fatalf("el import fallido destruyó el documento anterior: %+v", got)
	}
}

// Restaurar descarta contenido: nunca sin confirmar.
func TestDocs_RestaurarPideConfirmacion(t *testing.T) {
	m := modeloDocs(t)
	dir := t.TempDir()
	ruta := filepath.Join(dir, "r.md")
	if err := os.WriteFile(ruta, []byte("PERSONALIZADO\n"), 0o644); err != nil {
		t.Fatalf("preparar: %v", err)
	}
	m = teclaDocs(t, m, "i")
	m.docPath.SetValue(ruta)
	next, _ := m.updateDocsRuta(tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"})
	m = next.(model)

	m = teclaDocs(t, m, "r")
	if !m.docConfirmReset {
		t.Fatal("restaurar debe pedir confirmación")
	}
	if !strings.Contains(m.docsView(), "¿Restaurar") {
		t.Error("la vista debe mostrar la pregunta de confirmación")
	}

	// Cancelar deja el contenido intacto.
	m = teclaDocs(t, m, "n")
	topics, _ := m.docsTopics()
	got, _ := topics.ByTopicKey("proj", domain.TopicWorkRules)
	if got.Content != "PERSONALIZADO\n" {
		t.Fatalf("cancelar la confirmación no debe restaurar: %q", got.Content)
	}

	// Confirmar sí restaura.
	m = teclaDocs(t, m, "r")
	m = teclaDocs(t, m, "s")
	got, _ = topics.ByTopicKey("proj", domain.TopicWorkRules)
	if got.Content != "PLANTILLA DE REGLAS\n" {
		t.Errorf("tras confirmar debe volver a la plantilla: %q", got.Content)
	}
}

const docEstadoEsperadoSinSembrar = "sin sembrar"
