package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mem/domain"
)

// localCopyVersionTimeout acota la identificación de una copia local: un
// ejecutable que no responde nunca bloquea un hook (R2 de la feature 034).
const localCopyVersionTimeout = 2 * time.Second

// localCopy es un ejecutable `mem` dentro de un proyecto.
type localCopy struct {
	Path    string
	Version string
}

// identifyLocalCopy devuelve la copia de gomemory del proyecto, si la hay. Solo
// cuenta un archivo regular (no un enlace ni un directorio) cuya salida de
// `version` es "gomemory <semver>": cualquier otro `mem` es de la persona y no
// se toca (FR-003).
func identifyLocalCopy(root string) (localCopy, bool) {
	path := filepath.Join(root, memBinaryName())
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return localCopy{}, false
	}
	v, ok := identifyVersion(path)
	if !ok {
		return localCopy{}, false
	}
	return localCopy{Path: path, Version: v}, true
}

// retireLocalCopy retira la copia de gomemory del proyecto cuando hay un
// binario global distinto. Sin global la copia es la instalación legítima
// (FR-002) y se conserva. Best-effort: quien llama desde un hook ignora el
// error (FR-004).
func retireLocalCopy(root, global string) (localCopy, bool, error) {
	if global == "" {
		return localCopy{}, false, nil
	}
	c, ok := identifyLocalCopy(root)
	if !ok {
		return localCopy{}, false, nil
	}
	gi, err := os.Stat(global)
	if err != nil {
		return localCopy{}, false, nil
	}
	if ci, err := os.Stat(c.Path); err != nil || os.SameFile(gi, ci) {
		return localCopy{}, false, nil
	}
	if err := os.Remove(c.Path); err != nil {
		return c, false, err
	}
	return c, true, nil
}

// retiredNotice es el aviso visible de una retirada (FR-004a).
func retiredNotice(c localCopy, global string) string {
	v := "?"
	if gv, ok := identifyVersion(global); ok {
		v = gv
	}
	return "Se retiró " + c.Path + " (v" + c.Version + "); ahora se usa el global v" + v + " (" + global + ")"
}

// identifyVersion ejecuta `<bin> version` con límite de tiempo y devuelve la
// versión (sin "v") solo si la salida es exactamente "gomemory <semver>".
func identifyVersion(bin string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), localCopyVersionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "version")
	cmd.WaitDelay = 100 * time.Millisecond
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	rest, ok := strings.CutPrefix(strings.TrimSpace(string(out)), "gomemory ")
	if !ok {
		return "", false
	}
	if _, ok := domain.ParseVersion(rest); !ok {
		return "", false
	}
	return strings.TrimPrefix(rest, "v"), true
}
