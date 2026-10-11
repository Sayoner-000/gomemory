package ports

import (
	"context"

	"mem/domain"
)

// Puertos del saneamiento histórico de privacidad (spec 037, US2). El caso
// de uso controla el orden y la transformación; los adaptadores controlan
// SQLite, archivos, permisos y publicación. Ningún tipo de driver ni SQL
// cruza estos puertos.

// PrivacyMaintenanceRepository lee candidatos fuera de una transacción, para
// la vista previa y la verificación final. Lee el texto ORIGINAL: no puede
// apoyarse en los lectores depurados, que ocultarían qué filas requieren
// saneamiento.
type PrivacyMaintenanceRepository interface {
	Candidates(ctx context.Context, project string) ([]domain.SanitizeCandidate, error)
}

// PrivacyTransaction es la vista transaccional que recibe el callback de
// WithTransaction.
type PrivacyTransaction interface {
	// Candidates relee los candidatos actuales dentro de la transacción.
	Candidates(ctx context.Context, project string) ([]domain.SanitizeCandidate, error)
	// Apply sanea un candidato escribible: reemplaza su texto por text si el
	// valor almacenado sigue siendo c.Text (un escritor concurrente que ya lo
	// cambió no se pisa), o elimina el original protegido. changed=false si no
	// hubo cambio. Solo toca el campo permitido; identidad y fechas intactas.
	Apply(ctx context.Context, c domain.SanitizeCandidate, text string) (changed bool, err error)
}

// PrivacyMaintenanceUnitOfWork abre la transacción inmediata de saneamiento
// y ejecuta la compactación física posterior.
type PrivacyMaintenanceUnitOfWork interface {
	// WithTransaction confirma solo si fn termina sin error; ante error o
	// cancelación hace rollback completo.
	WithTransaction(ctx context.Context, project string, fn func(PrivacyTransaction) error) error
	// Compact reclama páginas y vacía el WAL fuera de cualquier transacción.
	// Devuelve un *domain.SanitizeError con SanitizeCodeBusy si un lector o
	// escritor impide completar el checkpoint.
	Compact(ctx context.Context) error
}

// PrivacyArtifactsPort trata los archivos que gomemory controla fuera de la
// base: informes, snapshots de respaldo y contexto generado. Nunca sigue
// enlaces fuera de sus raíces administradas ni crea copias crudas.
type PrivacyArtifactsPort interface {
	// PublishReport escribe el informe depurado de forma atómica.
	PublishReport(ctx context.Context, report domain.SanitizeReport) error
	// PendingReports devuelve los informes del proyecto que quedaron en
	// pending_cleanup en ejecuciones anteriores y, aparte, los informes que no
	// se pudieron leer: no se puede saber si eran de este proyecto ni si
	// estaban pendientes, así que nunca se descartan en silencio.
	PendingReports(ctx context.Context, project string) ([]domain.SanitizeReport, []domain.SanitizeArtifact, error)
	// CleanBackups depura los snapshots administrados del proyecto.
	CleanBackups(ctx context.Context, project string) ([]domain.SanitizeArtifact, error)
	// ContextNeedsRefresh dice si el contexto generado existe y contiene
	// contenido protegido.
	ContextNeedsRefresh(ctx context.Context) (bool, error)
	// VerifyResidues revisa snapshots y contexto generado tras la limpieza y
	// devuelve los que todavía contienen contenido protegido.
	VerifyResidues(ctx context.Context, project string) ([]domain.SanitizeArtifact, error)
}

// PrivacySanitizer es el puerto estrecho que consume la consola: vista
// previa y aplicación explícita del saneamiento del proyecto actual.
type PrivacySanitizer interface {
	Preview(ctx context.Context, project string) (domain.SanitizeReport, error)
	Apply(ctx context.Context, project string) (domain.SanitizeReport, error)
}
