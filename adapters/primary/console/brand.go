package console

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"mem/assets"
	"mem/version"
)

// PrintBrand presenta la identidad del comando solo en terminales humanas.
func PrintBrand(command string) {
	printBrand(os.Stdout, DetectEnv(), command)
}

func visualTerminal(e Env) bool {
	return e.StdoutTTY && e.Getenv != nil && e.Getenv("CI") == ""
}

func animatedTerminal(e Env) bool {
	return visualTerminal(e) && e.Getenv("TERM") != "dumb" && e.Getenv("NO_COLOR") == "" && e.Width >= minRichWidth
}

func printBrand(out io.Writer, e Env, command string) {
	if !visualTerminal(e) {
		return
	}
	if !animatedTerminal(e) {
		_, _ = fmt.Fprintf(out, "\ngoMemory · %s · %s\n\n", version.Version, command)
		return
	}
	palette := Theme(e.Getenv)
	if e.Width < 78 {
		_, _ = fmt.Fprintf(out, "\n%sgo%sMemory\x1b[0m · %s\n  › %s\n\n", foreground(palette.Primary), foreground(palette.Text), version.Version, command)
		return
	}
	_, _ = fmt.Fprintln(out)
	colors := []string{Azur, Cian, Brillo, Violeta, Indigo}
	if palette.Name == "light" {
		colors = []string{Azur, Indigo, Violeta, Indigo, Violeta}
	}
	lines := strings.Split(strings.TrimSuffix(assets.TerminalLogo, "\n"), "\n")
	for i, line := range lines {
		runes := []rune(line)
		for len(runes) < 28 {
			runes = append(runes, ' ')
		}
		logo, text := string(runes[:28]), strings.TrimRight(string(runes[28:]), " ")
		_, _ = fmt.Fprintf(out, "  %s%s\x1b[0m%s%s\x1b[0m\n", foreground(colors[i*len(colors)/len(lines)]), logo, foreground(palette.Text), text)
	}
	_, _ = fmt.Fprintf(out, "\n  goMemory · \x1b[2m%s\x1b[0m  ›  %s\n\n", version.Version, command)
}

// StartProgress devuelve un cierre que detiene y limpia la animación antes
// de imprimir el resultado. Sin terminal no escribe secuencias de control.
func StartProgress(label string) func() {
	return startProgress(os.Stdout, DetectEnv(), label)
}

func startProgress(out io.Writer, e Env, label string) func() {
	if !animatedTerminal(e) {
		return func() {}
	}
	done, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		palette := Theme(e.Getenv)
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		started := time.Now()
		for i := 0; ; i++ {
			_, _ = fmt.Fprintf(out, "\r\x1b[2K  %s%s\x1b[0m %s · %s", foreground(palette.Primary), frames[i%len(frames)], label, time.Since(started).Round(time.Second))
			select {
			case <-done:
				_, _ = fmt.Fprint(out, "\r\x1b[2K")
				return
			case <-ticker.C:
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(done); <-stopped }) }
}
