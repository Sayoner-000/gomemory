package setup

import (
	"io/fs"
	"os"
	"path/filepath"
)

// speckitExtensionTemplatesBase es la raíz, dentro de templatesFS, de las
// plantillas embebidas del brazo extensor gomemory-context (feature 011/012).
// Vive bajo infrastructure/templates/ (cubierta por "go:embed all:templates"
// en infrastructure/main.go), con el mismo prefijo "templates/" que ya usa
// embeddedTemplate() en cmd_install.go — no hace falta una directiva
// go:embed nueva.
const speckitExtensionTemplatesBase = "templates/gomemory-context"

// speckitExtensionScriptPath es la ruta, relativa a la raíz del proyecto
// destino, del script bash del hook — el único archivo que necesita bit de
// ejecución (InstallPlugin/copyFileOrDir escribe todo lo demás en 0644,
// correcto para el resto: YAML, Markdown, el .ts del plugin de OpenCode).
const speckitExtensionScriptPath = ".specify/extensions/gomemory-context/scripts/bash/update-gomemory-context.sh"

// InstallSpeckitExtension copia el brazo extensor gomemory-context (spec
// 011) al proyecto destino, si y solo si ya tiene spec-kit inicializado
// (root/.specify presente) — nunca es un error que un proyecto no use
// spec-kit, así que en ese caso retorna nil sin tocar nada (feature 012,
// Historia 3). templatesFS nil (p. ej. en algunos tests) degrada igual: no
// hace nada, mismo criterio que embeddedTemplate() en cmd_install.go.
//
// Reutiliza InstallPlugin (ya usado para el plugin de OpenCode) para las
// tres copias: mismo criterio de escritura ya verificado en producción
// (solo reescribe un archivo si su contenido difiere del embebido, Historia
// 4), sin necesidad de una función de copia nueva.
func InstallSpeckitExtension(root string, templatesFS fs.FS) error {
	if templatesFS == nil {
		return nil
	}
	info, err := os.Stat(filepath.Join(root, ".specify"))
	if err != nil || !info.IsDir() {
		return nil
	}

	extDir := filepath.Join(root, ".specify", "extensions", "gomemory-context")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		return err
	}
	if _, err := InstallPlugin(templatesFS, speckitExtensionTemplatesBase+"/extension", extDir, nil); err != nil {
		return err
	}

	claudeSkillsDir := filepath.Join(root, ".claude", "skills")
	if err := os.MkdirAll(claudeSkillsDir, 0o755); err != nil {
		return err
	}
	if _, err := InstallPlugin(templatesFS, speckitExtensionTemplatesBase+"/claude", claudeSkillsDir, nil); err != nil {
		return err
	}

	opencodeCommandsDir := filepath.Join(root, ".opencode", "commands")
	if err := os.MkdirAll(opencodeCommandsDir, 0o755); err != nil {
		return err
	}
	if _, err := InstallPlugin(templatesFS, speckitExtensionTemplatesBase+"/opencode", opencodeCommandsDir, nil); err != nil {
		return err
	}

	return os.Chmod(filepath.Join(root, speckitExtensionScriptPath), 0o755)
}
