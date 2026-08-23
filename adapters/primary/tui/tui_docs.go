package tui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// ─── Documentos fijados (feature 021, US5) ──────────────────────────────────
//
// Paridad con `mem docs`: ver, exportar, importar y restaurar. Las mismas
// cuatro operaciones por las dos superficies, porque sin una vía cómoda de
// reemplazo las plantillas que envía la herramienta dejarían de ser un punto de
// partida para convertirse en las normas del equipo por omisión.
//
// Todo recorre domain.PinnedDocs: añadir un documento nuevo al catálogo no debe
// exigir tocar este archivo.

// docAction distingue qué pide la ruta que se está tecleando.
type docAction int

const (
	docActionNinguna docAction = iota
	docActionExportar
	docActionImportar
)

// docsSeeder y docsTopics extraen del repositorio las capacidades de siembra y
// consulta por clave. Aserción de tipo, igual que en el composition root.
func (m model) docsSeeder() (ports.MemorySeeder, bool) {
	s, ok := m.memRepo.(ports.MemorySeeder)
	return s, ok
}

func (m model) docsTopics() (ports.MemoryTopicQuerier, bool) {
	tq, ok := m.memRepo.(ports.MemoryTopicQuerier)
	return tq, ok
}

// docSeleccionado devuelve el documento del catálogo que la pantalla está
// mostrando.
func (m model) docSeleccionado() (domain.PinnedDoc, bool) {
	if m.docIndex < 0 || m.docIndex >= len(domain.PinnedDocs) {
		return domain.PinnedDoc{}, false
	}
	return domain.PinnedDocs[m.docIndex], true
}

// docEstado calcula el estado observable de un documento. La plantilla la
// aporta el llamador vía docTemplate, que la TUI recibe inyectada para no
// depender del sistema de archivos embebido del paquete cli.
func (m model) docEstado(d domain.PinnedDoc) usecases.DocStatus {
	topics, ok := m.docsTopics()
	if !ok {
		return usecases.DocStatus{State: usecases.DocSinSembrar}
	}
	return usecases.PinnedDocState(topics, m.project, d.TopicKey, m.docTemplate(d.Template))
}

// docTemplate resuelve el contenido por defecto de un documento. DocTemplates
// lo inyecta quien construye la TUI (infrastructure), que sí tiene acceso a las
// plantillas embebidas. Vacío = este binario no trae contenido por defecto, y
// restaurar se rechazará con un motivo claro en vez de dejar el documento en
// blanco.
func (m model) docTemplate(nombre string) string {
	if m.docTemplates == nil {
		return ""
	}
	return m.docTemplates[nombre]
}

func (m model) updateDocs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Con una ruta pendiente, la pantalla está en modo entrada de texto.
	if m.docPendiente != docActionNinguna {
		return m.updateDocsRuta(msg)
	}

	switch msg.String() {
	case "esc":
		m.screen = screenConfig
		m.docErr = ""
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "v":
		doc, ok := m.docSeleccionado()
		if !ok {
			return m, nil
		}
		topics, _ := m.docsTopics()
		res, err := usecases.ResolvePinnedDoc(topics, m.project, doc.TopicKey, m.docTemplate(doc.Template))
		if err != nil {
			m.docErr = err.Error()
			return m, nil
		}
		m.docVista = res.Content
		m.docErr = ""
		return m, nil

	case "e":
		m.docPendiente = docActionExportar
		m.docPath.SetValue("")
		m.docPath.Focus()
		m.docErr = ""
		return m, nil

	case "i":
		m.docPendiente = docActionImportar
		m.docPath.SetValue("")
		m.docPath.Focus()
		m.docErr = ""
		return m, nil

	case "r":
		// Restaurar descarta el contenido personalizado: nunca sin confirmar,
		// mismo criterio que las operaciones de mantenimiento.
		m.docConfirmReset = true
		m.docErr = ""
		return m, nil

	case "y", "s":
		if m.docConfirmReset {
			m.docConfirmReset = false
			return m.restaurarDoc()
		}

	case "n":
		if m.docConfirmReset {
			m.docConfirmReset = false
			m.statusMsg = "Restauración cancelada"
			m.statusTimer = 40
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateDocsRuta(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.docPendiente = docActionNinguna
		m.docErr = ""
		return m, nil

	case "ctrl+c":
		return m, tea.Quit

	case "enter":
		ruta := strings.TrimSpace(m.docPath.Value())
		if ruta == "" {
			m.docErr = "Indica una ruta de archivo"
			return m, nil
		}
		accion := m.docPendiente
		m.docPendiente = docActionNinguna
		if accion == docActionExportar {
			return m.exportarDoc(ruta)
		}
		return m.importarDoc(ruta)
	}

	var cmd tea.Cmd
	m.docPath, cmd = m.docPath.Update(msg)
	return m, cmd
}

func (m model) exportarDoc(ruta string) (tea.Model, tea.Cmd) {
	doc, ok := m.docSeleccionado()
	if !ok {
		return m, nil
	}
	topics, _ := m.docsTopics()
	res, err := usecases.ResolvePinnedDoc(topics, m.project, doc.TopicKey, m.docTemplate(doc.Template))
	if err != nil {
		m.docErr = err.Error()
		return m, nil
	}
	if err := os.WriteFile(ruta, []byte(res.Content), 0o644); err != nil {
		m.docErr = "No se pudo escribir: " + err.Error()
		return m, nil
	}
	m.docErr = ""
	m.statusMsg = fmt.Sprintf("%s exportado → %s", doc.Label, ruta)
	m.statusTimer = 80
	return m, nil
}

// importarDoc reemplaza el contenido desde un archivo. Ante cualquier fallo el
// documento anterior queda INTACTO y el error se muestra sin salir de la
// pantalla: perder el documento por un import fallido sería el peor desenlace
// posible de esta capacidad.
func (m model) importarDoc(ruta string) (tea.Model, tea.Cmd) {
	doc, ok := m.docSeleccionado()
	if !ok {
		return m, nil
	}
	datos, err := os.ReadFile(ruta)
	if err != nil {
		m.docErr = "No se pudo leer: " + err.Error()
		return m, nil
	}
	seeder, ok1 := m.docsSeeder()
	topics, ok2 := m.docsTopics()
	if !ok1 || !ok2 {
		m.docErr = "No hay acceso de escritura a la memoria"
		return m, nil
	}

	res, err := usecases.ImportPinnedDoc(seeder, topics, m.project,
		doc.TopicKey, doc.Type, doc.Title, string(datos))
	if err != nil {
		m.docErr = err.Error()
		return m, nil
	}

	m.docErr = ""
	switch {
	case res.Unchanged:
		m.statusMsg = doc.Label + " sin cambios"
	case res.Created:
		m.statusMsg = fmt.Sprintf("%s creado desde %s (%d líneas)", doc.Label, ruta, res.Lines)
	default:
		m.statusMsg = fmt.Sprintf("%s actualizado desde %s (%d líneas)", doc.Label, ruta, res.Lines)
	}
	m.statusTimer = 80
	m.memories, _ = m.memRepo.List(m.project, 200)
	m.applyFilter()
	return m, nil
}

func (m model) restaurarDoc() (tea.Model, tea.Cmd) {
	doc, ok := m.docSeleccionado()
	if !ok {
		return m, nil
	}
	seeder, ok1 := m.docsSeeder()
	topics, ok2 := m.docsTopics()
	if !ok1 || !ok2 {
		m.docErr = "No hay acceso de escritura a la memoria"
		return m, nil
	}

	res, err := usecases.ResetPinnedDoc(seeder, topics, m.project,
		doc.TopicKey, doc.Type, doc.Title, m.docTemplate(doc.Template))
	if err != nil {
		m.docErr = err.Error()
		return m, nil
	}
	m.docErr = ""
	if res.Unchanged {
		m.statusMsg = doc.Label + " ya estaba en su contenido por defecto"
	} else {
		m.statusMsg = fmt.Sprintf("%s restaurado al contenido por defecto (%d líneas)", doc.Label, res.Lines)
	}
	m.statusTimer = 80
	return m, nil
}

func (m model) docsView() string {
	doc, ok := m.docSeleccionado()
	if !ok {
		return appStyle.Render("Documento no encontrado")
	}
	st := m.docEstado(doc)

	var b strings.Builder
	b.WriteString(titleStyle.Render(doc.Label))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render(fmt.Sprintf("%s · %d líneas · clave %s", st.State, st.Lines, doc.TopicKey)))
	b.WriteString("\n\n")

	if m.docPendiente != docActionNinguna {
		etiqueta := "Ruta del archivo donde exportar:"
		if m.docPendiente == docActionImportar {
			etiqueta = "Ruta del archivo a importar:"
		}
		b.WriteString(formLabel.Render(etiqueta))
		b.WriteString("\n")
		b.WriteString(m.docPath.View())
		b.WriteString("\n\n")
		if m.docErr != "" {
			b.WriteString(errorStyle.Render("  " + m.docErr))
			b.WriteString("\n\n")
		}
		b.WriteString(helpStyle.Render("  enter confirmar  ·  esc cancelar"))
		return appStyle.Render(b.String())
	}

	if m.docConfirmReset {
		b.WriteString(errorStyle.Render("  ¿Restaurar el contenido por defecto? Se descarta el texto actual."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("  s confirmar  ·  n cancelar"))
		return appStyle.Render(b.String())
	}

	if m.docVista != "" {
		b.WriteString(m.docVista)
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("  esc volver"))
		return appStyle.Render(b.String())
	}

	for _, fila := range []string{
		"v  Ver contenido",
		"e  Exportar a archivo",
		"i  Importar desde archivo",
		"r  Restaurar contenido por defecto",
	} {
		b.WriteString(itemNormal.Render("  " + fila))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if m.docErr != "" {
		b.WriteString(errorStyle.Render("  " + m.docErr))
		b.WriteString("\n\n")
	}
	if status := m.statusLine(); status != "" {
		b.WriteString(status)
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("  esc volver"))
	return appStyle.Render(b.String())
}

// DocTemplates son los contenidos por defecto de los documentos fijados,
// indexados por nombre de plantilla. Lo inyecta el composition root
// (infrastructure/main.go), que es quien tiene acceso al sistema de archivos
// embebido — la TUI no debe leerlo por su cuenta.
//
// Nil o incompleto degrada con gracia: exportar sigue funcionando sobre el
// contenido guardado, y restaurar se rechaza con un motivo claro en vez de
// dejar el documento en blanco.
var DocTemplates map[string]string
