package usecases

import (
	"context"
	"errors"

	"mem/application/ports"
	"mem/domain"
)

// SanitizeProject sanea la información histórica protegida de un proyecto
// (spec 037, US2, FR-006–008).
//
// Orden de Apply: candidatos → informe pending_cleanup → transacción →
// limpieza de artefactos → compactación → verificación → completed. El
// informe se publica ANTES de tocar la base porque base y archivos no
// comparten commit: si el proceso muere en cualquier punto posterior, queda
// un rastro conservador que obliga a reintentar. Cada paso es idempotente,
// así que reintentar repite la transformación sin restaurar nada crudo.
type SanitizeProject struct {
	Repo      ports.PrivacyMaintenanceRepository
	UoW       ports.PrivacyMaintenanceUnitOfWork
	Artifacts ports.PrivacyArtifactsPort
	// Context regenera .memory/context.md si contiene contenido protegido.
	// Opcional: nil omite ese paso.
	Context  ports.ContextBuilder
	Clock    ports.ClockPort
	NewRunID func() string
}

// Preview enumera candidatos sin escribir filas ni informes.
func (u *SanitizeProject) Preview(ctx context.Context, project string) (domain.SanitizeReport, error) {
	report := domain.NewSanitizeReport(project, domain.SanitizeModePreview)
	cands, err := u.candidates(ctx, project)
	if err != nil {
		report.Status = domain.SanitizeStatusFailed
		return report, classifySanitize(err, domain.SanitizeCodeFailed)
	}
	report.Categories = domain.SummarizeCandidates(cands)
	prev, unreadable, err := u.Artifacts.PendingReports(ctx, project)
	if err == nil && len(prev) > 0 {
		report.Pending = append(report.Pending, domain.SanitizePending{
			Code:   domain.SanitizeCodePendingCleanup,
			Action: "Hay un saneamiento anterior sin completar: ejecuta mem sanitize --apply.",
		})
	}
	if len(unreadable) > 0 {
		report.Artifacts = append(report.Artifacts, unreadable...)
		report.Pending = append(report.Pending, unreadableReportsPending())
	}
	return report, nil
}

// Apply sanea el proyecto y verifica la limpieza. Devuelve un
// *domain.SanitizeError salvo que el informe termine completed.
func (u *SanitizeProject) Apply(ctx context.Context, project string) (domain.SanitizeReport, error) {
	report := domain.NewSanitizeReport(project, domain.SanitizeModeApply)
	report.RunID = u.NewRunID()
	started := u.Clock.Now()
	report.StartedAt = &started

	previous, unreadable, err := u.Artifacts.PendingReports(ctx, project)
	if err != nil {
		report.Status = domain.SanitizeStatusFailed
		return report, classifySanitize(err, domain.SanitizeCodeFailed)
	}
	cands, err := u.candidates(ctx, project)
	if err != nil {
		report.Status = domain.SanitizeStatusFailed
		return report, classifySanitize(err, domain.SanitizeCodeFailed)
	}
	report.Categories = domain.SummarizeCandidates(cands)
	if err := u.Artifacts.PublishReport(ctx, report); err != nil {
		report.Status = domain.SanitizeStatusFailed
		return report, domain.NewSanitizeError(domain.SanitizeCodeFailed, err)
	}

	changed := 0
	err = u.UoW.WithTransaction(ctx, project, func(tx ports.PrivacyTransaction) error {
		current, err := tx.Candidates(ctx, project)
		if err != nil {
			return err
		}
		// El informe describe lo que esta transacción ve y aplica, no la
		// lectura previa: un escritor pudo cambiar filas entre medias
		// (S-01, acr_9ba278e2).
		applied := domain.SummarizeCandidates(current)
		for _, c := range current {
			if err := c.Validate(project); err != nil {
				return err
			}
			if !c.Writable() {
				continue
			}
			ok, err := tx.Apply(ctx, c, domain.ProtectText(c.Text))
			if err != nil {
				return err
			}
			if ok {
				changed++
			}
		}
		report.Categories = applied
		return nil
	})
	if err != nil {
		return u.leavePending(ctx, report, classifySanitize(err, domain.SanitizeCodeFailed),
			"No se confirmó ningún cambio: corrige la causa y repite mem sanitize --apply.")
	}
	report.RowsChanged = changed

	if errs := u.cleanArtifacts(ctx, project, &report); errs != nil {
		return u.leavePending(ctx, report, classifySanitize(errs, domain.SanitizeCodePendingCleanup),
			"Las filas ya están saneadas; repite mem sanitize --apply para completar la limpieza.")
	}
	if err := u.UoW.Compact(ctx); err != nil {
		return u.leavePending(ctx, report, classifySanitize(err, domain.SanitizeCodePendingCleanup),
			"Detén los clientes de gomemory abiertos (agentes, MCP, TUI) y repite mem sanitize --apply.")
	}
	if err := u.verify(ctx, project, &report); err != nil {
		return u.leavePending(ctx, report, classifySanitize(err, domain.SanitizeCodePendingCleanup),
			"Quedan residuos controlados: revisa los artefactos pendientes y repite mem sanitize --apply.")
	}
	if len(unreadable) > 0 {
		report.Artifacts = append(report.Artifacts, unreadable...)
		return u.leavePending(ctx, report,
			domain.NewSanitizeError(domain.SanitizeCodePendingCleanup, errors.New("informes ilegibles")),
			unreadableReportsPending().Action)
	}

	finished := u.Clock.Now()
	report.Status = domain.SanitizeStatusCompleted
	report.FinishedAt = &finished
	if err := u.Artifacts.PublishReport(ctx, report); err != nil {
		return u.leavePending(ctx, report, domain.NewSanitizeError(domain.SanitizeCodePendingCleanup, err),
			"No se pudo registrar el cierre: repite mem sanitize --apply.")
	}
	// Una ejecución completa verifica también lo que dejaron pendiente las
	// anteriores, así que sus informes quedan cerrados.
	for _, prev := range previous {
		if prev.RunID == report.RunID {
			continue
		}
		prev.Status = domain.SanitizeStatusCompleted
		prev.FinishedAt = &finished
		_ = u.Artifacts.PublishReport(ctx, prev)
	}
	return report, nil
}

func (u *SanitizeProject) candidates(ctx context.Context, project string) ([]domain.SanitizeCandidate, error) {
	cands, err := u.Repo.Candidates(ctx, project)
	if err != nil {
		return nil, err
	}
	for _, c := range cands {
		if err := c.Validate(project); err != nil {
			return nil, err
		}
	}
	return cands, nil
}

// cleanArtifacts depura snapshots y regenera el contexto generado.
func (u *SanitizeProject) cleanArtifacts(ctx context.Context, project string, report *domain.SanitizeReport) error {
	arts, err := u.Artifacts.CleanBackups(ctx, project)
	report.Artifacts = append(report.Artifacts, arts...)
	if err != nil {
		return err
	}
	for _, a := range arts {
		if a.Status == domain.ArtifactPending {
			return domain.NewSanitizeError(domain.SanitizeCodePendingCleanup, errors.New(a.ErrorCode))
		}
	}
	if u.Context == nil {
		return nil
	}
	need, err := u.Artifacts.ContextNeedsRefresh(ctx)
	if err != nil {
		return err
	}
	if need {
		if err := u.Context.WriteFile(); err != nil {
			return err
		}
		report.Artifacts = append(report.Artifacts, domain.SanitizeArtifact{
			Identity: "context.md", Category: "generated_context", Status: domain.ArtifactSanitized,
		})
	}
	return nil
}

// verify relee la base y los artefactos: solo se declara completed si no
// queda nada escribible por sanear ni residuos en archivos controlados.
func (u *SanitizeProject) verify(ctx context.Context, project string, report *domain.SanitizeReport) error {
	left, err := u.Repo.Candidates(ctx, project)
	if err != nil {
		return err
	}
	for _, c := range left {
		if c.Writable() {
			return domain.NewSanitizeError(domain.SanitizeCodePendingCleanup, errors.New("candidatos remanentes"))
		}
	}
	residues, err := u.Artifacts.VerifyResidues(ctx, project)
	if err != nil {
		return err
	}
	if len(residues) > 0 {
		report.Artifacts = append(report.Artifacts, residues...)
		return domain.NewSanitizeError(domain.SanitizeCodePendingCleanup, errors.New("residuos en artefactos"))
	}
	return nil
}

// leavePending publica el informe como pending_cleanup con la acción a
// seguir y devuelve el error clasificado.
func (u *SanitizeProject) leavePending(ctx context.Context, report domain.SanitizeReport, err error, action string) (domain.SanitizeReport, error) {
	report.Status = domain.SanitizeStatusPendingCleanup
	code := domain.SanitizeCodePendingCleanup
	var se *domain.SanitizeError
	if errors.As(err, &se) {
		code = se.Code
	}
	report.Pending = append(report.Pending, domain.SanitizePending{Code: code, Action: action})
	// El informe pendiente ya existe desde antes de la transacción; este
	// segundo publish solo añade detalle. Se usa un contexto propio para que
	// un timeout vencido no impida dejar el rastro.
	_ = u.Artifacts.PublishReport(context.WithoutCancel(ctx), report)
	return report, err
}

// unreadableReportsPending es la acción ante informes de mantenimiento que
// no se pueden leer: no se borran solos porque podrían ser el único rastro de
// una limpieza a medias.
func unreadableReportsPending() domain.SanitizePending {
	return domain.SanitizePending{
		Code: domain.SanitizeCodePendingCleanup,
		Action: "Hay informes de mantenimiento ilegibles (ver artifacts): revísalos, retíralos " +
			"si ya no aplican y repite mem sanitize --apply.",
	}
}

// classifySanitize asigna un código estable a un error de saneamiento.
func classifySanitize(err error, fallback string) error {
	var se *domain.SanitizeError
	switch {
	case errors.As(err, &se):
		return err
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return domain.NewSanitizeError(domain.SanitizeCodeTimeout, err)
	default:
		return domain.NewSanitizeError(fallback, err)
	}
}
