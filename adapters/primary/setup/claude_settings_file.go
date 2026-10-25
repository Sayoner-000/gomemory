package setup

import (
	"os"
	"path/filepath"

	"mem/adapters/secondary/configpath"
)

// ProjectConfigPath devuelve root/rel… y se niega a seguir un symlink en
// cualquiera de sus componentes bajo root: el destino puede estar fuera del
// repositorio (o ser la configuración global del agente), e install, update y
// uninstall lo reescribirían (ADM 2026-10-22). Ver configpath.Project.
func ProjectConfigPath(root string, rel ...string) (string, error) {
	return configpath.Project(root, rel...)
}

// ClaudeSettingsPath devuelve el .claude/settings.json de root con la misma
// protección que ProjectConfigPath, salvo en el HOME (ámbito global), donde
// el enlace se sigue: un ~/.claude enlazado desde un repositorio de dotfiles
// es la configuración que la persona eligió.
func ClaudeSettingsPath(root string) (string, error) {
	if home, err := os.UserHomeDir(); err == nil && sameDir(home, root) {
		return GlobalConfigPath(filepath.Join(root, ".claude", "settings.json")), nil
	}
	return ProjectConfigPath(root, ".claude", "settings.json")
}

// ClaudePluginDir devuelve el directorio del plugin de gomemory en root con la
// regla de ClaudeSettingsPath. uninstall lo borra con RemoveAll: con un tramo
// enlazado en un proyecto, borraría fuera del repositorio.
func ClaudePluginDir(root string) (string, error) {
	if home, err := os.UserHomeDir(); err == nil && sameDir(home, root) {
		return filepath.Join(root, ".claude", "plugins", "gomemory"), nil
	}
	return ProjectConfigPath(root, ".claude", "plugins", "gomemory")
}

// GlobalConfigPath resuelve una configuración de ámbito usuario hasta su
// destino real. WriteConfigFile reemplaza con rename la entrada que recibe: si
// recibiera el enlace de dotfiles, lo sustituiría por un archivo normal y la
// configuración de la persona dejaría de apuntar a su repositorio. Ver
// configpath.Global.
func GlobalConfigPath(path string) string {
	return configpath.Global(path)
}

// WriteConfigFile escribe data en path con un temporal y rename, que
// reemplaza la entrada del directorio sin seguir un enlace creado entre la
// comprobación y la escritura. Conserva el modo del archivo existente: un
// 0600 no pasa a ser legible por otros usuarios. Un archivo nuevo queda 0644.
func WriteConfigFile(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".gomemory-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil && cerr == nil {
		werr = os.Chmod(name, mode)
	}
	if werr == nil && cerr == nil {
		werr = os.Rename(name, path)
	}
	if werr != nil || cerr != nil {
		_ = os.Remove(name)
		if werr != nil {
			return werr
		}
		return cerr
	}
	return nil
}
