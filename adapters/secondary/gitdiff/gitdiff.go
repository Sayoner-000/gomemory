// Package gitdiff calcula con git el delta entre dos commits (HCD-032).
package gitdiff

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"mem/application/ports"
)

// gitTimeout acota cada llamada: el delta se calcula una vez al iniciar la
// revisión y nunca debe colgarla.
const gitTimeout = 10 * time.Second

// Diff implementa ports.RevisionDiff sobre el repositorio de Root.
type Diff struct{ Root string }

var _ ports.RevisionDiff = Diff{}

func New(root string) Diff { return Diff{Root: root} }

// ChangedFiles devuelve, ordenados, los archivos que cambiaron entre from y
// to. Ambas deben resolver a un commit: un objetivo que no lo es (un árbol de
// trabajo, un texto libre) no tiene delta comparable.
func (d Diff) ChangedFiles(from, to string) ([]string, error) {
	for _, rev := range []string{from, to} {
		if strings.TrimSpace(rev) == "" || strings.HasPrefix(rev, "-") {
			return nil, fmt.Errorf("revisión %q no es un commit", rev)
		}
		if _, err := d.git("rev-parse", "--verify", "--quiet", rev+"^{commit}"); err != nil {
			return nil, fmt.Errorf("revisión %q no es un commit: %w", rev, err)
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
