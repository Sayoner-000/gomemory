package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitEnRepo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// repoConPreparado deja un repositorio donde el índice y el árbol de trabajo tienen
// contenidos DISTINTOS: se prepara `preparado` y después el árbol vuelve a su versión
// original. Es lo que ocurre al preparar una versión y seguir editando o revertir.
func repoConPreparado(t *testing.T, preparado string) string {
	t.Helper()
	root := t.TempDir()
	gitEnRepo(t, root, "init", "-q", ".")
	gitEnRepo(t, root, "config", "user.email", "prueba@local")
	gitEnRepo(t, root, "config", "user.name", "prueba")
	ruta := filepath.Join(root, "f.txt")
	if err := os.WriteFile(ruta, []byte("V0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitEnRepo(t, root, "add", "f.txt")
	gitEnRepo(t, root, "commit", "-qm", "base")
	if err := os.WriteFile(ruta, []byte(preparado), 0o644); err != nil {
		t.Fatal(err)
	}
	gitEnRepo(t, root, "add", "f.txt")
	if err := os.WriteFile(ruta, []byte("V0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestPendingDistingueElContenidoPreparado cubre el hallazgo de --pending de
// acr_83428b4c, reproducido antes de corregirlo.
//
// resolvePendingTarget documenta que congela "cambios preparados, sin preparar y
// archivos nuevos", pero solo hasheaba os.ReadFile: el contenido del árbol. Con el
// índice y el árbol divergentes, dos repositorios con contenidos preparados
// completamente distintos producían el MISMO digest, así que el target no identificaba
// lo que decía estar congelando y los revisores inspeccionaban otra cosa.
func TestPendingDistingueElContenidoPreparado(t *testing.T) {
	_, digestA, _, err := resolvePendingTarget(repoConPreparado(t, "PREPARADO-A\n"))
	if err != nil {
		t.Fatal(err)
	}
	_, digestB, _, err := resolvePendingTarget(repoConPreparado(t, "PREPARADO-B-MUY-DISTINTO\n"))
	if err != nil {
		t.Fatal(err)
	}
	if digestA == digestB {
		t.Fatal("mismo digest con contenidos preparados distintos: --pending no congela lo preparado")
	}
}

// TestBlobsPreparadosDistingueElModo evita que dos índices con el mismo blob pero
// permisos distintos vuelvan a compartir identidad. Git conserva el modo en la
// entrada del índice aunque los bytes sean idénticos.
func TestBlobsPreparadosDistingueElModo(t *testing.T) {
	root := repoConPreparado(t, "MISMOS-BYTES\n")
	gitEnRepo(t, root, "update-index", "--chmod=+x", "f.txt")
	ejecutable, err := blobsPreparados(root, []string{"f.txt"})
	if err != nil {
		t.Fatal(err)
	}

	gitEnRepo(t, root, "update-index", "--chmod=-x", "f.txt")
	normal, err := blobsPreparados(root, []string{"f.txt"})
	if err != nil {
		t.Fatal(err)
	}

	if ejecutable["f.txt"] == normal["f.txt"] {
		t.Fatal("misma identidad preparada con modos 100755 y 100644")
	}
}

// TestPendingSigueDistinguiendoElArbol es el control: mezclar el blob preparado no
// puede haber dejado de mirar el árbol de trabajo, que era lo único que miraba antes.
func TestPendingSigueDistinguiendoElArbol(t *testing.T) {
	repo := func(t *testing.T, arbol string) string {
		t.Helper()
		root := t.TempDir()
		gitEnRepo(t, root, "init", "-q", ".")
		gitEnRepo(t, root, "config", "user.email", "prueba@local")
		gitEnRepo(t, root, "config", "user.name", "prueba")
		ruta := filepath.Join(root, "f.txt")
		os.WriteFile(ruta, []byte("V0\n"), 0o644)
		gitEnRepo(t, root, "add", "f.txt")
		gitEnRepo(t, root, "commit", "-qm", "base")
		os.WriteFile(ruta, []byte(arbol), 0o644)
		return root
	}
	_, digestA, _, err := resolvePendingTarget(repo(t, "ARBOL-A\n"))
	if err != nil {
		t.Fatal(err)
	}
	_, digestB, _, err := resolvePendingTarget(repo(t, "ARBOL-B-DISTINTO\n"))
	if err != nil {
		t.Fatal(err)
	}
	if digestA == digestB {
		t.Fatal("mismo digest con árboles de trabajo distintos")
	}
}

// TestPendingDistingueSinSeguimientoDeBorradoPreparado: un archivo sin seguimiento y
// uno preparado para borrar son intenciones opuestas sobre la misma ruta. Con el mismo
// contenido visible en el árbol, colapsarlas en un solo marcador les daba la misma
// identidad congelada.
func TestPendingDistingueSinSeguimientoDeBorradoPreparado(t *testing.T) {
	sinSeguimiento := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		gitEnRepo(t, root, "init", "-q", ".")
		gitEnRepo(t, root, "config", "user.email", "prueba@local")
		gitEnRepo(t, root, "config", "user.name", "prueba")
		os.WriteFile(filepath.Join(root, "otro.txt"), []byte("ancla\n"), 0o644)
		gitEnRepo(t, root, "add", "otro.txt")
		gitEnRepo(t, root, "commit", "-qm", "base")
		// f.txt nunca entró en el repositorio.
		os.WriteFile(filepath.Join(root, "f.txt"), []byte("CONTENIDO\n"), 0o644)
		return root
	}
	borradoPreparado := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		gitEnRepo(t, root, "init", "-q", ".")
		gitEnRepo(t, root, "config", "user.email", "prueba@local")
		gitEnRepo(t, root, "config", "user.name", "prueba")
		os.WriteFile(filepath.Join(root, "otro.txt"), []byte("ancla\n"), 0o644)
		os.WriteFile(filepath.Join(root, "f.txt"), []byte("CONTENIDO\n"), 0o644)
		gitEnRepo(t, root, "add", ".")
		gitEnRepo(t, root, "commit", "-qm", "base")
		// f.txt se prepara para borrar, pero vuelve a existir en el árbol con el
		// mismo contenido: lo único que los diferencia es el índice.
		gitEnRepo(t, root, "rm", "--cached", "-q", "f.txt")
		os.WriteFile(filepath.Join(root, "f.txt"), []byte("CONTENIDO\n"), 0o644)
		return root
	}

	_, digestA, _, err := resolvePendingTarget(sinSeguimiento(t))
	if err != nil {
		t.Fatal(err)
	}
	_, digestB, _, err := resolvePendingTarget(borradoPreparado(t))
	if err != nil {
		t.Fatal(err)
	}
	if digestA == digestB {
		t.Fatal("mismo digest para un archivo sin seguimiento y uno preparado para borrar")
	}
}
