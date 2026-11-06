// Package gitdiff calcula con git el delta entre dos commits (HCD-032).
package gitdiff

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	"mem/application/ports"
)

// gitTimeout acota cada llamada: el delta se calcula una vez al iniciar la
// revisión y nunca debe colgarla.
const gitTimeout = 10 * time.Second

// Diff implementa ports.RevisionDiff sobre el repositorio de Root.
type Diff struct{ Root string }

var (
	_ ports.RevisionDiff  = Diff{}
	_ ports.RevisionFiles = Diff{}
)

func New(root string) Diff { return Diff{Root: root} }

// ChangedFiles devuelve, ordenados, los archivos que cambiaron entre from y
// to. Ambas deben resolver a un commit: un objetivo que no lo es (un árbol de
// trabajo, un texto libre) no tiene delta comparable.
func (d Diff) ChangedFiles(from, to string) ([]string, error) {
	for _, rev := range []string{from, to} {
		if err := d.verifyCommit(rev); err != nil {
			return nil, err
		}
	}
	out, err := d.git("diff", "--name-only", "--no-renames", from, to, "--")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			files = append(files, line)
		}
	}
	slices.Sort(files)
	return files, nil
}

// TextFilesAt devuelve el contenido de los archivos de texto del alcance tal
// como están en rev, con un solo `git cat-file --batch`. Los binarios (con un
// byte NUL) no cuentan: nadie los revisa como texto.
func (d Diff) TextFilesAt(rev string, scope []string) (map[string]string, error) {
	if err := d.verifyCommit(rev); err != nil {
		return nil, err
	}
	listed, err := d.git(append([]string{"ls-tree", "-r", "-z", "--name-only", rev, "--"}, scope...)...)
	if err != nil {
		return nil, err
	}
	var names []string
	var batch strings.Builder
	for _, name := range strings.Split(listed, "\x00") {
		if name == "" || strings.Contains(name, "\n") {
			continue
		}
		names = append(names, name)
		batch.WriteString(rev + ":" + name + "\n")
	}
	out := map[string]string{}
	if len(names) == 0 {
		return out, nil
	}
	raw, err := d.gitInput(batch.String(), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	r := bufio.NewReader(strings.NewReader(raw))
	for _, name := range names {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("git cat-file: salida incompleta en %s", name)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 {
			continue // «<objeto> missing»: no hay contenido que leer
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("git cat-file: tamaño inválido en %s", name)
		}
		body := make([]byte, size+1) // contenido más el salto final
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, fmt.Errorf("git cat-file: contenido incompleto en %s", name)
		}
		if fields[1] == "blob" && !bytes.Contains(body[:size], []byte{0}) {
			out[name] = string(body[:size])
		}
	}
	return out, nil
}

func (d Diff) verifyCommit(rev string) error {
	if strings.TrimSpace(rev) == "" || strings.HasPrefix(rev, "-") {
		return fmt.Errorf("revisión %q no es un commit", rev)
	}
	if _, err := d.git("rev-parse", "--verify", "--quiet", rev+"^{commit}"); err != nil {
		return fmt.Errorf("revisión %q no es un commit: %w", rev, err)
	}
	return nil
}

func (d Diff) gitInput(stdin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir, cmd.Stdin = d.Root, strings.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return string(out), nil
}

func (d Diff) git(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = d.Root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return string(out), nil
}
