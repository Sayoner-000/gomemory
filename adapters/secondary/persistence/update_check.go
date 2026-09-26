package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"mem/application/ports"
	"mem/domain"
)

// updateCheckFile es la caché de versiones, relativa al almacén global
// (feature 034, R4). Es del usuario: la comparten todos sus proyectos.
const updateCheckFile = "update-check.json"

// UpdateCheckRepository implementa ports.UpdateCheckRepository.
type UpdateCheckRepository struct{}

var _ ports.UpdateCheckRepository = UpdateCheckRepository{}

func updateCheckPath() (string, error) {
	home, err := DataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, updateCheckFile), nil
}

// Read trata una caché ausente, ilegible o corrupta como inexistente: la
// consecuencia es solo una consulta más, nunca un aviso erróneo.
func (UpdateCheckRepository) Read(_ context.Context) (domain.UpdateCheck, bool) {
	path, err := updateCheckPath()
	if err != nil {
		return domain.UpdateCheck{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.UpdateCheck{}, false
	}
	var c domain.UpdateCheck
	if err := json.Unmarshal(data, &c); err != nil {
		return domain.UpdateCheck{}, false
	}
	return c, true
}

// Write escribe de forma atómica (temporal + rename): varias sesiones pueden
// consultar a la vez y ninguna debe leer un JSON a medias.
func (UpdateCheckRepository) Write(_ context.Context, c domain.UpdateCheck) error {
	path, err := updateCheckPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".update-check-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	_, werr := tmp.Write(append(data, '\n'))
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
