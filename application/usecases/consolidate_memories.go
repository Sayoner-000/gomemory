package usecases

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// ConsolidationGroup es un conjunto de memorias que la consolidación funde
// en una sola (feature 020, fase B). Criterion identifica por qué se
// agruparon: "topic_key" (FR-026, clave de tópico explícita) o
// "checkpoint_duplicate" (research.md §5: registros automáticos de
// actividad con contenido byte a byte idéntico — el criterio ampliado, sin
// el cual el Δ de FR-030 es demostrablemente cero contra la base real).
type ConsolidationGroup struct {
	Criterion string
	Key       string
	// Memories va ordenado ascendente por ID; el último (mayor ID) es el que
	// se conserva.
	Memories []domain.Memory
}

// ConsolidationReport es el resultado de ConsolidateMemories, tanto en
// previsualización como tras aplicar.
type ConsolidationReport struct {
	Groups       []ConsolidationGroup
	DeletedCount int
}

// ConsolidateMemories agrupa memorias redundantes de un proyecto por dos
// criterios — clave de tópico y registros de actividad idénticos — y funde
// cada grupo en su fila más reciente. apply=false solo previsualiza (FR-027,
// operación irreversible: la previsualización es el comportamiento por
// defecto); apply=true funde y elimina de verdad.
//
// Ningún contenido se pierde (FR-026): si el grupo tiene contenido distinto
// entre sus filas (posible en grupos de topic_key con datos anteriores al
// mecanismo de upsert), todos los textos distintos sobreviven, unidos, en la
// fila que se conserva. Los grupos por contenido idéntico no necesitan
// fusión textual: conservar una copia ya conserva todo el contenido.
func ConsolidateMemories(memRepo ports.MemoryRepository, project string, apply bool) (ConsolidationReport, error) {
	mems, err := memRepo.ListAll(project)
	if err != nil {
		return ConsolidationReport{}, err
	}

	groups := groupByTopicKey(mems)
	groups = append(groups, groupCheckpointDuplicates(mems)...)

	report := ConsolidationReport{Groups: groups}
	for _, g := range groups {
		report.DeletedCount += len(g.Memories) - 1
	}

	if !apply {
		return report, nil
	}

	// El borrado va al final y en una sola operación. Un fallo a mitad borra lo
	// ya fundido: esas memorias ya están contenidas en su fila superviviente.
	var fundidas []int64
	var fallo error
	for _, g := range groups {
		ids, err := applyGroup(memRepo, project, g)
		if err != nil {
			fallo = err
			break
		}
		fundidas = append(fundidas, ids...)
	}
	// El error conserva ErrLimpiezaPendiente: lo fundido ya se borró.
	if _, err := memRepo.DeleteMany(project, fundidas); err != nil && fallo == nil {
		fallo = fmt.Errorf("eliminar memorias fundidas: %w", err)
	}
	return report, fallo
}

// groupByTopicKey agrupa por project+topic_key (FR-026). Las memorias sin
// clave de tópico no se tocan (FR-029).
func groupByTopicKey(mems []domain.Memory) []ConsolidationGroup {
	byKey := map[string][]domain.Memory{}
	for _, m := range mems {
		tk := strings.TrimSpace(m.TopicKey)
		if tk == "" {
			continue
		}
		byKey[tk] = append(byKey[tk], m)
	}
	return buildGroups("topic_key", byKey)
}

// groupCheckpointDuplicates agrupa registros de actividad (domain.Checkpoint)
// cuyo contenido es byte a byte idéntico — el criterio ampliado que hace
// medible el Δ de FR-030 (research.md §5): sobre la base real del proyecto,
// el 55% de los checkpoints recientes son duplicados literales, mientras que
// los grupos de topic_key con más de una fila son cero.
func groupCheckpointDuplicates(mems []domain.Memory) []ConsolidationGroup {
	byHash := map[string][]domain.Memory{}
	for _, m := range mems {
		if m.Type != domain.Checkpoint {
			continue
		}
		sum := sha256.Sum256([]byte(m.Content))
		key := fmt.Sprintf("%x", sum)
		byHash[key] = append(byHash[key], m)
	}
	return buildGroups("checkpoint_duplicate", byHash)
}

func buildGroups(criterion string, byKey map[string][]domain.Memory) []ConsolidationGroup {
	var groups []ConsolidationGroup
	for key, ms := range byKey {
		if len(ms) < 2 {
			continue
		}
		sort.Slice(ms, func(i, j int) bool { return ms[i].ID < ms[j].ID })
		groups = append(groups, ConsolidationGroup{Criterion: criterion, Key: key, Memories: ms})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Criterion != groups[j].Criterion {
			return groups[i].Criterion < groups[j].Criterion
		}
		return groups[i].Memories[0].ID < groups[j].Memories[0].ID
	})
	return groups
}

// applyGroup funde un grupo en su fila más reciente y devuelve los ids del resto,
// ya sobrantes.
func applyGroup(memRepo ports.MemoryRepository, project string, g ConsolidationGroup) ([]int64, error) {
	kept := g.Memories[len(g.Memories)-1]
	mergedContent := mergeContents(g.Memories)
	if mergedContent != kept.Content {
		if err := memRepo.UpdateContent(project, kept.ID, kept.Title, mergedContent); err != nil {
			return nil, fmt.Errorf("fusionar grupo %s=%q en id=%d: %w", g.Criterion, g.Key, kept.ID, err)
		}
	}
	ids := make([]int64, 0, len(g.Memories)-1)
	for _, m := range g.Memories[:len(g.Memories)-1] {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// mergeContents concatena los textos DISTINTOS de un grupo, en orden, sin
// perder ninguno (FR-026). Cuando todos son idénticos (el caso típico de
// checkpoint_duplicate) devuelve ese único texto, sin duplicarlo.
func mergeContents(mems []domain.Memory) string {
	seen := map[string]bool{}
	var parts []string
	for _, m := range mems {
		c := strings.TrimSpace(m.Content)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		parts = append(parts, m.Content)
	}
	if len(parts) <= 1 {
		if len(parts) == 1 {
			return parts[0]
		}
		return ""
	}
	return strings.Join(parts, "\n\n---\n\n")
}
