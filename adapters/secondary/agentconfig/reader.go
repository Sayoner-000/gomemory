package agentconfig

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/pelletier/go-toml/v2"
	"io"
	"os"
	"path/filepath"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

type Reader struct {
	Home, Root string
	Paths      []string
}

// New lista los archivos de configuración de agentes del proyecto y del
// home. Sin un home absoluto (HOME ausente o relativo: systemd, cron,
// contenedores) sus rutas se omiten en vez de resolverse contra el
// directorio actual, que las confundiría con las del proyecto (R3-05), y
// Files lo informa como omitido. Una ruta compartida (proyecto en el propio
// home) se escanea una sola vez.
func New(home, root string) *Reader {
	if !filepath.IsAbs(home) {
		home = ""
	}
	candidates := []string{filepath.Join(root, ".claude", "settings.json"), filepath.Join(root, ".claude", "settings.local.json"), filepath.Join(root, ".mcp.json"), filepath.Join(root, "AGENTS.md"), filepath.Join(root, "CLAUDE.md")}
	if home != "" {
		candidates = append(candidates, filepath.Join(home, ".claude", "settings.json"), filepath.Join(home, ".claude.json"), filepath.Join(home, ".codex", "config.toml"), filepath.Join(home, ".config", "opencode", "opencode.json"), filepath.Join(home, ".config", "opencode", "opencode.jsonc"), filepath.Join(home, "AGENTS.md"), filepath.Join(home, "CLAUDE.md"))
	}
	paths := []string{}
	seen := map[string]bool{}
	for _, p := range candidates {
		if p = filepath.Clean(p); !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	return &Reader{Home: home, Root: root, Paths: paths}
}

func (r *Reader) Fingerprint(ctx context.Context) string {
	hash := sha256.New()
	for _, p := range r.Paths {
		if ctx.Err() != nil {
			return ""
		}
		info, err := os.Stat(p)
		if err == nil && info.Mode().IsRegular() {
			fmt.Fprintf(hash, "%s\x00%d\x00%d\n", p, info.Size(), info.ModTime().UnixNano())
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (r *Reader) Files(ctx context.Context, maxFiles, maxBytes int) ([]ports.AgentConfigFile, []string) {
	files := []ports.AgentConfigFile{}
	skipped := []string{}
	if r.Home == "" {
		skipped = append(skipped, "~ (HOME no definido: no se escanea la configuración del usuario)")
	}
	count := 0
	for _, p := range r.Paths {
		info, err := os.Stat(p)
		if os.IsNotExist(err) {
			continue
		}
		display := p
		if r.Home != "" {
			display = strings.ReplaceAll(p, r.Home, "~")
		}
		display = domain.StripTerminalControls(domain.RedactSecrets(display))
		if err != nil || !info.Mode().IsRegular() || ctx.Err() != nil || count >= maxFiles || info.Size() > int64(maxBytes) {
			skipped = append(skipped, display)
			continue
		}
		count++
		f, err := os.Open(p)
		if err != nil {
			skipped = append(skipped, display)
			continue
		}
		raw, readErr := io.ReadAll(io.LimitReader(f, int64(maxBytes)+1))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil || len(raw) > maxBytes {
			skipped = append(skipped, display)
			continue
		}
		rewritten := false
		if filepath.Clean(p) == filepath.Join(r.Home, ".claude.json") {
			var doc map[string]json.RawMessage
			if json.Unmarshal(raw, &doc) != nil {
				skipped = append(skipped, display)
				continue
			}
			raw, _ = json.Marshal(map[string]json.RawMessage{"mcpServers": doc["mcpServers"]})
			rewritten = true
		}
		if strings.HasSuffix(p, ".jsonc") {
			raw = stripJSONComments(raw)
		}
		if strings.HasSuffix(p, ".toml") {
			var doc map[string]any
			if toml.Unmarshal(raw, &doc) == nil {
				raw, _ = json.Marshal(doc)
				rewritten = true
			}
		}
		dirs := filepath.SplitList(os.Getenv("PATH"))
		unsafe := []string{}
		// Only metadata is inspected; hooks are never executed.
		for _, tokens := range domain.SplitShellCommands(strings.NewReplacer(`"`, " ", "'", " ", "\\n", " ").Replace(string(raw))) {
			for _, token := range tokens {
				token = strings.Trim(token, ",:{}[]")
				if !filepath.IsAbs(token) {
					continue
				}
				dir := filepath.Dir(token)
				for dir != filepath.Dir(dir) {
					if st, e := os.Stat(dir); e == nil && st.Mode().Perm()&0002 != 0 && st.Mode()&os.ModeSticky == 0 {
						unsafe = append(unsafe, token)
						break
					}
					dir = filepath.Dir(dir)
				}
			}
		}
		files = append(files, ports.AgentConfigFile{Path: p, Content: raw, Rewritten: rewritten, Home: r.Home, PathDirs: dirs, UnsafePaths: unsafe})
	}
	return files, skipped
}

// stripJSONComments preserves quoted strings and line breaks while removing JSONC comments.
func stripJSONComments(raw []byte) []byte {
	out := append([]byte(nil), raw...)
	quoted, escaped := false, false
	for i := 0; i < len(out); i++ {
		c := out[i]
		if quoted {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
			continue
		}
		if c == '/' && i+1 < len(out) {
			if out[i+1] == '/' {
				for i < len(out) && out[i] != '\n' {
					out[i] = ' '
					i++
				}
				i--
			} else if out[i+1] == '*' {
				out[i] = ' '
				out[i+1] = ' '
				i += 2
				for i+1 < len(out) && !(out[i] == '*' && out[i+1] == '/') {
					if out[i] != '\n' {
						out[i] = ' '
					}
					i++
				}
				if i+1 < len(out) {
					out[i] = ' '
					out[i+1] = ' '
					i++
				}
			}
		}
	}
	// JSONC accepts a trailing comma before a closing array/object.
	quoted, escaped = false, false
	for i, c := range out {
		if quoted {
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
			continue
		}
		if c == ',' {
			tail := bytes.TrimSpace(out[i+1:])
			if len(tail) > 0 && (tail[0] == ']' || tail[0] == '}') {
				out[i] = ' '
			}
		}
	}
	return out
}
