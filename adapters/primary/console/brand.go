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

	"github.com/charmbracelet/x/ansi"
)

// PrintBrand presenta la identidad del comando solo en terminales humanas.
func PrintBrand(command string) {
	printBrand(os.Stdout, DetectEnv(), command)
}

func visualTerminal(e Env) bool {
	return e.StdoutTTY && e.Getenv != nil && e.Getenv("CI") == ""
}

func animatedTerminal(e Env) bool {
	return styledTerminal(e) && e.Width >= minRichWidth && e.Getenv("GOMEMORY_NO_MOTION") == "" && e.Getenv("GOMEMORY_REDUCED_MOTION") == ""
}

func styledTerminal(e Env) bool {
	return visualTerminal(e) && e.Getenv("TERM") != "dumb" && e.Getenv("NO_COLOR") == "" && e.Width >= 40
}

func printBrand(out io.Writer, e Env, command string) {
	if !visualTerminal(e) {
		return
	}
	layout := NewLayout(e)
	if !styledTerminal(e) {
		_, _ = fmt.Fprintln(out, "\n"+ansi.Wrap("goMemory · "+version.Version+" · "+command, layout.Width(), "")+"\n")
		return
	}
	palette := Theme(e.Getenv)
	if e.Width < 78 || (command != "help" && command != "install" && command != "welcome") {
		_, _ = fmt.Fprintln(out, "\n"+ansi.Wrap(layout.Header(command)+" · "+layout.Muted(version.Version), layout.Width(), "")+"\n")
		return
	}
	_, _ = fmt.Fprintln(out)
	colors := palette.Logo[:]
	lines := strings.Split(strings.TrimSuffix(assets.TerminalLogo, "\n"), "\n")
	markWidth := 0
	for _, line := range lines {
		markWidth = max(markWidth, ansi.StringWidth(line))
	}
	for i, line := range lines {
		text := ""
		switch i {
		case 3:
			text = layout.Header("Memoria persistente")
		case 5:
			text = layout.Muted("Para agentes de código")
		}
		logo := foreground(colors[i*len(colors)/len(lines)]) + line + "\x1b[0m"
		_, _ = fmt.Fprintln(out, "  "+logo+strings.Repeat(" ", markWidth-ansi.StringWidth(line)+4)+text)
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
			elapsed := time.Since(started).Round(time.Second).String()
			available := max(1, e.Width-ansi.StringWidth(elapsed)-7)
			_, _ = fmt.Fprintf(out, "\r\x1b[2K  %s%s\x1b[0m %s · %s", foreground(palette.Primary), frames[i%len(frames)], ansi.Truncate(label, available, "…"), elapsed)
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
