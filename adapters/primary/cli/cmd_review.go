package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"mem/application/usecases"
	"mem/domain"
)

func CmdReview(deps *Deps, args []string) {
	if len(args) == 0 {
		fail("uso: mem review --diff [rango] | --commit <sha> | --file <ruta> | --pending\n" +
			"     opciones: --read-only\n" +
			"     mem review status [<review-id>] | history [--limit N] | show <review-id>")
	}
	// --read-only puede venir en cualquier posición: es un modificador del alcance
	// de la revisión, no un modo de target.
	soloLectura := false
	filtrados := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--read-only" {
			soloLectura = true
			continue
		}
		filtrados = append(filtrados, arg)
	}
	args = filtrados
	if len(args) == 0 {
		fail("--read-only modifica una revisión: indica también el target")
	}

	var targetType domain.TargetType
	var revision, digest string
	var scope []string
	var err error
	switch args[0] {
	case "--diff":
		var diffRange string
		if len(args) > 1 {
			diffRange = args[1]
		}
		targetType = domain.TargetDiff
		revision, digest, err = resolveDiffTarget(deps.Root, diffRange)
	case "--pending":
		targetType = domain.TargetFileSet
		revision, digest, scope, err = resolvePendingTarget(deps.Root)
	case "--commit":
		if len(args) != 2 {
			fail("--commit requiere un SHA o referencia")
		}
		targetType = domain.TargetCommit
		revision, digest, err = resolveCommitTarget(deps.Root, args[1])
	case "--file":
		if len(args) != 2 {
			fail("--file requiere una ruta")
		}
		targetType = domain.TargetFileSet
		revision, digest, scope, err = resolveFileTarget(deps.Root, args[1])
	case "status":
		cmdReviewStatus(deps, args[1:])
		return
	case "history":
		cmdReviewHistory(deps, args[1:])
		return
	case "show":
		cmdReviewShow(deps, args[1:])
		return
	default:
		fail("subcomando de review desconocido: %s\n"+
			"uso: mem review --diff [rango] | --commit <sha> | --file <ruta> | --pending\n"+
			"     opciones: --read-only\n"+
			"     mem review status [<review-id>] | history [--limit N] | show <review-id>", args[0])
	}
	if err != nil {
		fail("resolver target: %v", err)
	}
	entrada := usecases.StartReviewInput{
		Project: deps.Project, TargetType: targetType, Revision: revision, Digest: digest, Scope: scope,
		Policy: reviewPolicyDelProyecto(deps),
	}
	if soloLectura {
		no := false
		entrada.FixAuthorized = &no
	}
	review, err := usecases.StartReview(deps.ReviewRepo, entrada)
	if err != nil {
		fail("iniciar revisión: %v", err)
	}
	fmt.Println(review.ID)
	fmt.Printf("target_digest: %s\n", review.Target.Digest())
	if len(scope) > 0 {
		fmt.Printf("target_files: %d\n", len(scope))
	}
	fmt.Printf("independence: %s", review.IndependenceLevel)
	if review.IndependenceReason != "" {
		fmt.Printf(" (%s)", review.IndependenceReason)
	}
	fmt.Println()
	// La política efectiva se imprime siempre: hasta la funcionalidad 028 salía de
	// constantes escritas en el código y configurar el proyecto no cambiaba nada,
	// así que no había forma de saber con qué reglas quedó congelada la revisión.
	fmt.Printf("fix_authorized: %t\n", review.FixAuthorized)
	fmt.Printf("max_fix_rounds: %d\n", review.MaxFixRounds)
	severidades := make([]string, 0, len(review.AutoFixSeverities))
	for _, severidad := range review.AutoFixSeverities {
		severidades = append(severidades, string(severidad))
	}
	fmt.Printf("auto_fix_severities: %s\n", strings.Join(severidades, ", "))
}

func resolveCommitTarget(root, ref string) (string, string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", ref+"^{commit}")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("commit %q no existe", ref)
	}
	sha := strings.TrimSpace(string(out))
	return sha, sha, nil
}

func resolveDiffTarget(root, diffRange string) (string, string, error) {
	args := []string{"diff", "--binary"}
	revision := "working-tree"
	if diffRange != "" {
		args = append(args, diffRange)
		revision = diffRange
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("git diff %q: %w", diffRange, err)
	}
	sum := sha256.Sum256(append([]byte("diff\x00"+revision+"\x00"), out...))
	return revision, hex.EncodeToString(sum[:]), nil
}

func resolveFileTarget(root, requested string) (string, string, []string, error) {
	abs := requested
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, requested)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", nil, err
	}
	var paths []string
	if info.IsDir() {
		err = filepath.WalkDir(abs, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return "", "", nil, err
		}
	} else {
		paths = []string{abs}
	}
	sort.Strings(paths)
	hash := sha256.New()
	scope := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return "", "", nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", "", nil, err
		}
		scope = append(scope, filepath.ToSlash(rel))
		hash.Write([]byte("file\x00" + filepath.ToSlash(rel) + "\x00"))
		hash.Write(data)
		hash.Write([]byte{0})
	}
	revision := filepath.ToSlash(requested)
	return revision, hex.EncodeToString(hash.Sum(nil)), scope, nil
}

// resolvePendingTarget congela TODO el trabajo pendiente del proyecto: cambios
// preparados, sin preparar y archivos nuevos no ignorados (FR-025).
//
// Existe porque --diff usa `git diff --binary`, que NO ve los archivos sin
// seguimiento: una revisión de trabajo en curso con archivos recién creados
// congelaba un target que no los contenía, y los revisores inspeccionaban menos de lo
// que se creía que inspeccionaban.
//
// El -z no es cosmético: los nombres con espacios rompen cualquier parseo por líneas,
// y es uno de los casos límite que la especificación enumera. --untracked-files=all
// respeta .gitignore, así que lo ignorado sigue fuera del target.
func resolvePendingTarget(root string) (string, string, []string, error) {
	cmd := exec.Command("git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", "", nil, fmt.Errorf("git status: %w", err)
	}

	rutas, indice, err := parsePorcelainZ(string(out))
	if err != nil {
		return "", "", nil, err
	}
	if len(rutas) == 0 {
		return "", "", nil, fmt.Errorf("no hay cambios pendientes que revisar")
	}
	sort.Strings(rutas)

	preparados, err := blobsPreparados(root, rutas)
	if err != nil {
		return "", "", nil, err
	}

	hash := sha256.New()
	scope := make([]string, 0, len(rutas))
	for _, rel := range rutas {
		scope = append(scope, rel)
		hash.Write([]byte("pending\x00" + rel + "\x00"))
		// Lo PREPARADO entra en la identidad del target, no solo el árbol de trabajo.
		//
		// Solo se hasheaba os.ReadFile, y eso deja fuera justo el caso que el
		// comentario de arriba promete cubrir: preparar una versión y seguir editando
		// —o revertir— el archivo deja el índice y el árbol con contenidos distintos.
		// Dos revisiones con contenidos preparados completamente distintos producían
		// el MISMO digest, así que el target no identificaba lo que decía congelar.
		if sha, ok := preparados[rel]; ok {
			hash.Write([]byte("staged\x00" + sha + "\x00"))
		} else {
			// Sin entrada en el índice hay DOS situaciones distintas, y colapsarlas en
			// un solo marcador hacía que un archivo sin seguimiento y otro preparado
			// para borrar —intenciones opuestas sobre la misma ruta— compartieran
			// identidad congelada si el árbol tenía el mismo contenido. El estado del
			// índice lo trae `git status` en el primer carácter de cada registro: '?'
			// es sin seguimiento y 'D' es preparado para borrar.
			hash.Write([]byte{'u', 'n', 's', 't', 'a', 'g', 'e', 'd', 0, indice[rel], 0})
		}
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			if !os.IsNotExist(err) {
				return "", "", nil, err
			}
			// Un archivo borrado contribuye su ruta con un marcador: borrar debe
			// cambiar la identidad del target, no pasar desapercibido.
			hash.Write([]byte("deleted"))
		} else {
			hash.Write(data)
		}
		hash.Write([]byte{0})
	}
	return "pending-changes", hex.EncodeToString(hash.Sum(nil)), scope, nil
}

// blobsPreparados devuelve, por ruta, el identificador del blob que hay en el índice.
//
// Se hashea el identificador y no el contenido porque git ya lo calculó sobre ese
// contenido: dos índices distintos dan blobs distintos, que es la única propiedad que
// el digest necesita. Y es una sola invocación en vez de una por archivo.
//
// Una ruta sin entrada en el índice —sin seguimiento, o preparada para borrar— no
// aparece en el mapa, y quien llama la distingue con un marcador.
func blobsPreparados(root string, rutas []string) (map[string]string, error) {
	// SIN pathspec y con --full-name, por dos motivos distintos que se cierran igual.
	//
	// El pathspec pasaba cada ruta pendiente como argumento: con decenas de miles de
	// archivos sin seguimiento —el caso que --untracked-files=all produce en cuanto hay
	// un directorio generado y no ignorado— la invocación superaba el límite de
	// argumentos del sistema y la congelación fallaba por completo, cuando antes de
	// mezclar el índice funcionaba.
	//
	// Y las dos órdenes no hablan la misma convención de rutas: `git status` las
	// devuelve relativas a la raíz del repositorio y `git ls-files` relativas al
	// directorio de trabajo. Con una raíz de proyecto anidada dentro de un repositorio
	// mayor el pathspec no casaba con nada, y `ls-files` sale con código 0 y salida
	// vacía cuando eso ocurre: el mapa quedaba vacío SIN error, cada ruta se marcaba
	// como no preparada y la mezcla del índice se desactivaba en silencio. --full-name
	// fuerza rutas relativas a la raíz, que es la convención de `git status`.
	cmd := exec.Command("git", "ls-files", "--stage", "-z", "--full-name")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	entradas := make(map[string][]string, len(rutas))
	for _, registro := range strings.Split(string(out), "\x00") {
		if registro == "" {
			continue
		}
		// Formato: "<modo> <blob> <etapa>\t<ruta>". La ruta puede llevar espacios,
		// así que se corta por el tabulador y no por espacios.
		meta, ruta, ok := strings.Cut(registro, "\t")
		if !ok {
			continue
		}
		campos := strings.Fields(meta)
		if len(campos) < 3 {
			continue
		}
		// La identidad del índice no es solo el blob: el modo distingue, por
		// ejemplo, un archivo ejecutable de uno normal, y la etapa distingue las
		// entradas 1/2/3 de un conflicto. Una ruta puede tener varias etapas, así
		// que no se deben sobrescribir entre sí.
		entradas[ruta] = append(entradas[ruta], strings.Join(campos[:3], " "))
	}
	preparados := make(map[string]string, len(entradas))
	for ruta, valores := range entradas {
		sort.Strings(valores)
		preparados[ruta] = strings.Join(valores, "\x00")
	}
	return preparados, nil
}

// parsePorcelainZ extrae las rutas de la salida de `git status -z`.
//
// El formato son registros terminados en NUL, cada uno "XY <ruta>". Un renombrado
// (R) gasta DOS registros: el destino y, a continuación, el origen. Se toman ambos:
// mover un archivo cambia lo que hay que revisar en los dos extremos.
func parsePorcelainZ(salida string) ([]string, map[string]byte, error) {
	campos := strings.Split(salida, "\x00")
	vistas := map[string]bool{}
	indice := map[string]byte{}
	var rutas []string
	for i := 0; i < len(campos); i++ {
		registro := campos[i]
		if len(registro) < 4 {
			continue
		}
		estado := registro[:2]
		ruta := registro[3:]
		if !vistas[ruta] {
			vistas[ruta] = true
			rutas = append(rutas, ruta)
			indice[ruta] = estado[0]
		}
		if estado[0] == 'R' || estado[0] == 'C' {
			i++
			if i < len(campos) && campos[i] != "" && !vistas[campos[i]] {
				vistas[campos[i]] = true
				rutas = append(rutas, campos[i])
				indice[campos[i]] = estado[0]
			}
		}
	}
	return rutas, indice, nil
}
