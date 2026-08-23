package domain

// Claves de tópico canónicas de las memorias que gomemory siembra al usarse por
// primera vez en un proyecto (feature 021).
//
// Son CONTRATO, no configuración: cambiarlas convertiría en huérfanas las
// semillas ya creadas en proyectos existentes y provocaría una segunda siembra
// duplicada. Si alguna vez hay que versionarlas, la migración debe ser
// explícita — nunca un cambio de constante (data-model.md §1).
//
// El prefijo "gomemory:" distingue una fila sembrada por el producto de una
// agrupación que la persona haya creado con su propio topic_key al guardar.
const (
	TopicWorkRules    = "gomemory:work-rules"
	TopicConstitution = "gomemory:constitution"
)

// PinnedDoc describe un documento fijado: la misma fila que la siembra crea,
// vista desde la perspectiva de quien la administra. Como semilla, la
// herramienta la crea si falta; como documento fijado, la persona la exporta,
// la edita y la reimporta.
//
// El catálogo es table-driven a propósito: la CLI y la TUI lo RECORREN en vez
// de enumerarlo, así que añadir un documento nuevo es una entrada más y nunca
// un comando ni una pantalla nueva. Si alguna vez hace falta tocar cmd_docs.go
// o tui_docs.go para sumar un documento, el diseño se rompió.
type PinnedDoc struct {
	// Alias es lo que teclea la persona: "rules", "constitution".
	Alias string
	// TopicKey es la identidad de la memoria en la base de datos.
	TopicKey string
	// Type clasifica la memoria. Determina en qué sección del contexto aparece
	// y, para Architecture/Decision, si sería exportable a ADR — de ahí que la
	// siembra y la importación usen SIEMPRE la vía inerte (research.md §R4).
	Type MemoryType
	// Title es el título de la memoria guardada.
	Title string
	// Label es el rótulo legible en la TUI.
	Label string
	// Template es el nombre del archivo embebido bajo templates/ que aporta el
	// contenido por defecto y el punto de retorno de una restauración.
	Template string
}

// PinnedDocs es el catálogo de documentos fijados que gomemory conoce por su
// nombre. No es un límite: la importación admite cualquier clave de tópico,
// dentro o fuera de esta lista (FR-042). El catálogo es una comodidad.
var PinnedDocs = []PinnedDoc{
	{
		Alias:    "rules",
		TopicKey: TopicWorkRules,
		Type:     Preference,
		Title:    "Reglas de trabajo del proyecto",
		Label:    "Reglas IA",
		Template: "agent-preamble.md",
	},
	{
		Alias:    "constitution",
		TopicKey: TopicConstitution,
		Type:     Architecture,
		Title:    "Constitución del proyecto (spec-kit)",
		Label:    "Constitución",
		Template: "speckit-constitution-gen.md",
	},
}

// PinnedDocByAlias resuelve un documento fijado por el alias que teclea la
// persona. El segundo valor es false si el alias no está en el catálogo — no es
// un error, es una consulta que no encontró nada (constitución, manejo de
// errores).
func PinnedDocByAlias(alias string) (PinnedDoc, bool) {
	for _, d := range PinnedDocs {
		if d.Alias == alias {
			return d, true
		}
	}
	return PinnedDoc{}, false
}

// PinnedDocByTopicKey resuelve un documento fijado por su clave de tópico. Lo
// usa el constructor de contexto para saber si una memoria listada es una
// semilla que ya emitió aparte y no debe repetir.
func PinnedDocByTopicKey(topicKey string) (PinnedDoc, bool) {
	for _, d := range PinnedDocs {
		if d.TopicKey == topicKey {
			return d, true
		}
	}
	return PinnedDoc{}, false
}

// PinnedDocAliases devuelve los alias válidos, en el orden del catálogo, para
// poder listarlos cuando alguien teclea uno que no existe.
func PinnedDocAliases() []string {
	out := make([]string, 0, len(PinnedDocs))
	for _, d := range PinnedDocs {
		out = append(out, d.Alias)
	}
	return out
}
