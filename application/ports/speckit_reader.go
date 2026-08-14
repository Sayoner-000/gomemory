package ports

import "mem/domain"

// SpecKitReader lee artefactos de Spec Kit (.specify/feature.json,
// specs/<feature>/{spec.md,plan.md,research.md,tasks.md},
// .specify/memory/constitution.md) acotados a UNA feature — nunca mezcla
// contenido de otras (feature 015, FR-015).
type SpecKitReader interface {
	// ActiveFeature lee .specify/feature.json y devuelve el nombre de
	// carpeta bajo specs/ (p. ej. "015-context-optimization"), o "" sin
	// error si el proyecto no tiene Spec Kit inicializado.
	ActiveFeature(root string) (string, error)
	// Read carga los artefactos de feature y los recorta a lo relevante
	// para task.
	Read(root, feature, task string) (domain.SpecKitFeatureContext, error)
}
