package ports

import (
	"context"

	"mem/domain"
)

// RuleSourcePort carga las reglas efectivas de expand (feature 040): las de
// serie y las propias de <root>/.gomemory/rules/*.md. Las reglas inválidas
// se devuelven como errores de carga y no impiden cargar las demás.
type RuleSourcePort interface {
	Load(ctx context.Context, root string) ([]domain.Rule, []domain.RuleLoadError)
}
