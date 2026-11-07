package ports

import (
	"context"
	"mem/domain"
)

// HCDStore guarda el estado de control HCD de un proyecto: unidades,
// evidencia y aprobaciones. El proyecto queda fijado en el constructor.
type HCDStore interface {
	// CreateActive asigna el siguiente número, crea la unidad construida por
	// build y la deja como única activa en una transacción. publish corre al
	// final, dentro de ella: si falla, la unidad no se crea.
	CreateActive(ctx context.Context, build func(number int) domain.Unit, publish func(domain.Unit) error) (domain.Unit, error)
	Get(ctx context.Context, id string) (domain.Unit, error)
	// List devuelve todas las unidades del proyecto por número (HCD-046).
	List(ctx context.Context) ([]domain.Unit, error)
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
	// UnitDocs devuelve las rutas relativas de los documentos hcd/HCD-*.md,
	// para indexar unidades sin fila en el motor (HCD-046, Q7).
	UnitDocs() ([]string, error)
	ReadDoc(path string) ([]byte, error)
	// WriteDoc solo escribe bajo hcd/.
	WriteDoc(path string, data []byte) error
	// Fingerprint identifica el código evaluado, excluyendo hcd/.
	Fingerprint(ctx context.Context) (string, error)
	// Run ejecuta el comando y devuelve el código de salida y un extracto acotado.
	Run(ctx context.Context, command string) (exitCode int, excerpt string, err error)
	// MaxUnitNumber devuelve el número de unidad más alto presente en hcd/HCD-*.md (0 si ninguno).
	MaxUnitNumber() (int, error)
	// PathExists informa si la ruta relativa existe dentro de la raíz del
	// proyecto; una ruta absoluta o que sale de la raíz no existe (HCD-034).
	PathExists(rel string) bool
}

// HCDContextRepository alimenta la sección «HCD activo» del contexto.
type HCDContextRepository interface {
	ActiveHCDContext(ctx context.Context) (domain.HCDActiveContext, bool, error)
}
