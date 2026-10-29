package hcdfiles

import (
	"context"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"os"
	"path/filepath"
)

func (s *Store) ApproachCandidates(ctx context.Context) ([]domain.HCDApproachCandidate, error) {
	var out []domain.HCDApproachCandidate
	for _, base := range []struct{ root, origin string }{{filepath.Join(filepath.Dir(s.root), "specs"), "speckit"}, {filepath.Join(s.root, "features"), "hcd"}} {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		entries, e := os.ReadDir(base.root)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		for _, entry := range entries {
			if !entry.IsDir() || !unitDirectory.MatchString(entry.Name()) {
				continue
			}
			folder := filepath.Join(base.root, entry.Name())
			definition, e := os.Lstat(filepath.Join(folder, "definition.md"))
			if e != nil && !os.IsNotExist(e) {
				return nil, e
			}
			hasDefinition := e == nil && definition.Mode().IsRegular()
			spec, e := os.Lstat(filepath.Join(folder, "spec.md"))
			if e != nil && !os.IsNotExist(e) {
				return nil, e
			}
			hasSpec := e == nil && spec.Mode().IsRegular()
			relative, e := filepath.Rel(filepath.Dir(s.root), folder)
			if e != nil {
				return nil, e
			}
			out = append(out, domain.HCDApproachCandidate{Folder: filepath.ToSlash(relative), Name: entry.Name(), Origin: base.origin, HasDefinition: hasDefinition, HasSpec: hasSpec})
		}
	}
	return out, nil
}

var _ ports.HCDLegacySourceReader = (*Store)(nil)

// ReadLegacySources lee spec.md, plan.md y tasks.md de specs/<feature> sin escribir nada.
func (s *Store) ReadLegacySources(ctx context.Context, feature string) (map[string][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !unitDirectory.MatchString(feature) {
		return nil, fmt.Errorf("invalid Spec Kit directory %q: %w", feature, domain.ErrValidation)
	}
	dir := filepath.Join(filepath.Dir(s.root), "specs", feature)
	out := map[string][]byte{}
	for _, name := range domain.MigrationSourceFiles {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("unsafe Spec Kit source %q: %w", path, domain.ErrValidation)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out[name] = content
	}
	if _, ok := out["spec.md"]; !ok {
		return nil, fmt.Errorf("Spec Kit unit %s: %w", feature, domain.ErrNotFound)
	}
	return out, nil
}
