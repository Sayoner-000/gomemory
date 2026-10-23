package persistence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"mem/adapters/secondary/atomicfs"
	"mem/application/ports"
	"mem/domain"
)

// PrivacyArtifacts trata los archivos que gomemory controla fuera de la base
// durante el saneamiento histórico (spec 037, US2, FR-008): informes de
// mantenimiento, snapshots de respaldo del proyecto y contexto generado.
//
// Reglas: nunca sigue enlaces, nunca borra a ciegas un archivo que no
// reconoce y nunca crea copias crudas; las reescrituras van por temporal en
// el mismo directorio y rename.
type PrivacyArtifacts struct {
	dataHome    string
	projectKey  string
	contextPath string
}

var _ ports.PrivacyArtifactsPort = (*PrivacyArtifacts)(nil)

// NewPrivacyArtifacts construye el adaptador para un proyecto. dataHome es
// la raíz administrada (DataHome) y contextPath el .memory/context.md.
func NewPrivacyArtifacts(dataHome, projectKey, contextPath string) *PrivacyArtifacts {
	return &PrivacyArtifacts{dataHome: dataHome, projectKey: projectKey, contextPath: contextPath}
}

// Códigos de artefacto pendiente, sin contenido sensible.
const (
	artifactUnsafeLink  = "unsafe_link"
	artifactInvalidJSON = "invalid_json"
	artifactUnreadable  = "unreadable"
	artifactWriteFailed = "write_failed"
	artifactProtected   = "protected_content"
	artifactBadReport   = "unreadable_report"
)

var runIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func (a *PrivacyArtifacts) reportsDir() string {
	return filepath.Join(a.dataHome, "maintenance", "privacy")
}

func (a *PrivacyArtifacts) backupsDir() string {
	return filepath.Join(a.dataHome, "backups", a.projectKey)
}

// PublishReport escribe <run_id>.json con permisos privados y reemplazo
// atómico.
func (a *PrivacyArtifacts) PublishReport(_ context.Context, report domain.SanitizeReport) error {
	if !runIDPattern.MatchString(report.RunID) {
		return fmt.Errorf("%w: run_id inválido", domain.ErrValidation)
	}
	dir := a.reportsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("crear directorio de informes: %w", err)
	}
	_ = os.Chmod(dir, 0o700)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(dir, report.RunID+".json"), append(data, '\n'), 0o600)
}

// PendingReports lee los informes pending_cleanup del proyecto. Un informe
// que no se puede leer o decodificar se devuelve como artefacto pendiente
// (C-002, acr_9ba278e2): descartarlo permitiría declarar completed con un
// rastro de limpieza incompleta todavía en disco.
func (a *PrivacyArtifacts) PendingReports(_ context.Context, project string) ([]domain.SanitizeReport, []domain.SanitizeArtifact, error) {
	entries, err := os.ReadDir(a.reportsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var out []domain.SanitizeReport
	var unreadable []domain.SanitizeArtifact
	for _, e := range entries {
		// Los temporales de PublishReport empiezan por "." y no son informes.
		if strings.HasPrefix(e.Name(), ".") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		var r domain.SanitizeReport
		data, err := os.ReadFile(filepath.Join(a.reportsDir(), e.Name()))
		if err == nil && e.Type().IsRegular() {
			err = json.Unmarshal(data, &r)
		} else if err == nil {
			err = errors.New("no es un archivo regular")
		}
		if err != nil {
			unreadable = append(unreadable, domain.SanitizeArtifact{
				Identity: "maintenance/privacy/" + e.Name(), Category: "report",
				Status: domain.ArtifactPending, ErrorCode: artifactBadReport,
			})
			continue
		}
		if r.Project == project && r.Status == domain.SanitizeStatusPendingCleanup {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RunID < out[j].RunID })
	return out, unreadable, nil
}

// CleanBackups reescribe protegidos los snapshots del proyecto.
func (a *PrivacyArtifacts) CleanBackups(ctx context.Context, _ string) ([]domain.SanitizeArtifact, error) {
	return a.walkBackups(ctx, true)
}

// ContextNeedsRefresh dice si el contexto generado contiene contenido
// protegido. Un archivo ausente no necesita nada.
func (a *PrivacyArtifacts) ContextNeedsRefresh(context.Context) (bool, error) {
	info, err := os.Lstat(a.contextPath)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, nil
	}
	data, err := os.ReadFile(a.contextPath)
	if err != nil {
		return false, err
	}
	return domain.ContainsProtected(string(data)), nil
}

// VerifyResidues revisa snapshots y contexto sin modificarlos.
func (a *PrivacyArtifacts) VerifyResidues(ctx context.Context, _ string) ([]domain.SanitizeArtifact, error) {
	arts, err := a.walkBackups(ctx, false)
	if err != nil {
		return nil, err
	}
	var residues []domain.SanitizeArtifact
	for _, art := range arts {
		if art.Status == domain.ArtifactPending {
			residues = append(residues, art)
		}
	}
	if need, err := a.ContextNeedsRefresh(ctx); err != nil {
		return nil, err
	} else if need {
		residues = append(residues, domain.SanitizeArtifact{
			Identity: "context.md", Category: "generated_context", Status: domain.ArtifactPending, ErrorCode: artifactProtected,
		})
	}
	return residues, nil
}

// walkBackups recorre los snapshots; con write=false solo clasifica.
func (a *PrivacyArtifacts) walkBackups(ctx context.Context, write bool) ([]domain.SanitizeArtifact, error) {
	dir := a.backupsDir()
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []domain.SanitizeArtifact
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		identity := filepath.ToSlash(filepath.Join("backups", a.projectKey, e.Name()))
		art := domain.SanitizeArtifact{Identity: identity, Category: "backup"}
		path := filepath.Join(dir, e.Name())
		art.Status, art.ErrorCode = a.treatBackup(path, write)
		out = append(out, art)
	}
	return out, nil
}

func (a *PrivacyArtifacts) treatBackup(path string, write bool) (status, code string) {
	info, err := os.Lstat(path)
	if err != nil {
		return domain.ArtifactPending, artifactUnreadable
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return domain.ArtifactPending, artifactUnsafeLink
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.ArtifactPending, artifactUnreadable
	}
	// Recorrido genérico (S-02, acr_9ba278e2): un snapshot de otra versión
	// con campos que esta no conoce se sanea igual y los conserva. Solo un
	// archivo que no es JSON queda pendiente, con un código propio que no se
	// confunde con contenido protegido.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil || dec.More() {
		return domain.ArtifactPending, artifactInvalidJSON
	}
	clean, changed := protectJSON(doc)
	if !changed {
		return domain.ArtifactClean, ""
	}
	if !write {
		return domain.ArtifactPending, artifactProtected
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(clean); err != nil {
		return domain.ArtifactPending, artifactWriteFailed
	}
	if err := writeFileAtomic(path, out.Bytes(), 0o600); err != nil {
		return domain.ArtifactPending, artifactWriteFailed
	}
	return domain.ArtifactSanitized, ""
}

// protectJSON aplica la protección a todos los textos de un documento JSON
// decodificado (claves incluidas) y dice si cambió algo. Números
// (json.Number), booleanos y estructura quedan intactos.
func protectJSON(v any) (any, bool) {
	switch t := v.(type) {
	case string:
		p := domain.ProtectText(t)
		return p, p != t
	case []any:
		changed := false
		for i, item := range t {
			var c bool
			t[i], c = protectJSON(item)
			changed = changed || c
		}
		return t, changed
	case map[string]any:
		changed := false
		out := make(map[string]any, len(t))
		for k, item := range t {
			pk := domain.ProtectText(k)
			pv, c := protectJSON(item)
			out[pk] = pv
			changed = changed || c || pk != k
		}
		return out, changed
	default:
		return v, false
	}
}

// writeFileAtomic escribe en un temporal del mismo directorio, sincroniza,
// cierra y renombra sobre el destino.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	cleanup := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		return cleanup(err)
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(err)
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := atomicfs.Replace(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}
