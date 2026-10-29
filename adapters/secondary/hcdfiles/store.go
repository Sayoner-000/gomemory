package hcdfiles

import (
	"context"
	"errors"
	"fmt"
	"mem/application/ports"
	"mem/domain"
	"os"
	"path/filepath"
	"regexp"
)

var unitDirectory = regexp.MustCompile(`^[0-9]{3}-[a-z0-9]+(?:-[a-z0-9]+)*$`)

type Store struct{ root string }

func NewStore(projectRoot string) (*Store, error) {
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("resolve project root symlinks: %w", err)
	}
	return &Store{root: filepath.Join(abs, "hcd")}, nil
}

var _ ports.HCDArtifactRepository = (*Store)(nil)

func (s *Store) Root() string { return s.root }

func (s *Store) path(unitDir, name string) (string, error) {
	if !unitDirectory.MatchString(unitDir) {
		return "", fmt.Errorf("invalid HCD unit directory %q", unitDir)
	}
	if name != "definition.md" && name != "delivery.md" && name != "verification.md" {
		return "", fmt.Errorf("unsupported HCD artifact %q", name)
	}
	features := filepath.Join(s.root, "features")
	return filepath.Join(features, unitDir, name), nil
}

func (s *Store) safeParent(path string, create bool) error {
	for _, dir := range []string{s.root, filepath.Join(s.root, "features"), filepath.Dir(path)} {
		info, err := os.Lstat(dir)
		if os.IsNotExist(err) && create {
			if err := os.Mkdir(dir, 0o755); err != nil && !os.IsExist(err) {
				return err
			}
			info, err = os.Lstat(dir)
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("unsafe HCD directory %q", dir)
		}
	}
	return nil
}

func (s *Store) Read(ctx context.Context, unitDir, name string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, err := s.path(unitDir, name)
	if err != nil {
		return nil, err
	}
	if err = s.safeParent(p, false); err != nil {
		return nil, err
	}
	info, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("unsafe HCD artifact %q", name)
	}
	return os.ReadFile(p)
}

func (s *Store) Write(ctx context.Context, unitDir, name string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := s.path(unitDir, name)
	if err != nil {
		return err
	}
	if err = s.safeParent(p, true); err != nil {
		return err
	}
	if info, e := os.Lstat(p); e == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return fmt.Errorf("unsafe HCD artifact %q", name)
	} else if e != nil && !os.IsNotExist(e) {
		return e
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".hcd-*.tmp")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(temp, p); err != nil {
		return fmt.Errorf("replace HCD artifact: %w", err)
	}
	return nil
}

func (s *Store) Hash(ctx context.Context, unitDir, name string) (string, error) {
	data, err := s.Read(ctx, unitDir, name)
	if err != nil {
		return "", err
	}
	return domain.ContentRevision(data), nil
}

func (s *Store) Delete(ctx context.Context, unitDir, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := s.path(unitDir, name)
	if err != nil {
		return err
	}
	if err = s.safeParent(p, false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	info, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("unsafe HCD artifact %q", name)
	}
	if err = os.Remove(p); err != nil {
		return err
	}
	// Una carpeta vacía se vería como unidad huérfana; Remove no toca carpetas con contenido.
	_ = os.Remove(filepath.Dir(p))
	return nil
}
func (s *Store) Exists(ctx context.Context, unitDir, name string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	p, err := s.path(unitDir, name)
	if err != nil {
		return false, err
	}
	if err = s.safeParent(p, false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	info, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("unsafe HCD artifact %q", name)
	}
	return true, nil
}
func (s *Store) ListUnits(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir := filepath.Join(s.root, "features")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && unitDirectory.MatchString(entry.Name()) {
			out = append(out, entry.Name())
		}
	}
	return out, nil
}
