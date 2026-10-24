package usecases

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"mem/application/ports"
)

// snapshotNow es el reloj de CreateSnapshot; las pruebas lo fijan para
// reproducir la marca repetida del reloj de Windows.
var snapshotNow = time.Now

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

	// El reloj de Windows puede repetir la marca entre dos snapshots seguidos:
	// el archivo se crea en exclusiva y el contador evita sobrescribir el
	// anterior. El contador de dos cifras mantiene el orden lexicográfico.
	stamp := snapshotNow().UTC().Format("20060102-150405.000000000")
	var (
		f    *os.File
		path string
	)
	for n := 0; ; n++ {
		path = filepath.Join(backupDir, fmt.Sprintf("%s-%02d.json", stamp, n))
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrExist) || n == 99 {
			return "", fmt.Errorf("create snapshot: crear archivo: %w", err)
		}
	}
	if err := EncodeBundle(f, bundle); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return "", fmt.Errorf("create snapshot: escribir bundle: %w; cerrar archivo: %v", err, closeErr)
		}
		return "", fmt.Errorf("create snapshot: escribir bundle: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("create snapshot: cerrar archivo: %w", err)
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

	// Con la misma fecha de modificación decide el nombre, que lleva la marca.
	sort.Slice(files, func(i, j int) bool {
		if !files[i].modTime.Equal(files[j].modTime) {
			return files[i].modTime.Before(files[j].modTime)
		}
		return files[i].path < files[j].path
	})
	for _, f := range files[:len(files)-keep] {
		_ = os.Remove(f.path)
	}
}
