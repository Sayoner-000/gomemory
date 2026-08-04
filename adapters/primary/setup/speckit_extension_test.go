package setup

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

//go:embed testdata/speckit-templates
var speckitExtensionTestFS embed.FS

func speckitTestTemplatesFS(t *testing.T) fs.FS {
	t.Helper()
	fsys, err := fs.Sub(speckitExtensionTestFS, "testdata/speckit-templates")
	if err != nil {
		t.Fatalf("sub fs: %v", err)
	}
	return fsys
}

func TestInstallSpeckitExtension_NoSpecifyDir_NoOp(t *testing.T) {
	root := t.TempDir()

	if err := InstallSpeckitExtension(root, speckitTestTemplatesFS(t)); err != nil {
		t.Fatalf("InstallSpeckitExtension: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".specify")); !os.IsNotExist(err) {
		t.Errorf("sin .specify/ preexistente, InstallSpeckitExtension no debería crearlo (err=%v)", err)
	}
}

func TestInstallSpeckitExtension_NilTemplatesFS_NoOp(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specify"), 0755); err != nil {
		t.Fatalf("mkdir .specify: %v", err)
	}

	if err := InstallSpeckitExtension(root, nil); err != nil {
		t.Fatalf("InstallSpeckitExtension con templatesFS nil no debería fallar: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".specify", "extensions")); !os.IsNotExist(err) {
		t.Errorf("templatesFS nil no debería crear .specify/extensions/ (err=%v)", err)
	}
}

func TestInstallSpeckitExtension_WithSpecify_CopiesExtensionTree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specify"), 0755); err != nil {
		t.Fatalf("mkdir .specify: %v", err)
	}

	if err := InstallSpeckitExtension(root, speckitTestTemplatesFS(t)); err != nil {
		t.Fatalf("InstallSpeckitExtension: %v", err)
	}

	extDir := filepath.Join(root, ".specify", "extensions", "gomemory-context")
	if _, err := os.Stat(filepath.Join(extDir, "extension.yml")); err != nil {
		t.Errorf("extension.yml no se copió: %v", err)
	}
	scriptPath := filepath.Join(extDir, "scripts", "bash", "update-gomemory-context.sh")
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("el script bash no se copió: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("el script bash debería quedar con permiso de ejecución, tiene %v", info.Mode().Perm())
	}
}

func TestInstallSpeckitExtension_WithSpecify_CopiesClaudeArtifact(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specify"), 0755); err != nil {
		t.Fatalf("mkdir .specify: %v", err)
	}

	if err := InstallSpeckitExtension(root, speckitTestTemplatesFS(t)); err != nil {
		t.Fatalf("InstallSpeckitExtension: %v", err)
	}

	skillPath := filepath.Join(root, ".claude", "skills", "speckit-gomemory-context-update", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("el artefacto de Claude Code no se copió: %v", err)
	}
	if len(data) == 0 {
		t.Error("el artefacto de Claude Code se copió vacío")
	}
}

func TestInstallSpeckitExtension_WithSpecify_CopiesOpenCodeArtifact(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specify"), 0755); err != nil {
		t.Fatalf("mkdir .specify: %v", err)
	}

	if err := InstallSpeckitExtension(root, speckitTestTemplatesFS(t)); err != nil {
		t.Fatalf("InstallSpeckitExtension: %v", err)
	}

	cmdPath := filepath.Join(root, ".opencode", "commands", "speckit.gomemory-context.update.md")
	data, err := os.ReadFile(cmdPath)
	if err != nil {
		t.Fatalf("el artefacto de OpenCode no se copió: %v", err)
	}
	if len(data) == 0 {
		t.Error("el artefacto de OpenCode se copió vacío")
	}
}

func TestInstallSpeckitExtension_Idempotent_NoRewriteWhenUnchanged(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specify"), 0755); err != nil {
		t.Fatalf("mkdir .specify: %v", err)
	}
	templatesFS := speckitTestTemplatesFS(t)

	if err := InstallSpeckitExtension(root, templatesFS); err != nil {
		t.Fatalf("primera instalación: %v", err)
	}
	target := filepath.Join(root, ".specify", "extensions", "gomemory-context", "extension.yml")
	before, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat tras primera instalación: %v", err)
	}

	// mtime tiene resolución de hasta 1s en algunos filesystems: forzar un
	// desfase para que una reescritura accidental sea detectable.
	past := before.ModTime().Add(-time.Hour)
	if err := os.Chtimes(target, past, past); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if err := InstallSpeckitExtension(root, templatesFS); err != nil {
		t.Fatalf("segunda instalación: %v", err)
	}
	after, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat tras segunda instalación: %v", err)
	}
	if !after.ModTime().Equal(past) {
		t.Errorf("un archivo sin cambios no debería reescribirse: mtime esperado %v, obtuvo %v", past, after.ModTime())
	}
}

func TestInstallSpeckitExtension_OutdatedFile_GetsOverwritten(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specify"), 0755); err != nil {
		t.Fatalf("mkdir .specify: %v", err)
	}
	templatesFS := speckitTestTemplatesFS(t)

	if err := InstallSpeckitExtension(root, templatesFS); err != nil {
		t.Fatalf("primera instalación: %v", err)
	}

	// Simula una versión anterior (o una edición manual): el contenido en
	// disco ya no coincide con la plantilla embebida actual.
	target := filepath.Join(root, ".specify", "extensions", "gomemory-context", "extension.yml")
	original, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("leer extension.yml: %v", err)
	}
	if err := os.WriteFile(target, []byte("# versión desactualizada / editada a mano\n"), 0644); err != nil {
		t.Fatalf("escribir versión desactualizada: %v", err)
	}

	if err := InstallSpeckitExtension(root, templatesFS); err != nil {
		t.Fatalf("segunda instalación: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("leer extension.yml tras reinstalar: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("un archivo desactualizado/editado debería sobrescribirse con la plantilla embebida; quedó: %q", got)
	}
}

func TestInstallSpeckitExtension_SpecifyIsFile_NoOp(t *testing.T) {
	root := t.TempDir()
	// .specify existe pero como archivo, no directorio: no debe tratarse
	// como "spec-kit instalado". Nota: una vez .specify es un archivo,
	// CUALQUIER os.Stat("<root>/.specify/algo") falla con ENOTDIR (no
	// ENOENT), así que verificar con os.IsNotExist no probaría nada real
	// — en cambio, se confirma que InstallSpeckitExtension no devuelve
	// error y que el archivo .specify queda intacto (no se convirtió en
	// directorio).
	specifyPath := filepath.Join(root, ".specify")
	if err := os.WriteFile(specifyPath, []byte("x"), 0644); err != nil {
		t.Fatalf("write .specify file: %v", err)
	}

	if err := InstallSpeckitExtension(root, speckitTestTemplatesFS(t)); err != nil {
		t.Fatalf("InstallSpeckitExtension: %v", err)
	}

	info, err := os.Stat(specifyPath)
	if err != nil {
		t.Fatalf(".specify debería seguir existiendo: %v", err)
	}
	if info.IsDir() {
		t.Error(".specify era un archivo y no debería haberse convertido en directorio")
	}
}
