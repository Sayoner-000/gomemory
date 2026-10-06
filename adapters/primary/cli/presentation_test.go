package cli

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"mem/adapters/primary/console"
	"mem/domain"
)

func TestHelpHasQuickStartCategoriesAndFocusedCommandHelp(t *testing.T) {
	out := captureStdout(t, Usage)
	for _, want := range []string{"INICIO RÁPIDO", "Código y arquitectura", "mem pack", "mem seed", "mem adr-sync", "--no-motion"} {
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
		for _, want := range []string{"aproximado", "tokens", "POR OPERACIÓN", "build_context", "1000", "200"} {
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
	out := captureStdout(t, func() { CmdUsage(nil, []string{"--json", "--unknown-option"}) })
	if out != "" {
		t.Fatalf("ayuda de error invade stdout: %q", out)
	}
}

func TestManualCommandFamiliesHaveSafeHelp(t *testing.T) {
	for _, command := range []string{"session", "pack", "review", "docs", "forget", "get", "gc", "purge", "octopus", "uninstall", "project"} {
		t.Run(command, func(t *testing.T) {
			out := captureStdout(t, func() { Run(command, []string{"--help"}, nil) })
			if !strings.Contains(out, "AYUDA · "+command) {
				t.Fatalf("familia sin ayuda: %q", out)
			}
		})
	}
}

func TestInstallHelpWinsOverEventsWithoutOpeningProject(t *testing.T) {
	for _, args := range [][]string{{"--events", "--help"}, {"--help", "--events"}, {"/directorio/inexistente", "--events", "-h"}, {"--agents", "none", "--events", "--help"}} {
		out := captureStdout(t, func() { CmdInstall(nil, args) })
		if !strings.Contains(out, "AYUDA · install") || !strings.Contains(out, "--events") || strings.Contains(out, "contract_version") {
			t.Errorf("ayuda incorrecta para %v: %q", args, out)
		}
	}
}
