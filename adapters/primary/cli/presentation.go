package cli

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"mem/adapters/primary/console"
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
	return err
}

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
	fmt.Print(console.NewLayout(console.DetectEnv()).Document("AYUDA · " + command + "\n\n" + b.String() + "\nUsa mem " + command + " --help para consultar las opciones registradas.\n"))
	return true
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
