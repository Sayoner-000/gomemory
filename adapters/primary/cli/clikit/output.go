// Package clikit es la base que comparten los paquetes de entrada de cada
// slice bajo adapters/primary/cli: dependencias, presentación y registro de
// tools MCP. No importa ningún slice ni el paquete cli.
package clikit

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"mem/adapters/primary/console"
	"mem/domain"
)

// Fail informa de un error y termina el proceso con estado 1.
func Fail(format string, args ...any) {
	msg := fmt.Sprintf("Error: "+format+"\n", args...)
	if env := console.DetectEnv(); HumanTerminal(env) {
		// Mismo color y ajuste que el resto de la salida; sigue en stderr.
		msg = console.NewLayout(env).Document(msg)
	}
	fmt.Fprint(os.Stderr, msg)
	os.Exit(1)
}

// HumanTerminal indica salida para una persona: terminal real fuera de CI.
// Fuera de ella cada comando conserva su texto plano, que agentes y scripts leen.
func HumanTerminal(env console.Env) bool {
	return env.StdoutTTY && (env.Getenv == nil || env.Getenv("CI") == "")
}

// PanelTable compone una tabla dentro de un panel, con las columnas
// numéricas alineadas a la derecha.
func PanelTable(l console.Layout, title, meta string, headers []string, rows [][]string) string {
	inner := l.WithWidth(min(l.Width(), 100) - 4)
	rows = fitLastColumn(inner.Width(), headers, rows)
	table := strings.TrimRight(inner.Table(headers, rows), "\n")
	return l.Panel(title, meta, "", strings.Split(table, "\n"))
}

// PrintListPanel presenta un listado como tabla en un panel cuando la salida
// es para una persona. Devuelve false en otro caso: el llamador conserva su
// formato plano, que agentes y scripts leen.
func PrintListPanel(banner, title, meta string, headers []string, rows [][]string) bool {
	env := console.DetectEnv()
	if !HumanTerminal(env) {
		return false
	}
	if banner != "" {
		console.PrintBrand(banner)
	}
	fmt.Print(PanelTable(console.NewLayout(env), title, meta, headers, rows))
	return true
}

// fitLastColumn recorta con "…" la columna final de texto libre (resumen,
// razonamiento) para que el listado siga siendo tabla en vez de fichas.
func fitLastColumn(width int, headers []string, rows [][]string) [][]string {
	last := len(headers) - 1
	if last < 1 {
		return rows
	}
	fixed := 2 * last
	for i := 0; i < last; i++ {
		w := ansi.StringWidth(headers[i])
		for _, row := range rows {
			if i < len(row) {
				w = max(w, ansi.StringWidth(row[i]))
			}
		}
		fixed += w
	}
	room := width - fixed
	if room < 12 {
		return rows
	}
	out := make([][]string, len(rows))
	for i, row := range rows {
		out[i] = append([]string(nil), row...)
		if last < len(row) {
			out[i][last] = ansi.Truncate(row[last], room, "…")
		}
	}
	return out
}

// TerminalSafe writes through domain.StripTerminalControls, for plain-text
// reports whose fields come from agents or storage and carry no styling of
// their own. Stripping is per character, so how fmt splits writes is moot.
type TerminalSafe struct{ W io.Writer }

func (t TerminalSafe) Write(p []byte) (int, error) {
	if _, err := io.WriteString(t.W, domain.StripTerminalControls(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}
