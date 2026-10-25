// Package configpath resuelve rutas de configuración sin escribir a través de
// enlaces simbólicos del proyecto. Vive en un adaptador secundario para que lo
// usen tanto setup como persistence (setup importa persistence, no al revés).
package configpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Project devuelve root/rel… y se niega a seguir un symlink en cualquiera de
// sus componentes bajo root: el destino puede estar fuera del repositorio (o
// ser la configuración global del agente), y reescribirlo lo modificaría allí
// (ADM 2026-10-22).
func Project(root string, rel ...string) (string, error) {
	path := root
	var parts []string
	for _, r := range rel {
		// Un rel con varios tramos (".cursor/mcp.json") se comprueba tramo a tramo.
		parts = append(parts, strings.Split(filepath.ToSlash(r), "/")...)
	}
	for _, part := range parts {
		path = filepath.Join(path, part)
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%s es un enlace simbólico: gomemory no escribe a través de él porque modificaría un archivo fuera del proyecto", path)
		}
	}
	return path, nil
}

// Global resuelve una configuración de ámbito usuario hasta su destino real.
// Reemplazar con rename la entrada que se recibe sustituiría un enlace de
// dotfiles por un archivo normal, y la configuración de la persona dejaría de
// apuntar a su repositorio.
func Global(path string) string {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if real, err := filepath.EvalSymlinks(path); err == nil {
			return real
		}
	}
	return path
}

// ForRoot aplica Project en un proyecto y Global en el HOME (ámbito global),
// donde un enlace de dotfiles es la configuración que la persona eligió.
func ForRoot(root string, rel ...string) (string, error) {
	if home, err := os.UserHomeDir(); err == nil && sameDir(home, root) {
		return Global(filepath.Join(append([]string{root}, rel...)...)), nil
	}
	return Project(root, rel...)
}

func sameDir(a, b string) bool {
	ca, errA := filepath.Abs(a)
	cb, errB := filepath.Abs(b)
	return errA == nil && errB == nil && filepath.Clean(ca) == filepath.Clean(cb)
}
