package console

import (
	"fmt"
	"io"
	"os"
	"strings"

	"mem/assets"

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
	if !styledTerminal(e) || e.Width < 78 || (command != "help" && command != "install" && command != "welcome") {
		_, _ = fmt.Fprintln(out, "\n"+layout.Banner(command)+"\n")
		return
	}
	palette := Theme(e.Getenv)
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
	_, _ = fmt.Fprintln(out, "\n"+layout.Banner(command)+"\n")
}
