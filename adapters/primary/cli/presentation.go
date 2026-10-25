package cli

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"os"
	"strings"
	"text/tabwriter"

	"mem/adapters/primary/cli/clikit"
	"mem/adapters/primary/console"
	"mem/domain"
)

// PresentationArgs solo consume opciones globales anteriores al comando.
// Nunca retira argumentos destinados a save, wrap u otros subprocesos.
func PresentationArgs(args []string) ([]string, bool) {
	motion := false
	for len(args) > 0 && args[0] == "--no-motion" {
		motion = true
		args = args[1:]
	}
	return args, motion
}

// newFlagSet conserva las opciones registradas como fuente única de la ayuda.
type commandFlags struct{ *flag.FlagSet }

func newFlagSet(name string, handling flag.ErrorHandling) *commandFlags {
	fs := flag.NewFlagSet(name, handling)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), flagHelp(fs))
	}
	return &commandFlags{fs}
}

func flagHelp(fs *flag.FlagSet) string {
	var b bytes.Buffer
	previous := fs.Output()
	fs.SetOutput(&b)
	fs.PrintDefaults()
	fs.SetOutput(previous)
	return console.NewLayout(console.DetectEnv()).Document("Uso: mem " + fs.Name() + " [opciones]\n\nOPCIONES\n" + b.String())
}

// La ayuda solicitada va a stdout; la ayuda de un error queda en el canal de
// errores. También se respeta SetOutput para parsers y consumidores inyectados.
func (f *commandFlags) Parse(args []string) error {
	usage := f.Usage
	requested := false
	f.Usage = func() { requested = true }
	err := f.FlagSet.Parse(args)
	f.Usage = usage
	if requested {
		output := f.Output()
		if err == flag.ErrHelp && output == os.Stderr {
			output = os.Stdout
		}
		fmt.Fprint(output, flagHelp(f.FlagSet))
	}
	if err != nil && err != flag.ErrHelp {
		// Un flag inválido es un error de uso: el comando no se ejecutó, así
		// que no puede terminar como éxito (C-001). Código 2, como install.
		exitProcess(2)
	}
	return err
}

// exitProcess es os.Exit; las pruebas lo sustituyen para observar el código.
var exitProcess = os.Exit

func commandHelp(command string) bool {
	aliases := map[string]string{"log": "list", "judge": "compare", "mcp-setup": "setup-mcp", "--help": "help", "-h": "help", "--version": "version", "-v": "version"}
	if alias, ok := aliases[command]; ok {
		command = alias
	}
	var b strings.Builder
	active := false
	for _, line := range strings.Split(helpReference(), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "EJEMPLOS COMUNES" {
			break
		}
		if strings.HasPrefix(trim, "mem ") {
			active = trim == "mem "+command || strings.HasPrefix(trim, "mem "+command+" ")
		}
		if trim == "" {
			active = false
		}
		if active {
			b.WriteString(line + "\n")
		}
	}
	if b.Len() == 0 {
		return false
	}
	fmt.Print(console.NewLayout(console.DetectEnv()).Document("Ayuda de " + command + ":\n\n" + b.String() + "\nUsa mem " + command + " --help para consultar las opciones registradas.\n"))
	return true
}

// Los helpers de presentación que comparten los slices viven en clikit; aquí
// quedan con su nombre de siempre para el resto del paquete.
func humanTerminal(env console.Env) bool { return clikit.HumanTerminal(env) }

func panelTable(l console.Layout, title, meta string, headers []string, rows [][]string) string {
	return clikit.PanelTable(l, title, meta, headers, rows)
}

func printListPanel(banner, title, meta string, headers []string, rows [][]string) bool {
	return clikit.PrintListPanel(banner, title, meta, headers, rows)
}

// printReport imprime la salida de un comando de estado. En terminal humana
// la primera línea ("Título — meta") titula un panel con el resto, y una
// nota final suelta queda fuera como pie; si no, el texto sale intacto.
func printReport(text string, banner string) {
	env := console.DetectEnv()
	if !humanTerminal(env) {
		fmt.Print(text)
		return
	}
	if banner != "" {
		console.PrintBrand(banner)
	}
	first, body, _ := strings.Cut(strings.TrimLeft(text, "\n"), "\n")
	title, meta, _ := strings.Cut(first, " — ")
	fmt.Print(reportPanel(console.NewLayout(env), title, meta, body))
}

func reportPanel(l console.Layout, title, meta, body string) string {
	body = strings.Trim(body, "\n")
	footer := ""
	if i := strings.LastIndex(body, "\n\n"); i >= 0 {
		if last := body[i+2:]; !strings.Contains(last, "\n") && !strings.HasPrefix(last, " ") && !strings.Contains(strings.SplitN(last, " ", 2)[0], ":") {
			body, footer = body[:i], last
		}
	}
	out := l.Report(title, meta, body)
	if footer != "" {
		out += "\n" + l.Document("  "+l.Muted(footer)) + "\n"
	}
	return out
}

// memoryPanel presenta memorias como fichas dentro de un panel: id y tipo
// arriba, el título como texto principal y un extracto atenuado.
func memoryPanel(l console.Layout, title, meta string, mems []domain.Memory) string {
	inner := min(l.Width(), 100) - 4
	lines := make([]string, 0, len(mems)*4)
	for i, m := range mems {
		if i > 0 {
			lines = append(lines, "")
		}
		date := m.CreatedAt
		if len(date) > 10 {
			date = date[:10]
		}
		lines = append(lines, l.Section(fmt.Sprintf("#%d", m.ID))+"  "+l.Muted(string(m.Type)+" · "+date), m.Title)
		excerpt, _, _ := strings.Cut(strings.TrimSpace(m.Content), "\n")
		if excerpt != "" {
			lines = append(lines, l.Muted(ansi.Truncate(excerpt, inner, "…")))
		}
	}
	return l.Panel(title, meta, "", lines)
}

func formatMemoryRows(env console.Env, headers []string, rows [][]string) string {
	if env.StdoutTTY && (env.Getenv == nil || env.Getenv("CI") == "") {
		return console.NewLayout(env).Table(headers, rows)
	}
	var b bytes.Buffer
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	separators := make([]string, len(headers))
	for i, h := range headers {
		separators[i] = strings.Repeat("-", len([]rune(h)))
	}
	fmt.Fprintln(w, strings.Join(separators, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
	return b.String()
}
