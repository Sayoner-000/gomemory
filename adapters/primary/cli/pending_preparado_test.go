package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// repoConConflicto deja el índice con las TRES etapas de un conflicto de merge para
// la misma ruta: 1 (base común), 2 (nuestra versión) y 3 (la de ellos). Es el estado
// que git deja tras un merge que no resuelve, y la razón por la que blobsPreparados
// acumula varias entradas por ruta en vez de sobrescribirlas.
func repoConConflicto(t *testing.T, base, nuestra, suya string) string {
	t.Helper()
	root := t.TempDir()
	gitEnRepo(t, root, "init", "-q", ".")
	gitEnRepo(t, root, "config", "user.email", "prueba@local")
	gitEnRepo(t, root, "config", "user.name", "prueba")
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("V0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitEnRepo(t, root, "add", "f.txt")
	gitEnRepo(t, root, "commit", "-qm", "base")

	blob := func(contenido string) string {
		t.Helper()
		cmd := exec.Command("git", "hash-object", "-w", "--stdin")
		cmd.Dir = root
		cmd.Stdin = strings.NewReader(contenido)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git hash-object: %v", err)
		}
		return strings.TrimSpace(string(out))
	}

	gitEnRepo(t, root, "rm", "-q", "--cached", "f.txt")
	entradas := fmt.Sprintf("100644 %s 1\tf.txt\n100644 %s 2\tf.txt\n100644 %s 3\tf.txt\n",
		blob(base), blob(nuestra), blob(suya))
	cmd := exec.Command("git", "update-index", "--index-info")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(entradas)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git update-index --index-info: %v\n%s", err, out)
	}
	return root
}

// TestBlobsPreparadosDistingueLasEtapasDelConflicto cubre la otra mitad del arreglo
// de blobsPreparados: la que motivó pasar de map[ruta]sha a acumular, ordenar y unir.
//
// Conservar solo el SHA del blob hacía que las etapas 1/2/3 de una misma ruta se
// sobrescribieran entre sí, así que dos conflictos con resoluciones pendientes
// distintas compartían identidad congelada. El modo tenía prueba desde v2.16.5; la
// etapa no, y era el caso que el propio comentario declara como motivo principal.
func TestBlobsPreparadosDistingueLasEtapasDelConflicto(t *testing.T) {
	// Los MISMOS blobs, cruzados de etapa: en uno queremos X y ellos quieren Y, en
	// el otro al revés. Son intenciones opuestas sobre la misma ruta.
	//
	// El caso está elegido a propósito: si las tres etapas llevan blobs distintos,
	// la simple acumulación ya basta para separarlas y la etapa no aporta nada. Solo
	// cuando el conjunto de blobs coincide se ve que el número de etapa es parte de
	// la identidad del índice, y no un adorno del formato.
	nuestraPrimero := repoConConflicto(t, "BASE\n", "X\n", "Y\n")
	suyaPrimero := repoConConflicto(t, "BASE\n", "Y\n", "X\n")

	uno, err := blobsPreparados(nuestraPrimero, []string{"f.txt"})
	if err != nil {
		t.Fatal(err)
	}
	otro, err := blobsPreparados(suyaPrimero, []string{"f.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if uno["f.txt"] == otro["f.txt"] {
		t.Fatal("dos conflictos con las etapas cruzadas comparten identidad preparada")
	}

	// Y las tres etapas deben estar representadas: quedarse con una sola volvería a
	// colapsar estados distintos del índice.
	if partes := strings.Split(uno["f.txt"], "\x00"); len(partes) != 3 {
		t.Fatalf("se conservaron %d etapas de 3: %q", len(partes), uno["f.txt"])
	}
}

// repoConIndiceCrudo construye un repositorio cuyo índice se escribe entrada por
// entrada, y deja en el árbol de trabajo el contenido indicado. Permite fabricar
// estados del índice que git no produciría por sí solo, que es lo que hace falta
// para exhibir una colisión de identidad.
func repoConIndiceCrudo(t *testing.T, entradas string, contenido []byte) string {
	t.Helper()
	root := t.TempDir()
	gitEnRepo(t, root, "init", "-q", ".")
	gitEnRepo(t, root, "config", "user.email", "prueba@local")
	gitEnRepo(t, root, "config", "user.name", "prueba")
	if err := os.WriteFile(filepath.Join(root, "semilla.txt"), []byte("s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitEnRepo(t, root, "add", "semilla.txt")
	gitEnRepo(t, root, "commit", "-qm", "base")

	cmd := exec.Command("git", "update-index", "--index-info")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(entradas)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git update-index --index-info: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(root, "f.txt"), contenido, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func blobDe(t *testing.T, root, contenido string) string {
	t.Helper()
	cmd := exec.Command("git", "hash-object", "-w", "--stdin")
	cmd.Dir = root
	cmd.Stdin = strings.NewReader(contenido)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git hash-object: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestPendingNoColisionaAlRepartirLosCamposDeOtraForma exhibe la ambigüedad que la
// concatenación sin longitud permitía.
//
// El digest escribía «staged\0» + identidad + «\0» + contenido crudo + «\0». Como ni
// la identidad ni el contenido tienen prohibido contener NUL, el mismo flujo de bytes
// admitía dos lecturas: un índice con DOS entradas y un contenido C producía
// exactamente los mismos bytes que un índice con UNA entrada y un contenido que
// empezara por la segunda entrada seguida de NUL. Dos estados pendientes distintos,
// una sola identidad congelada.
func TestPendingNoColisionaAlRepartirLosCamposDeOtraForma(t *testing.T) {
	semilla := t.TempDir()
	gitEnRepo(t, semilla, "init", "-q", ".")
	gitEnRepo(t, semilla, "config", "user.email", "prueba@local")
	gitEnRepo(t, semilla, "config", "user.name", "prueba")
	sha := blobDe(t, semilla, "X\n")
	etapa1 := fmt.Sprintf("100644 %s 1", sha)
	etapa2 := fmt.Sprintf("100644 %s 2", sha)

	// Dos entradas en el índice, contenido limpio.
	dosEntradas := repoConIndiceCrudo(t,
		fmt.Sprintf("%s\tf.txt\n%s\tf.txt\n", etapa1, etapa2),
		[]byte("HOLA\n"))

	// Una entrada, y el contenido absorbe la segunda: los mismos bytes, repartidos
	// de otra forma.
	unaEntrada := repoConIndiceCrudo(t,
		fmt.Sprintf("%s\tf.txt\n", etapa1),
		append([]byte(etapa2+"\x00"), []byte("HOLA\n")...))

	_, digestDos, _, err := resolvePendingTarget(dosEntradas)
	if err != nil {
		t.Fatal(err)
	}
	_, digestUna, _, err := resolvePendingTarget(unaEntrada)
	if err != nil {
		t.Fatal(err)
	}
	if digestDos == digestUna {
		t.Fatal("dos estados pendientes distintos comparten digest: los campos se pueden repartir de más de una forma")
	}
}
