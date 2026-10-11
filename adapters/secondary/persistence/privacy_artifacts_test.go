package persistence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mem/domain"
)

type artifacts037 struct {
	dataHome, key, contextPath string
	arts                       *PrivacyArtifacts
}

func newArtifacts037(t *testing.T) artifacts037 {
	t.Helper()
	root := t.TempDir()
	a := artifacts037{
		dataHome:    filepath.Join(root, "data"),
		key:         "proj-abc",
		contextPath: filepath.Join(root, "project", ".memory", "context.md"),
	}
	a.arts = NewPrivacyArtifacts(a.dataHome, a.key, a.contextPath)
	return a
}

func (a artifacts037) backup(t *testing.T, name, body string) string {
	t.Helper()
	dir := filepath.Join(a.dataHome, "backups", a.key)
	mustMkdirAll(t, dir)
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func mustMkdirAll(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
}

func protectedBundle037(t *testing.T) string {
	t.Helper()
	b := domain.ExportBundle{Version: domain.ExportVersion, Source: "proj-abc",
		Memories:  []domain.ExportMemory{{RefID: 1, Type: "decision", Title: "t " + credMarker, Content: protectedIn, OriginPrompt: protectedIn, CreatedAt: "2025-01-01 00:00:00", UpdatedAt: "2025-01-01 00:00:00"}},
		Relations: []domain.ExportRelation{{RefA: 1, RefB: 2, Relation: "related", Confidence: 0.5, Reasoning: protectedIn, CreatedAt: "2024-01-01 00:00:00"}}}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + "\n"
}

// TestPrivacyArtifacts_PublicaInformeAtomicoYPrivado cubre FR-008: informe
// depurado en DataHome/maintenance/privacy, directorio 0700, archivo 0600,
// reemplazo sin temporales residuales y lectura de pendientes por proyecto.
func TestPrivacyArtifacts_PublicaInformeAtomicoYPrivado(t *testing.T) {
	a := newArtifacts037(t)
	ctx := context.Background()
	r := domain.NewSanitizeReport("proj-abc", domain.SanitizeModeApply)
	r.RunID = "run-1"
	if err := a.arts.PublishReport(ctx, r); err != nil {
		t.Fatalf("PublishReport: %v", err)
	}
	other := domain.NewSanitizeReport("otro", domain.SanitizeModeApply)
	other.RunID = "run-otro"
	if err := a.arts.PublishReport(ctx, other); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(a.dataHome, "maintenance", "privacy")
	path := filepath.Join(dir, "run-1.json")
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("permisos del informe: %v %v", info, err)
		}
		if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("permisos del directorio: %v %v", info, err)
		}
	}
	pending, _, err := a.arts.PendingReports(ctx, "proj-abc")
	if err != nil || len(pending) != 1 || pending[0].RunID != "run-1" {
		t.Fatalf("PendingReports: %+v %v", pending, err)
	}

	r.Status = domain.SanitizeStatusCompleted
	if err := a.arts.PublishReport(ctx, r); err != nil {
		t.Fatal(err)
	}
	if pending, _, _ := a.arts.PendingReports(ctx, "proj-abc"); len(pending) != 0 {
		t.Fatalf("el informe completado sigue pendiente: %+v", pending)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), ".") {
			t.Fatalf("temporal residual: %s", e.Name())
		}
	}
	if err := a.arts.PublishReport(ctx, domain.SanitizeReport{RunID: "../fuera"}); err == nil {
		t.Fatal("run_id con ruta debe rechazarse")
	}
}

// TestPrivacyArtifacts_DepuraSnapshots cubre US2.5: los snapshots propios se
// reescriben protegidos; un enlace o un JSON desconocido quedan pendientes
// sin seguirse ni borrarse.
func TestPrivacyArtifacts_DepuraSnapshots(t *testing.T) {
	a := newArtifacts037(t)
	ctx := context.Background()
	dirty := a.backup(t, "20250101-000000.json", protectedBundle037(t))
	clean := a.backup(t, "20250102-000000.json", `{"version":2,"exported_at":"","source_project":"proj-abc","memories":[],"relations":[]}`+"\n")
	cleanBefore, _ := os.ReadFile(clean)
	unknown := a.backup(t, "20250103-000000.json", `{"otro":"formato"}`)
	broken := a.backup(t, "20250105-000000.json", `{no es json`)
	future := a.backup(t, "20250106-000000.json", `{"version":9,"campo_futuro":{"nota":"`+protectedIn+`","n":12345678901234567890},"memories":[],"relations":[{"reasoning":"`+protectedIn+`","nuevo":true}]}`)

	outside := filepath.Join(t.TempDir(), "externo.json")
	if err := os.WriteFile(outside, []byte(protectedBundle037(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(a.dataHome, "backups", a.key, "20250104-000000.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink no disponible: %v", err)
	}

	arts, err := a.arts.CleanBackups(ctx, "proj-abc")
	if err != nil {
		t.Fatalf("CleanBackups: %v", err)
	}
	status := map[string]domain.SanitizeArtifact{}
	for _, art := range arts {
		status[filepath.Base(art.Identity)] = art
		if filepath.IsAbs(art.Identity) {
			t.Fatalf("identidad absoluta: %s", art.Identity)
		}
	}
	if status["20250101-000000.json"].Status != domain.ArtifactSanitized ||
		status["20250102-000000.json"].Status != domain.ArtifactClean ||
		status["20250103-000000.json"].Status != domain.ArtifactClean ||
		status["20250104-000000.json"].ErrorCode != "unsafe_link" ||
		status["20250105-000000.json"].ErrorCode != "invalid_json" ||
		status["20250106-000000.json"].Status != domain.ArtifactSanitized {
		t.Fatalf("estados: %+v", status)
	}
	// S-02 (acr_9ba278e2): un bundle de una versión futura se sanea igual y
	// conserva sus campos desconocidos y la precisión de sus números.
	fut, _ := os.ReadFile(future)
	assertProtected(t, "snapshot de versión futura", string(fut))
	for _, keep := range []string{`"campo_futuro"`, `"nuevo": true`, `12345678901234567890`} {
		if !strings.Contains(string(fut), keep) {
			t.Fatalf("se perdió %s:\n%s", keep, fut)
		}
	}
	if b, _ := os.ReadFile(broken); string(b) != `{no es json` {
		t.Fatal("se modificó un archivo que no es JSON")
	}

	got, _ := os.ReadFile(dirty)
	assertProtected(t, "snapshot depurado", string(got))
	if !strings.Contains(string(got), "2024-01-01 00:00:00") {
		t.Fatal("el snapshot perdió fechas")
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(dirty); info.Mode().Perm() != 0o600 {
			t.Fatalf("permisos del snapshot: %v", info.Mode())
		}
	}
	if after, _ := os.ReadFile(clean); string(after) != string(cleanBefore) {
		t.Fatal("se reescribió un snapshot limpio")
	}
	if b, _ := os.ReadFile(unknown); string(b) != `{"otro":"formato"}` {
		t.Fatal("se modificó un JSON desconocido")
	}
	if b, _ := os.ReadFile(outside); !strings.Contains(string(b), privMarker) {
		t.Fatal("se siguió el enlace fuera de la raíz administrada")
	}

	residues, err := a.arts.VerifyResidues(ctx, "proj-abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(residues) != 2 {
		t.Fatalf("residuos = %+v, quiero solo el enlace y el JSON inválido", residues)
	}
	for _, r := range residues {
		if r.ErrorCode == "protected_content" {
			t.Fatalf("un problema de formato no debe confundirse con contenido protegido: %+v", residues)
		}
	}
}

// TestPrivacyArtifacts_ContextoGenerado: solo pide regenerar si existe y
// contiene contenido protegido, y lo reporta como residuo mientras tanto.
func TestPrivacyArtifacts_ContextoGenerado(t *testing.T) {
	a := newArtifacts037(t)
	ctx := context.Background()
	if need, err := a.arts.ContextNeedsRefresh(ctx); err != nil || need {
		t.Fatalf("sin archivo: need=%v err=%v", need, err)
	}
	mustMkdirAll(t, filepath.Dir(a.contextPath))
	if err := os.WriteFile(a.contextPath, []byte("## Sesiones\n- "+protectedIn+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if need, _ := a.arts.ContextNeedsRefresh(ctx); !need {
		t.Fatal("contexto protegido no detectado")
	}
	if res, _ := a.arts.VerifyResidues(ctx, "proj-abc"); len(res) != 1 || res[0].Identity != "context.md" {
		t.Fatalf("residuo de contexto: %+v", res)
	}
	if err := os.WriteFile(a.contextPath, []byte("## Sesiones\n- público\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if need, _ := a.arts.ContextNeedsRefresh(ctx); need {
		t.Fatal("contexto limpio marcado")
	}
	// Un remanente memory_review (cierre huérfano) no es residuo: si lo
	// fuera, el saneamiento no podría completar nunca (hallado por el runner).
	if err := os.WriteFile(a.contextPath, []byte("- texto Z</private> fin\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if need, _ := a.arts.ContextNeedsRefresh(ctx); need {
		t.Fatal("un cierre huérfano no debe exigir regenerar el contexto")
	}
}

// TestPrivacyArtifacts_InformeIlegibleNoSeOculta cubre C-002 (acr_9ba278e2):
// un informe corrupto o ilegible no se descarta en silencio; se devuelve como
// artefacto pendiente para que nadie declare completed con ese rastro.
func TestPrivacyArtifacts_InformeIlegibleNoSeOculta(t *testing.T) {
	a := newArtifacts037(t)
	ctx := context.Background()
	ok := domain.NewSanitizeReport("proj-abc", domain.SanitizeModeApply)
	ok.RunID = "run-ok"
	if err := a.arts.PublishReport(ctx, ok); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(a.dataHome, "maintenance", "privacy")
	if err := os.WriteFile(filepath.Join(dir, "run-roto.json"), []byte("{no es json"), 0o600); err != nil {
		t.Fatal(err)
	}
	reports, unreadable, err := a.arts.PendingReports(ctx, "proj-abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].RunID != "run-ok" {
		t.Fatalf("informes legibles: %+v", reports)
	}
	if len(unreadable) != 1 || unreadable[0].Identity != "maintenance/privacy/run-roto.json" ||
		unreadable[0].Status != domain.ArtifactPending || unreadable[0].ErrorCode != "unreadable_report" {
		t.Fatalf("informe ilegible: %+v", unreadable)
	}
}
