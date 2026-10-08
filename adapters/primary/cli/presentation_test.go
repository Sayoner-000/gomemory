package cli

import (
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"mem/adapters/primary/console"
	"mem/domain"
)

func TestHelpHasQuickStartCategoriesAndFocusedCommandHelp(t *testing.T) {
	out := captureStdout(t, Usage)
	for _, want := range []string{"INICIO RÁPIDO", "CÓDIGO Y ARQUITECTURA", "mem pack", "mem seed", "mem adr-sync", "--no-motion", "mem help <comando>"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q", want)
		}
	}
	out = captureStdout(t, func() { Run("help", []string{"install"}, nil) })
	for _, want := range []string{"mem install", "--agents", "--scope", "--yes", "--events"} {
		if !strings.Contains(out, want) {
			t.Errorf("help install sin %q", want)
		}
	}
	if strings.Contains(out, "mem purge") {
		t.Fatal("ayuda específica contiene comandos ajenos")
	}
}

func TestUsageHumanTableKeepsUnitsAndFits(t *testing.T) {
	report := domain.UsageReport{Project: "demo", Calls: 2, BaselineTokens: 1000, EmittedTokens: 200, ByOperation: []domain.UsageBucket{{Key: "build_context", Calls: 2, BaselineTokens: 1000, EmittedTokens: 200}}}
	for _, width := range []int{40, 80, 120} {
		e := console.Env{StdoutTTY: true, Width: width, Getenv: func(string) string { return "" }}
		out := renderUsageHuman(report, "all", e)
		plain := ansi.Strip(out)
		for _, want := range []string{"aproximado", "tokens", "Por operación", "build_context", "1 000", "200", "80%"} {
			if !strings.Contains(plain, want) {
				t.Errorf("falta %q: %q", want, plain)
			}
		}
		for _, line := range strings.Split(out, "\n") {
			if ansi.StringWidth(line) > width {
				t.Errorf("usage desborda %d: %q", width, line)
			}
		}
	}
}

func TestFlagHelpUsesRegisteredOptionsWithoutDependencies(t *testing.T) {
	out := captureStdout(t, func() { CmdSettings(nil, []string{"--help"}) })
	if !strings.Contains(out, "OPCIONES") || !strings.Contains(out, "code-graph-providers") {
		t.Fatalf("ayuda incompleta: %q", out)
	}
}

func TestPresentationFlagsOnlyConsumeLeadingGlobalOptions(t *testing.T) {
	args, motion := PresentationArgs([]string{"--no-motion", "save", "--title", "--no-motion"})
	if !motion || strings.Join(args, "|") != "save|--title|--no-motion" {
		t.Fatalf("argumentos alterados: %v %v", args, motion)
	}
}

func TestMemoryTableFitsWithoutSplittingUTF8(t *testing.T) {
	e := console.Env{StdoutTTY: true, Width: 40, Getenv: func(string) string { return "" }}
	out := formatMemoryRows(e, []string{"ID", "Título"}, [][]string{{"1", strings.Repeat("界", 30)}})
	if strings.Contains(out, "�") || !strings.Contains(out, "Título") {
		t.Fatalf("tabla inválida: %q", out)
	}
}

func TestRegisteredHelpHasConsistentSections(t *testing.T) {
	for name, command := range map[string]func(*Deps, []string){"save": CmdSave, "capture": CmdCapture, "doctor": CmdDoctor, "index": CmdIndex, "init": CmdInit, "update": CmdUpdate, "export": CmdExport, "context": CmdContext} {
		t.Run(name, func(t *testing.T) {
			out := captureStdout(t, func() { command(nil, []string{"--help"}) })
			if !strings.Contains(out, "Uso: mem "+name) || !strings.Contains(out, "OPCIONES") {
				t.Fatalf("ayuda inconsistente: %q", out)
			}
		})
	}
}

func TestBadFlagsDoNotPolluteJSONOutput(t *testing.T) {
	code := -1
	out := captureStdout(t, func() {
		code = runExpectingExit(t, func() { CmdUsage(nil, []string{"--json", "--unknown-option"}) })
	})
	if out != "" {
		t.Fatalf("ayuda de error invade stdout: %q", out)
	}
	if code != 2 {
		t.Fatalf("flag inválido: código %d, se esperaba 2", code)
	}
}

func TestManualCommandFamiliesHaveSafeHelp(t *testing.T) {
	for _, command := range []string{"session", "pack", "review", "docs", "forget", "get", "gc", "purge", "octopus", "uninstall", "project"} {
		t.Run(command, func(t *testing.T) {
			out := captureStdout(t, func() { Run(command, []string{"--help"}, nil) })
			if !strings.Contains(out, "Ayuda de "+command+":") {
				t.Fatalf("familia sin ayuda: %q", out)
			}
		})
	}
}

func TestInstallHelpWinsOverEventsWithoutOpeningProject(t *testing.T) {
	for _, args := range [][]string{{"--events", "--help"}, {"--help", "--events"}, {"/directorio/inexistente", "--events", "-h"}, {"--agents", "none", "--events", "--help"}} {
		out := captureStdout(t, func() { CmdInstall(nil, args) })
		if !strings.Contains(out, "Ayuda de install:") || !strings.Contains(out, "--events") || strings.Contains(out, "contract_version") {
			t.Errorf("ayuda incorrecta para %v: %q", args, out)
		}
	}
}

// El índice compacto no lista flags: cada comando que muestra debe tener su
// ayuda detallada en mem help <comando>, o los flags quedarían inaccesibles.
func TestHelpIndexEsCompactoYCadaComandoTieneAyuda(t *testing.T) {
	out := captureStdout(t, Usage)
	if strings.Contains(out, "--title") || strings.Contains(out, "--older-than-days") {
		t.Fatalf("el índice no debe listar flags:\n%s", out)
	}
	for _, group := range helpIndex {
		for _, entry := range group.commands {
			verb := strings.Fields(entry.command)[0]
			if !strings.Contains(out, "mem "+entry.command) {
				t.Errorf("índice sin %q", entry.command)
			}
			if help := captureStdout(t, func() { commandHelp(verb) }); !strings.Contains(help, "mem "+verb) {
				t.Errorf("mem help %s no tiene ayuda detallada", verb)
			}
		}
	}
}

// C-001: un flag inválido es un error de uso (código 2), no un éxito; un
// agente que valida el código de salida no debe creer que guardó memoria.
func TestCommandFlags_FlagInvalidoTerminaConCodigo2(t *testing.T) {
	code := -1
	exitProcess = func(c int) { code = c }
	t.Cleanup(func() { exitProcess = os.Exit })

	fs := newFlagSet("save", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.String("title", "", "")
	_ = fs.Parse([]string{"--bogus"})
	if code != 2 {
		t.Fatalf("flag inválido: código %d, se esperaba 2", code)
	}

	code = -1
	fs = newFlagSet("save", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse([]string{"--help"}); err != flag.ErrHelp || code != -1 {
		t.Fatalf("--help no es un error: err=%v código=%d", err, code)
	}
}

// C-001: los comandos sin opciones rechazan argumentos en vez de ignorarlos
// y ejecutar su acción (seed, compact) declarando éxito.
func TestComandosSinOpcionesRechazanArgumentos(t *testing.T) {
	code := -1
	exitProcess = func(c int) { code = c; panic("exit") }
	t.Cleanup(func() { exitProcess = os.Exit })
	for _, cmd := range []string{"seed", "compact", "project", "version", "update-check"} {
		code = -1
		func() {
			defer func() { _ = recover() }()
			rejectUnexpectedArgs(cmd, []string{"--bogus"})
		}()
		if code != 2 {
			t.Errorf("mem %s --bogus: código %d, se esperaba 2", cmd, code)
		}
	}
	code = -1
	rejectUnexpectedArgs("seed", nil)
	rejectUnexpectedArgs("list", []string{"-n", "3"})
	if code != -1 {
		t.Fatalf("sin argumentos, o en comandos con opciones, no debe salir: %d", code)
	}
	if err := runDocs(nil, []string{"--bogus"}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "--bogus") {
		t.Fatalf("docs --bogus debe fallar: %v", err)
	}
}
