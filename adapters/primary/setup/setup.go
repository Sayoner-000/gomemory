package setup

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
)

var PluginFS fs.FS

type PluginContext struct {
	ProjectRoot string
	BinPath     string
	Port        int
}

func InstallPlugin(fsys fs.FS, pluginDir, targetDir string, ctx *PluginContext) (int, error) {
	entries, err := fs.ReadDir(fsys, pluginDir)
	if err != nil {
		return 0, fmt.Errorf("read embedded plugin dir %s: %w", pluginDir, err)
	}

	count := 0
	for _, entry := range entries {
		n, err := copyFileOrDir(fsys, pluginDir, entry, targetDir, ctx)
		if err != nil {
			return count, fmt.Errorf("install %s/%s: %w", pluginDir, entry.Name(), err)
		}
		count += n
	}
	return count, nil
}

func GenerateConfig(path string, data map[string]interface{}) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}

	existing, err := os.ReadFile(path)
	if err == nil && len(existing) > 0 {
		return nil
	}

	tmpl, err := template.New(filepath.Base(path)).Parse(string(existing))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", path, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template %s: %w", path, err)
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}

func copyFileOrDir(fsys fs.FS, baseDir string, entry os.DirEntry, targetDir string, ctx *PluginContext) (int, error) {
	srcPath := filepath.Join(baseDir, entry.Name())
	dstPath := filepath.Join(targetDir, entry.Name())

	if entry.IsDir() {
		if err := os.MkdirAll(dstPath, 0755); err != nil {
			return 0, fmt.Errorf("create dir %s: %w", dstPath, err)
		}
		children, err := fs.ReadDir(fsys, srcPath)
		if err != nil {
			return 0, fmt.Errorf("read dir %s: %w", srcPath, err)
		}
		count := 0
		for _, child := range children {
			n, err := copyFileOrDir(fsys, srcPath, child, dstPath, ctx)
			if err != nil {
				return count, err
			}
			count += n
		}
		return count, nil
	}

	if _, err := os.Stat(dstPath); err == nil {
		return 0, nil
	}

	data, err := fs.ReadFile(fsys, srcPath)
	if err != nil {
		return 0, fmt.Errorf("read embedded file %s: %w", srcPath, err)
	}

	if ctx != nil {
		data = replacePlaceholders(data, *ctx)
	}

	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return 0, fmt.Errorf("write file %s: %w", dstPath, err)
	}
	return 1, nil
}

func replacePlaceholders(data []byte, ctx PluginContext) []byte {
	result := bytes.ReplaceAll(data, []byte("{{PROJECT_ROOT}}"), []byte(ctx.ProjectRoot))
	result = bytes.ReplaceAll(result, []byte("{{BIN_PATH}}"), []byte(ctx.BinPath))
	result = bytes.ReplaceAll(result, []byte("{{PORT}}"), []byte(fmt.Sprintf("%d", ctx.Port)))
	return result
}
