package usecases

import (
	"fmt"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// ResolvedDoc es el contenido vigente de un documento fijado, con su
// procedencia. FromMemory permite a la capa de presentación avisar cuando lo
// que se está mostrando NO es lo que el proyecto tiene guardado, sino el
// contenido por defecto del binario.
type ResolvedDoc struct {
	Content    string
	FromMemory bool
	ID         int64
}

// ResolvePinnedDoc devuelve el contenido vigente de un documento fijado: el de
// la memoria del proyecto si existe, o el de reserva si no.
//
// El texto de reserva llega POR PARÁMETRO en vez de leerse aquí: este caso de
// uso no puede importar el sistema de archivos embebido sin romper la regla de
// dependencias de la constitución (principio I — la capa de aplicación solo
// importa dominio). Quien llama —la CLI, que sí tiene acceso a las plantillas—
// es quien lo aporta.
//
// topics nil degrada a la reserva sin fallar, mismo criterio nil-safe que el
// resto de colaboradores opcionales del proyecto.
func ResolvePinnedDoc(topics ports.MemoryTopicQuerier, project, topicKey, fallback string) (ResolvedDoc, error) {
	if topics != nil {
		m, err := topics.ByTopicKey(project, topicKey)
		if err != nil {
			return ResolvedDoc{}, fmt.Errorf("leer documento fijado: %w", err)
		}
		if m != nil {
			return ResolvedDoc{Content: m.Content, FromMemory: true, ID: m.ID}, nil
		}
	}

	if strings.TrimSpace(fallback) == "" {
		return ResolvedDoc{}, fmt.Errorf(
			"no hay contenido para %q: ni en la memoria del proyecto ni como plantilla en este binario", topicKey)
	}
	return ResolvedDoc{Content: fallback}, nil
}

// ImportResult describe qué ocurrió al importar, para que la capa de
// presentación informe con precisión en vez de decir siempre "actualizado".
type ImportResult struct {
	Created   bool
	Unchanged bool
	ID        int64
	Lines     int
}

// ImportPinnedDoc reemplaza (o crea) el contenido de un documento fijado.
//
// Es la operación hermana de la siembra y su opuesto deliberado: SeedDefaults
// nunca pisa un documento existente porque nadie se lo pidió; ImportPinnedDoc
// siempre lo reemplaza porque es exactamente lo que se pidió. La regla de fondo
// es una sola — el contenido es de la persona (data-model.md §3ter).
//
// Escribe por la vía INERTE: sin sinapsis automática y sin publicación al ADR
// externo, ni siquiera con la sincronización activada (FR-045).
//
// Rechaza contenido vacío —también el que queda vacío tras depurar secretos—
// dejando el documento anterior INTACTO (FR-040). Es el peor modo de fallo
// posible de esta capacidad: un import fallido que destruya lo que había.
func ImportPinnedDoc(
	seeder ports.MemorySeeder,
	topics ports.MemoryTopicQuerier,
	project, topicKey string,
	memType domain.MemoryType,
	title, content string,
) (ImportResult, error) {
	if seeder == nil {
		return ImportResult{}, fmt.Errorf("no hay escritura de memoria disponible")
	}
	if strings.TrimSpace(topicKey) == "" {
		return ImportResult{}, fmt.Errorf("la clave del documento no puede estar vacía")
	}

	if strings.TrimSpace(content) == "" {
		return ImportResult{}, fmt.Errorf("el contenido está vacío: no se modificó nada")
	}
	// Se comprueba ANTES de escribir, con la misma depuración que aplicará la
	// persistencia: un archivo que solo contiene un bloque privado quedaría
	// vacío al guardarse, y el documento no puede quedar en blanco.
	if strings.TrimSpace(domain.RedactPrivate(content)) == "" {
		return ImportResult{}, fmt.Errorf("el contenido queda vacío tras depurar los bloques privados: no se modificó nada")
	}

	var previo *domain.Memory
	if topics != nil {
		var err error
		if previo, err = topics.ByTopicKey(project, topicKey); err != nil {
			return ImportResult{}, fmt.Errorf("leer el documento actual: %w", err)
		}
	}

	if previo != nil && previo.Content == content {
		return ImportResult{Unchanged: true, ID: previo.ID, Lines: contarLineas(content)}, nil
	}

	id, err := seeder.InsertSeed(&domain.Memory{
		Project:  project,
		Type:     memType,
		Title:    title,
		Content:  content,
		TopicKey: topicKey,
	})
	if err != nil {
		return ImportResult{}, fmt.Errorf("guardar el documento: %w", err)
	}

	return ImportResult{Created: previo == nil, ID: id, Lines: contarLineas(content)}, nil
}

// ResetPinnedDoc restaura un documento fijado a su contenido por defecto.
// Es una importación de la plantilla: misma vía inerte, mismas validaciones.
func ResetPinnedDoc(
	seeder ports.MemorySeeder,
	topics ports.MemoryTopicQuerier,
	project, topicKey string,
	memType domain.MemoryType,
	title, template string,
) (ImportResult, error) {
	if strings.TrimSpace(template) == "" {
		return ImportResult{}, fmt.Errorf("este binario no trae contenido por defecto para %q", topicKey)
	}
	return ImportPinnedDoc(seeder, topics, project, topicKey, memType, title, template)
}

// DocState es el estado observable de un documento fijado. Se DERIVA comparando
// con la plantilla embebida en vez de almacenarse: una columna nueva podría
// quedar desincronizada del contenido real (data-model.md §3ter).
type DocState string

const (
	DocSinSembrar    DocState = "sin sembrar"
	DocPorDefecto    DocState = "por defecto"
	DocPersonalizado DocState = "personalizado"
)

// DocStatus reúne lo que la CLI y la TUI necesitan mostrar de un documento.
type DocStatus struct {
	State     DocState
	Lines     int
	UpdatedAt string
	ID        int64
}

// PinnedDocState calcula el estado de un documento fijado. Nunca falla: un
// error de lectura se reporta como "sin sembrar", porque para quien mira la
// lista el efecto es el mismo —no hay nada que enseñar— y este cálculo no debe
// tumbar la pantalla de configuración.
func PinnedDocState(topics ports.MemoryTopicQuerier, project, topicKey, template string) DocStatus {
	if topics == nil {
		return DocStatus{State: DocSinSembrar}
	}
	m, err := topics.ByTopicKey(project, topicKey)
	if err != nil || m == nil {
		return DocStatus{State: DocSinSembrar}
	}

	st := DocStatus{
		State:     DocPersonalizado,
		Lines:     contarLineas(m.Content),
		UpdatedAt: m.UpdatedAt,
		ID:        m.ID,
	}
	if strings.TrimSpace(m.Content) == strings.TrimSpace(template) {
		st.State = DocPorDefecto
	}
	return st
}

func contarLineas(s string) int {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}
