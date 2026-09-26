package persistence

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// registryFile guarda, dentro del directorio de cada proyecto en el almacén
// global, la ruta del proyecto en disco. La clave del almacén (slug + hash)
// no se puede invertir, y sin esta ruta la desinstalación de sistema no
// sabría dónde retirar la integración de cada proyecto (feature 034, FR-013).
const registryFile = "root"

// RegisterProject anota la ruta del proyecto en el almacén. Idempotente:
// reescribir la misma ruta no cambia nada.
func RegisterProject(root string) error {
	root = filepath.Clean(root)
	dir, err := GlobalProjectDir(ProjectKey(root))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, registryFile)
	if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) == root {
		return nil
	}
	tmp, err := os.CreateTemp(dir, ".root-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_, werr := tmp.WriteString(root + "\n")
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmpPath)
		return errors.Join(werr, cerr)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// ListRegisteredProjects devuelve los proyectos con ruta registrada. Los
// directorios del almacén sin registro (instalaciones anteriores) no salen
// aquí: para esos está el escaneo.
func ListRegisteredProjects() ([]domain.ProjectRegistration, error) {
	home, err := DataHome()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(home, "projects"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []domain.ProjectRegistration
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(home, "projects", e.Name(), registryFile))
		if err != nil {
			continue
		}
		root := strings.TrimSpace(string(data))
		if root == "" {
			continue
		}
		_, statErr := os.Stat(root)
		out = append(out, domain.ProjectRegistration{Key: e.Name(), Root: root, Exists: statErr == nil})
	}
	return out, nil
}

// ScanProjects busca proyectos de gomemory (un directorio con
// .memory/settings.json) bajo root, hasta maxDepth niveles. No sigue enlaces
// simbólicos ni entra en directorios pesados (domain.ShouldSkipScanDir). Lo
// que no se puede leer se devuelve en skipped y el recorrido sigue.
func ScanProjects(root string, maxDepth int) (found, skipped []string, err error) {
	root = filepath.Clean(root)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path != root {
				skipped = append(skipped, path)
			}
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && domain.ShouldSkipScanDir(d.Name()) {
			return fs.SkipDir
		}
		rel, _ := filepath.Rel(root, path)
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, string(filepath.Separator)) + 1
		}
		if depth > maxDepth {
			return fs.SkipDir
		}
		if info, err := os.Stat(filepath.Join(path, MemDir, "settings.json")); err == nil && info.Mode().IsRegular() {
			found = append(found, path)
		}
		return nil
	})
	return found, skipped, err
}

// RegistryRepository es el adaptador de ports.ProjectRegistryRepository.
type RegistryRepository struct{}

var _ ports.ProjectRegistryRepository = RegistryRepository{}

func (RegistryRepository) Register(_ context.Context, root string) error {
	return RegisterProject(root)
}

func (RegistryRepository) List(_ context.Context) ([]domain.ProjectRegistration, error) {
	return ListRegisteredProjects()
}

func (RegistryRepository) Scan(_ context.Context, root string, maxDepth int) ([]string, []string, error) {
	return ScanProjects(root, maxDepth)
}
