package ports

import "context"

type AgentConfigFile struct {
	Path      string
	Content   []byte
	Truncated bool
	// Rewritten indica que el lector transformó el contenido (TOML a JSON,
	// extracto de .claude.json): sus números de línea no son los del archivo.
	Rewritten   bool
	Home        string
	PathDirs    []string
	UnsafePaths []string
}

type AgentConfigReaderPort interface {
	Files(ctx context.Context, maxFiles, maxBytes int) ([]AgentConfigFile, []string)
	Fingerprint(ctx context.Context) string
}
