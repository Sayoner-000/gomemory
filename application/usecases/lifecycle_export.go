package usecases

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"mem/application/ports"
)

// ExportTarget es un proyecto del almacén que se exporta antes de borrarlo.
// Root puede ir vacío: los proyectos anteriores al registro no la tienen.
type ExportTarget struct {
	Key, Root string
}

// ExportIndexEntry es una línea de index.json (FR-009a).
type ExportIndexEntry struct {
	Key       string `json:"key"`
	Root      string `json:"root"`
	File      string `json:"file"`
	Memories  int    `json:"memories"`
	Relations int    `json:"relations"`
}

// OpenProjectStore abre los repositorios de un proyecto por su clave. ok=false
// si ese proyecto no tiene base (nada que exportar).
type OpenProjectStore func(key string) (mem ports.MemoryRepository, rel ports.RelationRepository, closeFn func(), ok bool, err error)

// ExportProjects exporta cada proyecto a <dir>/<clave>.json con el formato de
// `mem export` y escribe <dir>/index.json. Todo con permisos privados: el
// directorio 0700 y los archivos 0600 (FR-009a).
//
// Devuelve el índice de lo exportado y los errores por clave. Quien llama NO
// debe borrar la memoria de una clave con error (FR-009).
func ExportProjects(dir string, targets []ExportTarget, open OpenProjectStore) ([]ExportIndexEntry, map[string]error) {
	failed := map[string]error{}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		for _, t := range targets {
			failed[t.Key] = fmt.Errorf("crear %s: %w", dir, err)
		}
		return nil, failed
	}
	_ = os.Chmod(dir, 0o700)

	var index []ExportIndexEntry
	for _, t := range targets {
		entry, ok, err := exportOne(dir, t, open)
		if err != nil {
			failed[t.Key] = err
			continue
		}
		if ok {
			index = append(index, entry)
		}
	}
	if err := writePrivateJSON(filepath.Join(dir, "index.json"), index); err != nil {
		for _, e := range index {
			failed[e.Key] = fmt.Errorf("escribir index.json: %w", err)
		}
	}
	return index, failed
}

func exportOne(dir string, t ExportTarget, open OpenProjectStore) (ExportIndexEntry, bool, error) {
	mem, rel, closeFn, ok, err := open(t.Key)
	if err != nil || !ok {
		return ExportIndexEntry{}, false, err
	}
	defer closeFn()
	bundle, err := ExportProject(mem, rel, t.Key)
	if err != nil {
		return ExportIndexEntry{}, false, err
	}
	file := t.Key + ".json"
	f, err := os.OpenFile(filepath.Join(dir, file), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return ExportIndexEntry{}, false, err
	}
	_ = f.Chmod(0o600)
	if err := EncodeBundle(f, bundle); err != nil {
		_ = f.Close()
		return ExportIndexEntry{}, false, err
	}
	if err := f.Close(); err != nil {
		return ExportIndexEntry{}, false, err
	}
	return ExportIndexEntry{
		Key: t.Key, Root: t.Root, File: file,
		Memories: len(bundle.Memories), Relations: len(bundle.Relations),
	}, true, nil
}

func writePrivateJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_ = f.Chmod(0o600)
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
