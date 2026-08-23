package ports

import "mem/domain"

// MemoryTopicQuerier resuelve una memoria por su clave de tópico, sin depender
// de cuán reciente sea (feature 021, FR-006/FR-031).
//
// Es un puerto aparte de MemoryLister a propósito: el constructor de contexto
// solo necesita ESTA capacidad para emitir la sección de reglas fijadas, no la
// escritura ni el listado completo. Mantenerlo mínimo evita que un colaborador
// opcional arrastre toda la superficie del repositorio.
type MemoryTopicQuerier interface {
	// ByTopicKey devuelve la memoria con esa clave en el proyecto, o
	// (nil, nil) si no existe. Un error señala un fallo real de lectura,
	// nunca "no encontrado" — regla explícita de la constitución.
	ByTopicKey(project, topicKey string) (*domain.Memory, error)
}

// MemorySeeder inserta una memoria por la vía INERTE: sin sinapsis automática y
// sin publicación al proveedor externo de ADR, ni siquiera cuando la
// sincronización esté activada (feature 021, FR-033/FR-034).
//
// Puerto aparte de la escritura normal a propósito: quien lo usa DECLARA que
// está sembrando o reemplazando un documento fijado, no guardando el fruto de
// una sesión de trabajo. La depuración de secretos sigue activa por esta vía —
// es una defensa de seguridad, no un canal lateral.
type MemorySeeder interface {
	InsertSeed(m *domain.Memory) (int64, error)
}
