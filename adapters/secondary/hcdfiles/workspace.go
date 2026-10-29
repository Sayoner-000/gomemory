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

// Fingerprint identifica el código evaluado: HEAD más el contenido de los
// archivos cambiados y sin rastrear, excluyendo hcd/ para que reescribir el
// documento no caduque la evidencia. Fuera de git devuelve "nogit".
func (w Workspace) Fingerprint(ctx context.Context) (string, error) {
	if _, err := w.git(ctx, "rev-parse", "--is-inside-work-tree"); err != nil {
		return domain.FingerprintNoGit, nil
	}
	head, headErr := w.git(ctx, "rev-parse", "HEAD")
	var files []string
	if headErr == nil {
		changed, err := w.git(ctx, "diff", "HEAD", "--name-only", "-z")
		if err != nil {
			return "", err
		}
		files = append(files, splitZ(changed)...)
		untracked, err := w.git(ctx, "ls-files", "--others", "--exclude-standard", "-z")
		if err != nil {
			return "", err
		}
		files = append(files, splitZ(untracked)...)
	} else {
		// Repositorio sin commits: todo el árbol no ignorado es el sujeto.
		all, err := w.git(ctx, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
		if err != nil {
			return "", err
		}
		head, files = []byte("nohead"), splitZ(all)
	}
	sort.Strings(files)
	sum := sha256.New()
	seen := map[string]bool{}
	for _, f := range files {
		if seen[f] || f == "hcd" || strings.HasPrefix(f, "hcd/") {
			continue
		}
		seen[f] = true
		data, err := os.ReadFile(filepath.Join(w.Root, filepath.FromSlash(f)))
		if errors.Is(err, os.ErrNotExist) {
			data = []byte("\x00deleted")
		} else if err != nil {
			return "", err
		}
		fmt.Fprintf(sum, "%s\x00%d\x00", f, len(data))
		sum.Write(data)
	}
	short := strings.TrimSpace(string(head))
	if len(short) > 12 {
		short = short[:12]
	}
	return short + "+" + hex.EncodeToString(sum.Sum(nil))[:12], nil
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
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	excerpt := tail(out.String())
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
