package usecases

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

const memDir = ".memory"

func displayTitle(m domain.Memory) string {
	if m.Title != "" {
		return m.Title
	}
	r := []rune(m.Content)
	if len(r) > 60 {
		return string(r[:57]) + "..."
	}
	return string(r)
}

type Builder struct {
	Lister    ports.MemoryLister
	Session   ports.SessionQuerier
	Relations ports.RelationLister
	Project   string
	Root      string
}

func New(lister ports.MemoryLister, session ports.SessionQuerier, relations ports.RelationLister, root, project string) *Builder {
	return &Builder{Lister: lister, Session: session, Relations: relations, Project: project, Root: root}
}

func (b *Builder) Build() (string, error) {
	mems, err := b.Lister.List(b.Project, 100)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("# Memoria del Proyecto\n\n")

	byType := make(map[domain.MemoryType][]domain.Memory)
	titleByID := make(map[int64]string, len(mems))
	for _, m := range mems {
		byType[m.Type] = append(byType[m.Type], m)
		titleByID[m.ID] = displayTitle(m)
	}

	if b.Relations != nil {
		if rels, err := b.Relations.List(b.Project, 200); err == nil {
			var conflicts []domain.Relation
			for _, r := range rels {
				if r.Relation == domain.ConflictsWith {
					conflicts = append(conflicts, r)
				}
			}
			if len(conflicts) > 0 {
				sb.WriteString("## ⚠ Conflictos sin resolver\n\n")
				for _, r := range conflicts {
					titleA := titleByID[r.MemoryIDA]
					titleB := titleByID[r.MemoryIDB]
					sb.WriteString(fmt.Sprintf("- [%d] %q ↔ [%d] %q — releé el código actual y llamá a judge_memories para resolverlo\n",
						r.MemoryIDA, titleA, r.MemoryIDB, titleB))
				}
				sb.WriteString("\n")
			}
		}
	}

	if arch, ok := byType[domain.Architecture]; ok {
		sb.WriteString("## Decisiones de Arquitectura\n\n")
		for _, m := range arch {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", displayTitle(m), m.Content))
			if m.Filepath != "" {
				sb.WriteString(fmt.Sprintf("  → `%s`\n", m.Filepath))
			}
		}
		sb.WriteString("\n")
	}

	if dec, ok := byType[domain.Decision]; ok {
		sb.WriteString("## Decisiones Técnicas\n\n")
		for _, m := range dec {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", displayTitle(m), m.Content))
		}
		sb.WriteString("\n")
	}

	if pat, ok := byType[domain.Pattern]; ok {
		sb.WriteString("## Patrones y Convenciones\n\n")
		for _, m := range pat {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", displayTitle(m), m.Content))
		}
		sb.WriteString("\n")
	}

	if bugs, ok := byType[domain.Bugfix]; ok {
		sb.WriteString("## Bugfixes\n\n")
		for _, m := range bugs {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", displayTitle(m), m.Content))
			if m.Filepath != "" {
				sb.WriteString(fmt.Sprintf("  → `%s`\n", m.Filepath))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Aprendizajes Recientes\n\n")
	count := 0
	for _, m := range mems {
		if m.Type == domain.Architecture || m.Type == domain.Decision || m.Type == domain.Pattern || m.Type == domain.Bugfix || m.Type == domain.Checkpoint {
			continue
		}
		if count >= 15 {
			break
		}
		line := fmt.Sprintf("- %s", m.Content)
		if m.Title != "" {
			line = fmt.Sprintf("- **%s**: %s", m.Title, m.Content)
		}
		sb.WriteString(line)
		if m.Filepath != "" {
			sb.WriteString(fmt.Sprintf(" (`%s`)", m.Filepath))
		}
		sb.WriteString("\n")
		count++
	}
	sb.WriteString("\n")

	if checkpoints, ok := byType[domain.Checkpoint]; ok {
		sb.WriteString("## Actividad Reciente (auto)\n\n")
		for i, m := range checkpoints {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", m.Content))
		}
		sb.WriteString("\n")
	}

	sess, _ := b.Session.Active(b.Project)
	if sess != nil {
		sb.WriteString(fmt.Sprintf("## Sesión Activa\n\n- Iniciada: %s\n", sess.CreatedAt))
		sb.WriteString("\n")
	}

	sessions, _ := b.Session.Recent(b.Project, 5)
	if len(sessions) > 0 {
		sb.WriteString("## Sesiones Recientes\n\n")
		for _, s := range sessions {
			if s.EndedAt == nil {
				continue
			}
			summary := strings.TrimSpace(s.Summary)
			if summary == "" {
				summary = "(sin resumen)"
			}
			sb.WriteString(fmt.Sprintf("- %s → %s: %s\n", s.CreatedAt, *s.EndedAt, summary))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func (b *Builder) WriteFile() error {
	content, err := b.Build()
	if err != nil {
		return err
	}
	path := filepath.Join(b.Root, memDir, "context.md")
	return os.WriteFile(path, []byte(content), 0644)
}
