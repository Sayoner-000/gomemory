package usecases

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"mem/application/ports"
)

// CreateSnapshot exporta project a un archivo JSON con timestamp bajo backupDir
// (mismo formato que ExportProject/EncodeBundle ya generan para `mem export`,
// sin modificarlos) y poda los snapshots más antiguos si se supera keep
// (keep<=0 desactiva la poda). Mitiga la ausencia de backup automático entre
// máquinas (specs/009-mitigacion-riesgos) reutilizando el export ya existente
// en vez de copiar el archivo mem.db crudo, que en modo WAL puede quedar en un
// estado inconsistente si se copia a mitad de una transacción.
//
// El caller decide qué hacer con el error: hookSessionEnd lo descarta
// (best-effort, nunca debe abortar el cierre de sesión); otros callers pueden
// propagarlo.
func CreateSnapshot(memRepo ports.MemoryRepository, relRepo ports.RelationRepository, project, backupDir string, keep int) (string, error) {
	bundle, err := ExportProject(memRepo, relRepo, project)
	if err != nil {
		return "", fmt.Errorf("create snapshot: %w", err)
	}

	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", fmt.Errorf("create snapshot: crear directorio de backups: %w", err)
	}

	name := time.Now().UTC().Format("20060102-150405.000000000") + ".json"
	path := filepath.Join(backupDir, name)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("create snapshot: crear archivo: %w", err)
	}
	defer f.Close()

	if err := EncodeBundle(f, bundle); err != nil {
		return "", fmt.Errorf("create snapshot: escribir bundle: %w", err)
	}

	if keep > 0 {
		pruneSnapshots(backupDir, keep) // best-effort: no afecta el snapshot recién creado
	}

	return path, nil
}

// pruneSnapshots conserva únicamente los `keep` snapshots más recientes (por
// fecha de modificación) en dir, eliminando el resto. Best-effort: cualquier
// error de lectura del directorio o de borrado se ignora — el snapshot que
// CreateSnapshot acaba de escribir ya existe y no debe perderse por esto.
func pruneSnapshots(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var files []fileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{path: filepath.Join(dir, e.Name()), modTime: info.ModTime()})
	}
	if len(files) <= keep {
		return
	}

	sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })
	for _, f := range files[:len(files)-keep] {
		os.Remove(f.path)
	}
}
