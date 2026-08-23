package cli

import (
	"fmt"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// seedsFromCatalog construye las semillas recorriendo domain.PinnedDocs y
// leyendo el contenido por defecto de las plantillas embebidas. Una plantilla
// ausente (TemplatesFS nil en algunos tests) produce contenido vacío, que
// SeedDefaults omite en silencio.
func seedsFromCatalog() []usecases.Seed {
	out := make([]usecases.Seed, 0, len(domain.PinnedDocs))
	for _, d := range domain.PinnedDocs {
		out = append(out, usecases.Seed{
			TopicKey: d.TopicKey,
			Type:     d.Type,
			Title:    d.Title,
			Content:  embeddedTemplate(d.Template),
		})
	}
	return out
}

// seedDeps extrae del repositorio las dos capacidades que la siembra necesita.
// Aserción de tipo en vez de ensanchar ports.MemoryRepository: mismo patrón que
// el composition root usa con ADRSyncProvider.
func seedDeps(deps *Deps) (ports.MemorySeeder, ports.MemoryTopicQuerier, bool) {
	seeder, ok1 := deps.MemoryRepo.(ports.MemorySeeder)
	topics, ok2 := deps.MemoryRepo.(ports.MemoryTopicQuerier)
	return seeder, topics, ok1 && ok2
}

// seedProject siembra las memorias por defecto del proyecto. Devuelve cuántas
// se crearon y el error acumulado.
//
// Es una capa OPORTUNISTA: quien la llama informa el resultado pero nunca
// aborta por él. Se invoca desde dos sitios —`mem install` y el arranque del
// servidor MCP— porque desde v1.9 mucha gente registra el MCP en ámbito global
// y no ejecuta `install` nunca (research.md §R6).
func seedProject(deps *Deps, project string) ([]string, error) {
	seeder, topics, ok := seedDeps(deps)
	if !ok {
		return nil, nil
	}
	return usecases.SeedDefaults(seeder, topics, project, seedsFromCatalog())
}

// CmdSeed siembra las memorias por defecto del proyecto del directorio actual.
//
// Existe como subcomando propio por una razón concreta: `install` está en
// rootIndependentCommands, así que se despacha SIN contenedor y su Deps no trae
// MemoryRepo — no puede escribir en la memoria del proyecto destino, que además
// suele ser distinto del directorio actual. El instalador lo invoca como
// subproceso con cwd en el destino, exactamente igual que ya hace con `init`.
//
// También sirve de escape manual: `mem seed` recrea una semilla borrada.
func CmdSeed(deps *Deps, _ []string) {
	created, err := seedProject(deps, deps.Project)
	if err != nil {
		fail("sembrar memorias por defecto: %v", err)
	}
	if len(created) == 0 {
		fmt.Println("Semillas ya presentes, no se sobrescriben.")
		return
	}
	fmt.Printf("✓ %d memoria(s) por defecto sembradas\n", len(created))
}
