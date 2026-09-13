package usecases_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

const anchorsHeader031 = "## 🧭 Anclas sin evidencia"

type anchorFilesFake struct{ m map[string]string }

func (f anchorFilesFake) FileHashes(string) (map[string]string, error) { return f.m, nil }

var _ ports.IndexedFilesQuerier = anchorFilesFake{}

// newCtx031Fixture monta el contexto sobre persistencia real, igual que el
// resto de tests de build_context, para que la sección vea lo mismo que en uso.
func newCtx031Fixture(t *testing.T) (string, ports.MemoryRepository, ports.SessionRepository, ports.RelationRepository, *usecases.Builder) {
	t.Helper()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	memRepo := persistence.NewMemoryRepository(db)
	sessRepo := persistence.NewSessionRepository(db)
	relRepo := persistence.NewRelationRepository(db)
	return root, memRepo, sessRepo, relRepo, usecases.New(memRepo, sessRepo, relRepo, root, "proj")
}

func mustInsert031(t *testing.T, repo ports.MemoryRepository, m domain.Memory) int64 {
	t.Helper()
	m.Project = "proj"
	id, err := repo.Insert(&m)
	if err != nil {
		t.Fatalf("insert %q: %v", m.Title, err)
	}
	return id
}

func sectionOf031(out, header string) string {
	i := strings.Index(out, header)
	if i < 0 {
		return ""
	}
	rest := out[i+len(header):]
	if j := strings.Index(rest, "\n## "); j >= 0 {
		rest = rest[:j]
	}
	return header + rest
}

func entryLines031(section string) int {
	n := 0
	for _, line := range strings.Split(section, "\n") {
		if strings.HasPrefix(line, "- [") {
			n++
		}
	}
	return n
}

func TestBuild_AnclasSinEvidencia_SoloHuerfanas(t *testing.T) {
	root, memRepo, _, _, b := newCtx031Fixture(t)
	if err := os.WriteFile(filepath.Join(root, "vivo.go"), []byte("package x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	orphan := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "ancla a archivo borrado", Content: "c1", Filepath: "borrado.go"})
	live := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "ancla a archivo vivo", Content: "c2", Filepath: "vivo.go"})
	outside := mustInsert031(t, memRepo, domain.Memory{Type: domain.Bugfix, Title: "ancla fuera del repo", Content: "c3", Filepath: "/fuera/del/repo.ts"})
	cp := mustInsert031(t, memRepo, domain.Memory{Type: domain.Checkpoint, Title: "cp", Content: "cp único", Filepath: "falta-cp.go"})

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	sec := sectionOf031(out, anchorsHeader031)
	if sec == "" {
		t.Fatalf("falta la sección de anclas:\n%s", out)
	}
	if !strings.Contains(sec, fmt.Sprintf("[%d] «", orphan)) || !strings.Contains(sec, "huérfana candidata") {
		t.Fatalf("la huérfana no aparece:\n%s", sec)
	}
	for _, id := range []int64{live, outside, cp} {
		if strings.Contains(sec, fmt.Sprintf("[%d] «", id)) {
			t.Fatalf("la memoria %d no debe listarse:\n%s", id, sec)
		}
	}
	if !strings.Contains(sec, "Sin índice de código: solo se comprobó el disco") || strings.Contains(sec, "movida") {
		t.Fatalf("sin índice debe declararlo y nunca afirmar movida:\n%s", sec)
	}
}

func TestBuild_AnclasSinEvidencia_MovidaYAmbigua(t *testing.T) {
	_, memRepo, _, _, b := newCtx031Fixture(t)
	moved := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "ancla movida", Content: "c1", Filepath: "viejo/borrado.go"})
	amb := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "ancla ambigua", Content: "c2", Filepath: "viejo/comun.go"})
	b.Files = anchorFilesFake{m: map[string]string{"nuevo/borrado.go": "h1", "a/comun.go": "h2", "b/comun.go": "h3"}}

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	sec := sectionOf031(out, anchorsHeader031)
	if !strings.Contains(sec, fmt.Sprintf("[%d] «", moved)) || !strings.Contains(sec, "movida a `nuevo/borrado.go`") {
		t.Fatalf("la movida no aparece con su ruta candidata:\n%s", sec)
	}
	if !strings.Contains(sec, fmt.Sprintf("[%d] «", amb)) || !strings.Contains(sec, "movida (ambigua: 2 rutas con el mismo nombre)") {
		t.Fatalf("la ambigua no aparece sin ruta concreta:\n%s", sec)
	}
	if strings.Contains(sec, "Sin índice de código") {
		t.Fatalf("con índice no debe declarar su ausencia:\n%s", sec)
	}
}

func TestBuild_AnclasSinEvidencia_MaximoOcho(t *testing.T) {
	_, memRepo, _, _, b := newCtx031Fixture(t)
	for i := 0; i < 9; i++ {
		mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: fmt.Sprintf("huérfana %d", i), Content: fmt.Sprintf("contenido %d", i), Filepath: fmt.Sprintf("falta%d.go", i)})
	}
	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if n := entryLines031(sectionOf031(out, anchorsHeader031)); n != 8 {
		t.Fatalf("entradas = %d, want 8", n)
	}
}

// C-002 (ACR acr_0106584b): el ancla de una memoria antigua, fuera de las 100
// recientes que listan las secciones por tipo, también se clasifica.
func TestBuild_AnclasSinEvidencia_IncluyeMemoriasFueraDeLas100(t *testing.T) {
	_, memRepo, _, _, b := newCtx031Fixture(t)
	old := domain.Memory{Project: "proj", Type: domain.Decision, Title: "ancla antigua", Content: "vieja", Filepath: "antiguo/borrado.go", CreatedAt: "2020-01-01 00:00:00"}
	oldID, err := memRepo.ImportMemory(&old)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	for i := 0; i < 105; i++ {
		mustInsert031(t, memRepo, domain.Memory{Type: domain.Learning, Title: fmt.Sprintf("reciente %d", i), Content: fmt.Sprintf("r%d", i)})
	}
	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if sec := sectionOf031(out, anchorsHeader031); !strings.Contains(sec, fmt.Sprintf("[%d] «", oldID)) {
		t.Fatalf("el ancla antigua sin evidencia no se clasificó:\n%s", sec)
	}
}

// C-001 (ACR acr_0106584b): un error de permisos no prueba que el archivo
// falte; el ancla no es verificable y no debe listarse.
func TestBuild_AnclasSinEvidencia_ErrorDePermisoNoEsHuerfana(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignora los permisos de directorio")
	}
	root, memRepo, _, _, b := newCtx031Fixture(t)
	locked := filepath.Join(root, "bloqueado")
	if err := os.Mkdir(locked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(locked, "secreto.go"), []byte("package x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	id := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "ancla sin permiso", Content: "p", Filepath: "bloqueado/secreto.go"})

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if strings.Contains(out, fmt.Sprintf("[%d] «", id)) {
		t.Fatalf("un error de permisos se presentó como ancla sin evidencia:\n%s", out)
	}
}

// S-006 (ACR 031): como el resto de secciones, la de anclas dice cuántas
// quedaron fuera del tope, para que un recorte no parezca la lista completa.
func TestBuild_AnclasSinEvidencia_IndicaCuantasQuedanFuera(t *testing.T) {
	_, memRepo, _, _, b := newCtx031Fixture(t)
	for i := 0; i < 11; i++ {
		mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: fmt.Sprintf("huérfana %d", i), Content: fmt.Sprintf("contenido %d", i), Filepath: fmt.Sprintf("falta%d.go", i)})
	}
	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if sec := sectionOf031(out, anchorsHeader031); !strings.Contains(sec, "(+3 anclas más; usa search_memories/get_memory)") {
		t.Fatalf("falta la línea de las anclas que quedaron fuera:\n%s", sec)
	}
}

func TestBuild_AnclasSinEvidencia_RespetaPresupuesto(t *testing.T) {
	_, memRepo, _, _, b := newCtx031Fixture(t)
	for i := 0; i < 9; i++ {
		mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: fmt.Sprintf("huérfana con un título bastante largo %d", i), Content: fmt.Sprintf("contenido %d", i), Filepath: fmt.Sprintf("ruta/muy/larga/falta%d.go", i)})
	}
	b.Budget = 700
	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(out) > b.Budget {
		t.Fatalf("salida de %d bytes supera el presupuesto %d:\n%s", len(out), b.Budget, out)
	}
}
