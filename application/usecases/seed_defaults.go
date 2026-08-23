package usecases

import (
	"errors"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// Seed describe una memoria semilla a sembrar. El contenido llega por parámetro
// y no se lee aquí: las plantillas viven embebidas en el binario y esta capa no
// puede importar el sistema de archivos sin romper la regla de dependencias
// (constitución, principio I).
type Seed struct {
	TopicKey string
	Type     domain.MemoryType
	Title    string
	Content  string
}

// SeedDefaults siembra las memorias que falten y devuelve las claves de tópico
// realmente creadas. Vacío = no había nada que sembrar.
//
// INVARIANTE: nunca ejecuta un UPDATE. La única escritura posible es una
// inserción sobre una clave inexistente. En cuanto una semilla existe, es
// propiedad de la persona: aunque una versión futura del binario traiga una
// plantilla mejor, el texto del equipo gana. Ese es el motivo de comprobar
// antes de escribir en vez de confiar en el upsert por topic_key, que
// silenciosamente reemplazaría el contenido (research.md §R5).
//
// Escribe por la vía INERTE: sin sinapsis automática —una semilla no nace del
// trabajo de una sesión— y sin publicación al ADR externo, que con la
// sincronización activada habría publicado la constitución entera fuera.
//
// Es una capa oportunista: los errores se acumulan y se devuelven, pero nunca
// impiden intentar el resto de semillas ni deben abortar a quien la invoca.
func SeedDefaults(
	seeder ports.MemorySeeder,
	topics ports.MemoryTopicQuerier,
	project string,
	seeds []Seed,
) ([]string, error) {
	if seeder == nil || topics == nil {
		return nil, nil
	}

	var created []string
	var errs []error

	for _, s := range seeds {
		if strings.TrimSpace(s.Content) == "" {
			// Plantilla no embebida en este binario: omitir en silencio es
			// preferible a crear una memoria vacía que el agente tomaría por
			// buena. Mismo criterio que embeddedTemplate e
			// InstallAtomicPlanWrappers.
			continue
		}

		existente, err := topics.ByTopicKey(project, s.TopicKey)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if existente != nil {
			continue // ya es de la persona: no se toca
		}

		if _, err := seeder.InsertSeed(&domain.Memory{
			Project:  project,
			Type:     s.Type,
			Title:    s.Title,
			Content:  s.Content,
			TopicKey: s.TopicKey,
			// SessionID y Filepath quedan vacíos a propósito: una semilla no
			// pertenece a una sesión ni describe un archivo del proyecto.
		}); err != nil {
			errs = append(errs, err)
			continue
		}
		created = append(created, s.TopicKey)
	}

	return created, errors.Join(errs...)
}
