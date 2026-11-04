package console

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// emojiGuard reconoce los emojis retirados por HCD-022: pictogramas, el
// selector de variante U+FE0F y los emojis del plano básico que se usaban.
// Los símbolos del juego de console (✓ ✗ ⚠ ○ ● ▸ ⚑ −) quedan permitidos.
var emojiGuard = regexp.MustCompile("[\U0001F300-\U0001FAFF️✅❌➖ℹ✕♥]")

func TestEmojiGuard_Detecta(t *testing.T) {
	for _, s := range []string{"✅ ok", "❌ no", "⚠️ aviso", "ℹ️ info", "➖ omitido", "✕ error", "♥ pref", "🔌 plugin", "📦 binario"} {
		if !emojiGuard.MatchString(s) {
			t.Errorf("la guarda debe detectar %q", s)
		}
	}
	for _, s := range []string{"✓ ok", "✗ no", "⚠ aviso", "○ info", "● activo", "▸ elegido", "⚑ marcado", "− omitido", "├── └── │ •"} {
		if emojiGuard.MatchString(s) {
			t.Errorf("la guarda no debe rechazar el símbolo permitido %q", s)
		}
	}
}

// HCD-022 C-03: ningún archivo de producción de adapters/primary escribe un
// emoji; los símbolos de estado salen de Mark/stateGlyph.
func TestSinEmojis(t *testing.T) {
	for _, dir := range []string{"cli", "setup", "console", "tui"} {
		t.Run(dir, func(t *testing.T) {
			root := filepath.Join("..", dir)
			err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
					return err
				}
				data, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				for i, line := range strings.Split(string(data), "\n") {
					if m := emojiGuard.FindString(line); m != "" {
						t.Errorf("%s:%d: emoji %q; usa console.Mark o el glifo de stateGlyph", p, i+1, m)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
