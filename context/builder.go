package context

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mem/store"
	"mem/types"
)

func displayTitle(m types.Memory) string {
	if m.Title != "" {
		return m.Title
	}
	// Use first 60 chars of content as title fallback
	r := []rune(m.Content)
	if len(r) > 60 {
		return string(r[:57]) + "..."
	}
	return string(r)
}

type Builder struct {
	DB      *sql.DB
	Project string
	Root    string
}

func New(db *sql.DB, root, project string) *Builder {
	return &Builder{DB: db, Project: project, Root: root}
}

func (b *Builder) Build() (string, error) {
	mems, err := store.ListMemories(b.DB, b.Project, 100)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("# Memoria del Proyecto\n\n")

	byType := make(map[types.MemoryType][]types.Memory)
	for _, m := range mems {
		byType[m.Type] = append(byType[m.Type], m)
	}

	if arch, ok := byType[types.Architecture]; ok {
		sb.WriteString("## Decisiones de Arquitectura\n\n")
		for _, m := range arch {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", displayTitle(m), m.Content))
			if m.Filepath != "" {
				sb.WriteString(fmt.Sprintf("  → `%s`\n", m.Filepath))
			}
		}
		sb.WriteString("\n")
	}

	if dec, ok := byType[types.Decision]; ok {
		sb.WriteString("## Decisiones Técnicas\n\n")
		for _, m := range dec {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", displayTitle(m), m.Content))
		}
		sb.WriteString("\n")
	}

	if pat, ok := byType[types.Pattern]; ok {
		sb.WriteString("## Patrones y Convenciones\n\n")
		for _, m := range pat {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", displayTitle(m), m.Content))
		}
		sb.WriteString("\n")
	}

	if bugs, ok := byType[types.Bugfix]; ok {
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
		if m.Type == types.Architecture || m.Type == types.Decision || m.Type == types.Pattern || m.Type == types.Bugfix {
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

	sess, _ := store.ActiveSession(b.DB, b.Project)
	if sess != nil {
		sb.WriteString(fmt.Sprintf("## Sesión Activa\n\n- Iniciada: %s\n", sess.CreatedAt))
		sb.WriteString("\n")
	}

	sessions, _ := store.RecentSessions(b.DB, b.Project, 5)
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
	path := filepath.Join(b.Root, store.MemDir, "context.md")
	return os.WriteFile(path, []byte(content), 0644)
}
