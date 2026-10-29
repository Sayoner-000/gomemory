package usecases

import (
	"context"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"path"
	"strings"
	"time"
)

type HCDMigrateActivateInput struct {
	MigrationID, Revision, Quote, Client, Implementer string
	Kind                                              domain.Kind
}

func (h *HCD) legacySources(ctx context.Context, feature string) (map[string][]byte, error) {
	reader, ok := h.artifacts.(ports.HCDLegacySourceReader)
	if !ok {
		return nil, fmt.Errorf("legacy Spec Kit sources are not readable: %w", domain.ErrNotAllowed)
	}
	return reader.ReadLegacySources(ctx, feature)
}

// MigratePreview genera la propuesta clasificada sin activar HCD ni tocar la unidad Spec Kit (C-20).
func (h *HCD) MigratePreview(ctx context.Context, feature string) (domain.Migration, error) {
	sources, err := h.legacySources(ctx, feature)
	if err != nil {
		return domain.Migration{}, err
	}
	m := domain.Migration{SourceDir: "specs/" + feature, SourceRev: domain.MigrationSourceRevision(sources), Status: domain.MigrationProposed, Items: domain.ClassifyMigration(sources), CreatedAt: h.clock.Now().UTC().Format(time.RFC3339Nano)}
	m.Recount()
	err = h.uow.WithTransaction(ctx, func(repo ports.HCDRepository) error {
		previous, e := repo.ListMigrations(ctx)
		if e != nil {
			return e
		}
		for _, p := range previous {
			if p.SourceDir != m.SourceDir {
				continue
			}
			if p.Status == domain.MigrationActivated {
				return domain.ErrAlreadyExists
			}
			if p.Status == domain.MigrationProposed {
				p.Status = domain.MigrationStale
				if e = repo.SaveMigration(ctx, p); e != nil {
					return e
				}
			}
		}
		if m.ID, e = repo.NextMigrationID(ctx); e != nil {
			return e
		}
		return repo.SaveMigration(ctx, m)
	})
	return m, err
}

// MigrateActivate crea la unidad HCD en una única transacción; si falla, la unidad sigue siendo Spec Kit.
func (h *HCD) MigrateActivate(ctx context.Context, in HCDMigrateActivateInput) (domain.Unit, error) {
	if in.Client == "tui" {
		return domain.Unit{}, domain.ErrNotAllowed
	}
	if in.Kind == "" {
		in.Kind = domain.KindFeature
	}
	m, err := h.repo.GetMigration(ctx, in.MigrationID)
	if err != nil {
		return domain.Unit{}, err
	}
	if m.Status == domain.MigrationStale {
		return domain.Unit{}, domain.ErrRevisionConflict
	}
	if m.Status != domain.MigrationProposed {
		return domain.Unit{}, domain.ErrNotAllowed
	}
	if m.Revision != in.Revision {
		return domain.Unit{}, domain.ErrRevisionConflict
	}
	if !domain.ValidQuote(in.Quote, in.Revision) {
		return domain.Unit{}, domain.ErrInvalidQuote
	}
	sources, err := h.legacySources(ctx, path.Base(m.SourceDir))
	if err != nil {
		return domain.Unit{}, err
	}
	if domain.MigrationSourceRevision(sources) != m.SourceRev {
		m.Status = domain.MigrationStale
		if err = h.uow.WithTransaction(ctx, func(repo ports.HCDRepository) error { return repo.SaveMigration(ctx, m) }); err != nil {
			return domain.Unit{}, err
		}
		return domain.Unit{}, domain.ErrRevisionConflict
	}
	playbook, err := ResolvePinnedDoc(h.topics, h.project, domain.TopicHCDPlaybook, h.playbookTemplate)
	if err != nil {
		return domain.Unit{}, err
	}
	constitution, err := ResolvePinnedDoc(h.topics, h.project, domain.TopicConstitution, h.constitutionTemplate)
	if err != nil {
		return domain.Unit{}, err
	}
	playbookRev := domain.ContentRevision([]byte(playbook.Content))
	constitutionRev := domain.ContentRevision([]byte(constitution.Content))
	name := path.Base(m.SourceDir)
	title, slug := legacyTitle(sources["spec.md"], name), hcdSlug(strings.SplitN(name, "-", 2)[1])
	implementer := domain.ProtectHCDField(in.Implementer)
	var unit domain.Unit
	unitDir := ""
	err = h.uow.WithTransaction(ctx, func(repo ports.HCDRepository) error {
		id, e := repo.NextUnitID(ctx)
		if e != nil {
			return e
		}
		dir := strings.TrimPrefix(id, "HCD-") + "-" + slug
		if exists, e := h.artifacts.Exists(ctx, dir, "definition.md"); e != nil || exists {
			if e == nil {
				e = domain.ErrAlreadyExists
			}
			return e
		}
		definition := strings.ReplaceAll(h.definitionTemplate, "HCD-001", id)
		definition = strings.ReplaceAll(definition, "{Título}", title)
		definition = strings.ReplaceAll(definition, "Tipo: fix", "Tipo: "+string(in.Kind))
		definition = strings.ReplaceAll(definition, "{persona}", implementer)
		definition = strings.ReplaceAll(definition, "{revisión}", playbookRev)
		unit = domain.Unit{ID: id, Kind: in.Kind, Title: title, Slug: slug, Approach: "hcd", State: domain.StateDraft, StateRevision: 1, DefinitionRev: domain.ContentRevision([]byte(definition)), PlaybookRev: playbookRev, ConstitutionRev: constitutionRev, Implementer: implementer}
		// Como en Start: el INSERT reserva el ID antes de escribir el artefacto,
		// y unitDir solo se fija cuando el archivo es de esta llamada.
		if e = repo.CreateUnit(ctx, unit); e != nil {
			return e
		}
		if e = h.artifacts.Write(ctx, dir, "definition.md", []byte(definition)); e != nil {
			return e
		}
		unitDir = dir
		if e = repo.SaveNormativeRevision(ctx, "playbook", playbookRev, playbook.Content, docSource(playbook.FromMemory), "initial", false); e != nil {
			return e
		}
		if e = repo.SaveNormativeRevision(ctx, "constitution", constitutionRev, constitution.Content, docSource(constitution.FromMemory), "initial", false); e != nil {
			return e
		}
		approval, e := h.governanceApproval(ctx, repo, unit, "migration:"+m.ID, in.Revision, in.Quote, in.Client)
		if e != nil {
			return e
		}
		for _, file := range domain.MigrationSourceFiles {
			if _, ok := sources[file]; !ok {
				continue
			}
			if e = repo.AddLink(ctx, domain.Link{UnitID: id, FromKind: "unit", FromID: id, ToKind: "legacy-source", ToID: m.SourceDir + "/" + file, Relation: "legacy-reference"}); e != nil {
				return e
			}
		}
		for _, d := range []domain.HCDDecision{
			{UnitID: id, Kind: "approach", Status: "accepted", Summary: "HCD", Revision: unit.DefinitionRev, Refs: []string{id}, Source: "migrate", MemorySync: "pending"},
			{UnitID: id, Kind: "migration", Status: "accepted", Summary: "Migrar " + m.SourceDir + " a HCD (" + m.ID + ")", Revision: in.Revision, Refs: []string{m.ID, approval}, Source: "migrate", MemorySync: "pending"},
		} {
			if e = recordDecisionInRepo(ctx, repo, &d); e != nil {
				return e
			}
		}
		m.UnitID, m.Status, m.ActivatedAt = id, domain.MigrationActivated, h.clock.Now().UTC().Format(time.RFC3339Nano)
		return repo.SaveMigration(ctx, m)
	})
	if err != nil {
		if unitDir != "" {
			cctx, cancel := compensationContext(ctx)
			cleanup := h.artifacts.Delete(cctx, unitDir, "definition.md")
			cancel()
			if cleanup != nil {
				return domain.Unit{}, fmt.Errorf("activate migration: %w (artifact cleanup: %v)", err, cleanup)
			}
		}
		return domain.Unit{}, fmt.Errorf("activate migration: %w", err)
	}
	h.syncDecisionsAfterCommit(ctx, unit.ID)
	return unit, nil
}

// legacyTitle toma el título del encabezado de spec.md («# Feature Specification: X»).
func legacyTitle(spec []byte, fallback string) string {
	for _, line := range strings.Split(string(spec), "\n") {
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimSpace(strings.TrimPrefix(line, "# "))
			if _, after, ok := strings.Cut(title, ":"); ok && strings.TrimSpace(after) != "" {
				title = strings.TrimSpace(after)
			}
			return domain.ProtectHCDField(title)
		}
	}
	return domain.ProtectHCDField(fallback)
}
