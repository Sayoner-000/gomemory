package expandstate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// ShieldState implementa ports.ShieldStateRepository: la huella del último
// escaneo en .expand-shield-hash y el informe en expand-shield-last.json.
type ShieldState struct {
	hashPath   string
	reportPath string
}

var _ ports.ShieldStateRepository = (*ShieldState)(nil)

func NewShieldState(memDir string) *ShieldState {
	return &ShieldState{
		hashPath:   filepath.Join(memDir, ".expand-shield-hash"),
		reportPath: filepath.Join(memDir, "expand-shield-last.json"),
	}
}

func (s *ShieldState) LastHash(ctx context.Context) string {
	if ctx.Err() != nil {
		return ""
	}
	data, err := os.ReadFile(s.hashPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Save escribe primero el informe y después la huella: si el proceso muere
// entre ambos, el siguiente arranque vuelve a escanear en vez de dar por
// visto un informe que no se guardó.
func (s *ShieldState) Save(ctx context.Context, report domain.ShieldReport) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(s.reportPath, data); err != nil {
		return err
	}
	return writeAtomic(s.hashPath, []byte(report.Hash))
}

func (s *ShieldState) Last(ctx context.Context) (domain.ShieldReport, bool) {
	var r domain.ShieldReport
	if ctx.Err() != nil {
		return r, false
	}
	data, err := os.ReadFile(s.reportPath)
	if err != nil || json.Unmarshal(data, &r) != nil {
		return domain.ShieldReport{}, false
	}
	// El informe puede venir en un clon: se depura al leer (H-03).
	return r.Sanitized(), true
}
