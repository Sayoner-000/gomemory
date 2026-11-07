package usecases

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"mem/application/ports"
	"mem/domain"
)

// HistoryUnits da las fichas de las unidades HCD. Lo implementa *HCD.
type HistoryUnits interface {
	HistoryEntries(ctx context.Context) ([]HistoryEntry, error)
}

// IndexHistory es el backfill del historial de decisiones (HCD-046): crea o
// actualiza una ficha por unidad HCD y por auditoría, y retira las memorias
// sueltas de pasos de las unidades que ya tienen ficha. Sin apply solo
// informa (Q6).
type IndexHistory struct {
	Units    HistoryUnits
	Audits   ports.AuditSource
	Memories ports.HistoryMemories
	History  ports.DecisionHistory
	Project  string
}

// HistoryIndexReport resume lo que hizo, o haría, el índice.
type HistoryIndexReport struct {
	Applied    bool    `json:"applied"`
	Units      int     `json:"units"`
	Audits     int     `json:"audits"`
	Created    int     `json:"created"`
	Updated    int     `json:"updated"`
	Unchanged  int     `json:"unchanged"`
	Deleted    int     `json:"deleted"`
	DeletedIDs []int64 `json:"deleted_ids"`
}

var stepUnitID = regexp.MustCompile(`^(HCD-\d{3,}):`)

func (u *IndexHistory) Run(ctx context.Context, apply bool) (HistoryIndexReport, error) {
	rep := HistoryIndexReport{Applied: apply, DeletedIDs: []int64{}}
	entries, err := u.Units.HistoryEntries(ctx)
	if err != nil {
		return rep, err
	}
	rep.Units = len(entries)
	indexed := map[string]bool{}
	for i := range entries {
		// El backfill fija el proyecto: no depende de cómo se configuró HCD.
		entries[i].Memory.Project = u.Project
		indexed[entries[i].Memory.TopicKey[len(domain.HCDDigestTopicPrefix):]] = true
	}
	audits, err := u.auditEntries()
	if err != nil {
		return rep, err
	}
	rep.Audits = len(audits)
	for _, e := range append(entries, audits...) {
		existing, err := u.Memories.ByTopicKey(u.Project, e.Memory.TopicKey)
		if err != nil {
			return rep, err
		}
		switch {
		case existing == nil:
			rep.Created++
		case existing.Title == domain.ProtectText(e.Memory.Title) && existing.Content == domain.ProtectText(e.Memory.Content) && existing.Type == e.Memory.Type:
			rep.Unchanged++
			continue
		default:
			rep.Updated++
		}
		if apply {
			m := e.Memory
			if _, err := u.History.UpsertDigest(&m, e.Files, true); err != nil {
				return rep, fmt.Errorf("ficha %s: %w", e.Memory.TopicKey, err)
			}
		}
	}

	all, err := u.Memories.ListAll(u.Project)
	if err != nil {
		return rep, err
	}
	for _, m := range all {
		if m.Type != domain.Decision || !domain.IsHCDStepMemoryTitle(m.Title) {
			continue
		}
		if id := stepUnitID.FindStringSubmatch(m.Title); id != nil && indexed[id[1]] {
			rep.DeletedIDs = append(rep.DeletedIDs, m.ID)
		}
	}
	sort.Slice(rep.DeletedIDs, func(i, j int) bool { return rep.DeletedIDs[i] < rep.DeletedIDs[j] })
	rep.Deleted = len(rep.DeletedIDs)
	if apply && rep.Deleted > 0 {
		if _, err := u.Memories.DeleteMany(u.Project, rep.DeletedIDs); err != nil {
			return rep, fmt.Errorf("retirar memorias de pasos: %w", err)
		}
	}
	return rep, nil
}

func (u *IndexHistory) auditEntries() ([]HistoryEntry, error) {
	if u.Audits == nil {
		return nil, nil
	}
	files, err := u.Audits.ListAudits()
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	out := make([]HistoryEntry, 0, len(files))
	for _, f := range files {
		data, err := u.Audits.ReadAudit(f)
		if err != nil {
			return nil, err
		}
		d := domain.RenderAuditDigest(domain.ParseAuditReport(f, data))
		out = append(out, HistoryEntry{
			Memory: domain.Memory{Project: u.Project, Type: domain.Discovery, Title: d.Title, Content: d.Content, Filepath: f, TopicKey: domain.AuditDigestTopic(f)},
			Files:  d.Files,
		})
	}
	return out, nil
}
