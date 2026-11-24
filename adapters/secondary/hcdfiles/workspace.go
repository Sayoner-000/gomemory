// Package hcdfiles es el lado de archivos, git y ejecución de HCD (feature 045).
package hcdfiles

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"mem/domain"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	defaultTimeout = 10 * time.Minute
	excerptBytes   = 2000
	excerptLines   = 20
	// waitDelay es el margen máximo tras el plazo: Run vuelve como mucho en
	// Timeout + waitDelay.
	waitDelay = time.Second
)

// Workspace implementa ports.HCDWorkspace sobre la raíz del proyecto.
type Workspace struct {
	Root    string
	Timeout time.Duration
}

func (w Workspace) ReadDoc(rel string) ([]byte, error) {
	full, err := w.resolve(rel, "hcd/")
	if err != nil {
		return nil, err
	}
	return os.ReadFile(full)
}

// WriteDoc escribe de forma atómica y nunca fuera de hcd/ ni a través de un
// symlink.
func (w Workspace) WriteDoc(rel string, data []byte) error {
	full, err := w.resolve(rel, "hcd/")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(full), ".hcd-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), full)
}

// MoveDoc renombra un documento dentro de hcd/ sin pisar otro (HCD-058).
func (w Workspace) MoveDoc(from, to string) error {
	src, err := w.resolve(from, "hcd/")
	if err != nil {
		return err
	}
	dst, err := w.resolve(to, "hcd/")
	if err != nil {
		return err
	}
	if src == dst {
		if _, err := os.Lstat(src); err != nil {
			return err
		}
		return fmt.Errorf("%w: ya existe %s", domain.ErrNotAllowed, to)
	}
	if err := renameNoReplace(src, dst); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%w: ya existe %s", domain.ErrNotAllowed, to)
		}
		return err
	}
	return nil
}

// resolve admite solo rutas relativas limpias bajo prefix y rechaza cualquier
// componente que sea un symlink.
func (w Workspace) resolve(rel, prefix string) (string, error) {
	clean := path.Clean(filepath.ToSlash(rel))
	if clean != filepath.ToSlash(rel) || !strings.HasPrefix(clean, prefix) || strings.Contains(clean, "..") {
		return "", fmt.Errorf("ruta HCD inválida: %q", rel)
	}
	full := w.Root
	for _, part := range strings.Split(clean, "/") {
		full = filepath.Join(full, part)
		info, err := os.Lstat(full)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("ruta HCD con symlink: %q", rel)
		}
	}
	return filepath.Join(w.Root, filepath.FromSlash(clean)), nil
}

// Fingerprint identifica el código evaluado por su contenido: el id de blob
// de git de cada archivo no ignorado del árbol de trabajo, excluyendo hcd/
// para que reescribir el documento no caduque la evidencia. No depende de
// HEAD: hacer commit sin cambiar contenido no altera la huella (HCD-064).
// Fuera de git devuelve "nogit".
//
// Los archivos sin cambios toman su id del índice (`ls-files --stage`); solo
// los modificados respecto al índice y los sin rastrear se hashean, con
// `hash-object --stdin-paths`, que aplica los mismos filtros que `git add`.
// Así el id de un archivo coincide antes y después de prepararlo o
// commitearlo.
func (w Workspace) Fingerprint(ctx context.Context) (string, error) {
	if _, err := w.git(ctx, "rev-parse", "--is-inside-work-tree"); err != nil {
		return domain.FingerprintNoGit, nil
	}
	skip := func(p string) bool { return p == "hcd" || strings.HasPrefix(p, "hcd/") }

	staged, err := w.git(ctx, "ls-files", "--stage", "-z")
	if err != nil {
		return "", err
	}
	ids := map[string]string{}
	for _, rec := range splitZ(staged) {
		// "<modo> <id> <etapa>\t<ruta>"
		meta, p, ok := strings.Cut(rec, "\t")
		if fields := strings.Fields(meta); ok && len(fields) >= 2 && !skip(p) {
			ids[p] = fields[1]
		}
	}

	changed, err := w.git(ctx, "diff", "--name-only", "-z")
	if err != nil {
		return "", err
	}
	untracked, err := w.git(ctx, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}
	var dirty []string
	for _, p := range append(splitZ(changed), splitZ(untracked)...) {
		if skip(p) || strings.HasSuffix(p, "/") {
			continue
		}
		info, err := os.Lstat(filepath.Join(w.Root, filepath.FromSlash(p)))
		switch {
		case errors.Is(err, os.ErrNotExist):
			// Borrado en el árbol: ausente, igual que tras commitear el borrado.
			delete(ids, p)
		case err != nil:
			return "", err
		case info.Mode().IsRegular() && !strings.ContainsAny(p, "\n\r"):
			dirty = append(dirty, p)
		default:
			// Enlaces, rutas con saltos de línea u otros tipos: se
			// identifican por su contenido leído, sin pasar por git.
			data, err := readForFingerprint(filepath.Join(w.Root, filepath.FromSlash(p)), info)
			if err != nil {
				return "", err
			}
			h := sha256.Sum256(data)
			ids[p] = "raw:" + hex.EncodeToString(h[:])
		}
	}
	if len(dirty) > 0 {
		out, err := w.gitIn(ctx, strings.Join(dirty, "\n")+"\n", "hash-object", "--stdin-paths")
		if err != nil {
			return "", err
		}
		hashes := strings.Fields(string(out))
		if len(hashes) != len(dirty) {
			return "", fmt.Errorf("git hash-object devolvió %d ids para %d archivos", len(hashes), len(dirty))
		}
		for i, p := range dirty {
			ids[p] = hashes[i]
		}
	}

	paths := make([]string, 0, len(ids))
	for p := range ids {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	sum := sha256.New()
	for _, p := range paths {
		fmt.Fprintf(sum, "%s\x00%s\n", p, ids[p])
	}
	return "tree+" + hex.EncodeToString(sum.Sum(nil))[:12], nil
}

// readForFingerprint lee el destino de un enlace o el contenido de un archivo.
func readForFingerprint(abs string, info os.FileInfo) ([]byte, error) {
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(abs)
		return []byte(target), err
	}
	if !info.Mode().IsRegular() {
		return []byte(info.Mode().String()), nil
	}
	return os.ReadFile(abs)
}

// Run ejecuta el comando en la raíz con tiempo límite. Devuelve el código de
// salida y el final de la salida combinada, que es donde suele estar el fallo.
func (w Workspace) Run(ctx context.Context, command string) (int, string, error) {
	timeout := w.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	}
	cmd.Dir = w.Root
	// Al vencer el plazo se termina el árbol entero; WaitDelay acota además la
	// espera de los pipes si algún descendiente escapó del grupo (HCD-045).
	configureProcessTree(cmd)
	cmd.WaitDelay = waitDelay
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	// Proteger antes de recortar (HCD-043): si el recorte dejara la apertura
	// <private> fuera del extracto, ProtectText ya no podría reconocer el
	// contenido privado que queda dentro.
	excerpt := tail(domain.ProtectText(out.String()))
	if ctx.Err() == context.DeadlineExceeded {
		return -1, excerpt + "\n(tiempo límite agotado)", nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), excerpt, nil
	}
	if err != nil {
		return 0, "", err
	}
	return 0, excerpt, nil
}

func (w Workspace) git(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = w.Root
	return cmd.Output()
}

// gitIn es git con entrada estándar.
func (w Workspace) gitIn(ctx context.Context, stdin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = w.Root
	cmd.Stdin = strings.NewReader(stdin)
	return cmd.Output()
}

func splitZ(b []byte) []string {
	var out []string
	for _, s := range strings.Split(string(b), "\x00") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func tail(s string) string {
	s = strings.TrimRight(s, "\n")
	if len(s) > excerptBytes {
		s = strings.ToValidUTF8(s[len(s)-excerptBytes:], "")
	}
	lines := strings.Split(s, "\n")
	if len(lines) > excerptLines {
		lines = lines[len(lines)-excerptLines:]
	}
	return strings.Join(lines, "\n")
}

// PathExists informa si rel existe bajo la raíz. No sigue la ruta fuera de la
// raíz: una ruta absoluta, vacía o con «..» que escapa responde false.
func (w Workspace) PathExists(rel string) bool {
	clean := path.Clean(filepath.ToSlash(rel))
	if rel == "" || path.IsAbs(clean) || filepath.IsAbs(rel) || clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	_, err := os.Lstat(filepath.Join(w.Root, filepath.FromSlash(clean)))
	return err == nil
}

// MaxUnitNumber devuelve el número más alto entre los archivos hcd/HCD-*.md del workspace.
// Devuelve 0 si no hay directorio hcd/ o si no hay archivos que sigan el patrón.
func (w Workspace) MaxUnitNumber() (int, error) {
	hcdDir := filepath.Join(w.Root, "hcd")
	entries, err := os.ReadDir(hcdDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	maxNum := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "HCD-") || !strings.HasSuffix(name, ".md") {
			continue
		}
		rest := strings.TrimPrefix(name, "HCD-")
		idx := strings.Index(rest, "-")
		if idx == -1 {
			idx = strings.Index(rest, ".")
		}
		if idx > 0 {
			numStr := rest[:idx]
			var n int
			if _, err := fmt.Sscanf(numStr, "%d", &n); err == nil && n > maxNum {
				maxNum = n
			}
		}
	}
	return maxNum, nil
}
