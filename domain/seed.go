package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

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
	// DefaultSHA256 es la huella (ContentFingerprint) de la plantilla que trae
	// este binario. Un test la compara con el archivo embebido: editar la
	// plantilla obliga a mover la huella vieja a PreviousDefaultSHA256.
	DefaultSHA256 string
	// PreviousDefaultSHA256 son las huellas de las plantillas que distribuyeron
	// versiones anteriores. Una semilla cuyo contenido coincide con una de
	// ellas nunca la editó nadie: la siembra puede llevarla a la plantilla
	// actual sin pisar el trabajo de ningún equipo.
	PreviousDefaultSHA256 []string
}

// ContentFingerprint es la huella con la que se reconoce una plantilla: SHA-256
// del contenido sin espacios en los extremos, el mismo criterio que usa
// PinnedDocState para decidir si un documento sigue "por defecto".
func ContentFingerprint(content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return hex.EncodeToString(sum[:])
}

// IsPreviousDefault informa si content es, intacta, una plantilla que
// distribuyó una versión anterior del binario.
func (d PinnedDoc) IsPreviousDefault(content string) bool {
	h := ContentFingerprint(content)
	for _, previa := range d.PreviousDefaultSHA256 {
		if previa == h {
			return true
		}
	}
	return false
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
		// Huellas por commit de la plantilla: 8edbddd y d7b63ae.
		DefaultSHA256: "9dc0258d753c3b77fc16ee69a654a9eb96513205e1b6c4797e1a22b048003b8a",
		PreviousDefaultSHA256: []string{
			"d742e66f6593cd7d60ac6b59cdc7e046371f6ab570f67742dc4f432aba78b669",
			"aaf5312676b94b034a6923b0e01dddb9b091c7035eee54d6fd75b5f7298d807e",
		},
	},
	{
		Alias:    "constitution",
		TopicKey: TopicConstitution,
		Type:     Architecture,
		Title:    "Constitución del proyecto (spec-kit)",
		Label:    "Constitución",
		Template: "speckit-constitution-gen.md",
		// Constitución técnica 2.0.0 (fecha de corte 2026-09-24). Las anteriores
		// son, por commit, 8edbddd, a873463, 77a1a11 y 4514061.
		DefaultSHA256: "6903740dcb4f1f631c14954b7f54a337dac37da62169bbd0d5a8cf719bdcc3f7",
		PreviousDefaultSHA256: []string{
			"7acc79bc12fe8351afcc8a02e0aa933996b0606b439f4710fcafafa0c6e50a79",
			"ce0774314fe7d124325e7716a7b024b95e4f826dd567138b815a1bef2d635759",
			"ca5b8802e8e427f67c657ddb2bb41a4abf391f09259a8cd4178302e225ba03f0",
			"25cce48ee8896714d4a740b35f045f388f0aeaf6592995d21787d088232d279c",
		},
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
