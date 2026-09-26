package domain

import (
	"strings"
)

// ContentType es el tipo de contenido que el router del motor nativo de
// compresión asigna a cada bloque (feature 033, research.md R2). Decide qué
// compresor se aplica.
type ContentType string

const (
	ContentJSON    ContentType = "json"
	ContentCode    ContentType = "code"
	ContentLog     ContentType = "log"
	ContentDiff    ContentType = "diff"
	ContentTable   ContentType = "table"
	ContentProse   ContentType = "prose"
	ContentMixed   ContentType = "mixed"
	ContentUnknown ContentType = "unknown"
)

// Subtipos de lenguaje para ContentCode. "generic" cubre cualquier lenguaje
// que el router no reconozca con confianza.
const (
	LangGo      = "go"
	LangPython  = "python"
	LangJava    = "java"
	LangTS      = "ts"
	LangJS      = "js"
	LangSQL     = "sql"
	LangGeneric = "generic"
)

// Delimitadores del marcador de omisión (R10). U+27E6 y U+27E7 no aparecen en
// código real, así que el marcador nunca se confunde con contenido.
const (
	MarkerOpen  = "⟦"
	MarkerClose = "⟧"
	// MarkerTag es la etiqueta completa con la que empieza todo marcador y la
	// clave del objeto marcador en JSON.
	MarkerTag = MarkerOpen + "mem" + MarkerClose
	// DeliveredTag marca una entrada que el delta de sesión sustituyó porque ya
	// se envió en la sesión (feature 033, FR-018).
	DeliveredTag = MarkerOpen + "ya entregado" + MarkerClose
)

// Omission describe un fragmento que un compresor omitió: la referencia con la
// que se recupera el original y un resumen breve de lo que falta.
type Omission struct {
	Ref     string
	Summary string
}

// RenderTextMarker es la forma textual de una omisión:
// "⟦mem⟧ <Summary> · ref=<Ref>". Sin Ref (omisión sin original recuperable)
// el marcador no promete nada que no se pueda cumplir.
func RenderTextMarker(o Omission) string {
	if o.Ref == "" {
		return MarkerTag + " " + o.Summary
	}
	return MarkerTag + " " + o.Summary + " · ref=" + o.Ref
}

// RenderJSONMarker es la forma de una omisión dentro de un array JSON: un
// objeto más, en la posición del hueco, para que la salida siga siendo JSON
// válido. extra añade el resumen de campos (p. ej. {"status": "200×185"}).
func RenderJSONMarker(o Omission, extra map[string]string) map[string]any {
	m := map[string]any{MarkerTag: o.Summary}
	if o.Ref != "" {
		m["ref"] = o.Ref
	}
	if len(extra) > 0 {
		m["campos"] = extra
	}
	return m
}

// IsMarkerLine dice si una línea es (o contiene) una marca de gomemory: de
// omisión (MarkerTag) o de entrega de sesión (DeliveredTag). Una marca ocupa el
// lugar de un contenido concreto, así que ningún compresor la deduplica aunque
// su texto coincida con el de otra (C-002 de acr_6793454b).
func IsMarkerLine(line string) bool {
	return strings.Contains(line, MarkerTag) || strings.Contains(line, DeliveredTag)
}

// Longitudes de la referencia: 12 hex por defecto (48 bits, barato en tokens) y
// 16 hex si los 12 primeros ya pertenecen a otro original (colisión).
const (
	RefLen          = 12
	RefLenCollision = 16
)

// RefFromHash recorta la huella sha256 (hex) a n caracteres. Con n fuera de
// rango devuelve la huella completa.
func RefFromHash(sha256hex string, n int) string {
	if n <= 0 || n >= len(sha256hex) {
		return sha256hex
	}
	return sha256hex[:n]
}

// IsPrivateContent dice si un texto lleva contenido que no puede persistirse
// como original recuperable (FR-024): marcas <private> o secretos detectados.
// Es la misma detección que se aplica al guardar memorias.
func IsPrivateContent(s string) bool {
	return RedactPrivate(s) != s || RedactSecrets(s) != s
}
