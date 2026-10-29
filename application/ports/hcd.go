package ports

import (
	"context"
	"mem/domain"
)

// HCDStore guarda el estado de control HCD de un proyecto: unidades,
// evidencia y aprobaciones. El proyecto queda fijado en el constructor.
type HCDStore interface {
	NextNumber(ctx context.Context) (int, error)
	Create(ctx context.Context, unit domain.Unit) error
	Get(ctx context.Context, id string) (domain.Unit, error)
	Active(ctx context.Context) (domain.Unit, bool, error)
	// Activate deja id como única unidad activa del proyecto.
	Activate(ctx context.Context, id string) error
	Save(ctx context.Context, unit domain.Unit) error
	AddEvidence(ctx context.Context, evidence domain.Evidence) error
	Evidence(ctx context.Context, unitID string) ([]domain.Evidence, error)
	AddApproval(ctx context.Context, approval domain.Approval) error
	Approvals(ctx context.Context, unitID string) ([]domain.Approval, error)
}

// HCDWorkspace es el lado de archivos, git y ejecución de comandos.
type HCDWorkspace interface {
	ReadDoc(path string) ([]byte, error)
	// WriteDoc solo escribe bajo hcd/.
	WriteDoc(path string, data []byte) error
	// Fingerprint identifica el código evaluado, excluyendo hcd/.
	Fingerprint(ctx context.Context) (string, error)
	// Run ejecuta el comando y devuelve el código de salida y un extracto acotado.
	Run(ctx context.Context, command string) (exitCode int, excerpt string, err error)
}

// HCDContextRepository alimenta la sección «HCD activo» del contexto.
type HCDContextRepository interface {
	ActiveHCDContext(ctx context.Context) (domain.HCDActiveContext, bool, error)
}
