package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/primary/cli"
	"mem/adapters/secondary/persistence"
	"mem/domain"
)

func domainMemoryFixture(project string) domain.Memory {
	return domain.Memory{
		Project: project,
		Type:    domain.Learning,
		Content: "contenido de prueba",
	}
}

func TestParsePurgeFlagsDefaultsToCurrentProject(t *testing.T) {
	filter, yes, err := cli.ParsePurgeFlags([]string{}, "mi-proyecto")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if filter.Project != "mi-proyecto" || filter.All {
		t.Fatalf("expected alcance = proyecto actual por defecto, got %+v", filter)
	}
	if yes {
		t.Fatal("expected yes=false por defecto")
	}
}

func TestParsePurgeFlagsAllOverridesProject(t *testing.T) {
	filter, yes, err := cli.ParsePurgeFlags([]string{"--all", "--yes"}, "mi-proyecto")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !filter.All {
		t.Fatal("expected All=true con --all")
	}
	if !yes {
		t.Fatal("expected yes=true con --yes")
	}
}

func TestParsePurgeFlagsTypeAndOlderThanDays(t *testing.T) {
	filter, _, err := cli.ParsePurgeFlags([]string{"--type", "bugfix", "--older-than-days", "30"}, "mi-proyecto")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if filter.Type != "bugfix" || filter.OlderThanDays != 30 {
		t.Fatalf("expected type=bugfix, olderThanDays=30, got %+v", filter)
	}
}

func TestConfirmActionAcceptsSi(t *testing.T) {
	if !cli.ConfirmAction(strings.NewReader("si\n"), "¿continuar?") {
		t.Fatal("expected confirmacion con 'si'")
	}
	if !cli.ConfirmAction(strings.NewReader("SI\n"), "¿continuar?") {
		t.Fatal("expected confirmacion case-insensitive")
	}
}

func TestConfirmActionRejectsAnythingElse(t *testing.T) {
	if cli.ConfirmAction(strings.NewReader("no\n"), "¿continuar?") {
		t.Fatal("expected cancelacion con 'no'")
	}
	if cli.ConfirmAction(strings.NewReader("\n"), "¿continuar?") {
		t.Fatal("expected cancelacion con respuesta vacia")
	}
}

func TestParseGCFlagsDefaultsToNinetyDays(t *testing.T) {
	filter, yes, err := cli.ParseGCFlags([]string{}, "mi-proyecto")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if filter.OlderThanDays != 90 {
		t.Fatalf("expected default de 90 dias, got %d", filter.OlderThanDays)
	}
	if filter.Project != "mi-proyecto" {
		t.Fatalf("expected alcance = proyecto actual por defecto, got %+v", filter)
	}
	if yes {
		t.Fatal("expected yes=false por defecto")
	}
}

func TestParseGCFlagsOlderThanDaysOverride(t *testing.T) {
	filter, _, err := cli.ParseGCFlags([]string{"--older-than-days", "180", "--all"}, "mi-proyecto")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if filter.OlderThanDays != 180 || !filter.All {
		t.Fatalf("expected olderThanDays=180, All=true, got %+v", filter)
	}
}

// chdirTemp crea un directorio temporal con .memory/ inicializado, se mueve
// ahi (FindRoot depende del cwd), y devuelve unos Deps reales + una funcion
// de limpieza que restaura el cwd original y cierra la conexion.
func chdirTemp(t *testing.T) (*cli.Deps, string, func()) {
	t.Helper()
	target := t.TempDir()
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	db, err := persistence.Open(target)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(target); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	deps := &cli.Deps{
		ProjectRepo:     persistence.NewProjectRepository(),
		MaintenanceRepo: persistence.NewMaintenanceRepository(db, persistence.DbPath(target)),
		MemoryRepo:      persistence.NewMemoryRepository(db),
	}

	return deps, target, func() {
		db.Close()
		os.Chdir(origWd)
	}
}

func TestCmdPurgeRequiresConfirmationThenDeletes(t *testing.T) {
	deps, target, cleanup := chdirTemp(t)
	defer cleanup()

	project := deps.ProjectRepo.Key(target)
	fixture := domainMemoryFixture(project)
	if _, err := deps.MemoryRepo.Insert(&fixture); err != nil {
		t.Fatalf("insert: %v", err)
	}

	stdin, w, _ := os.Pipe()
	w.WriteString("no\n")
	w.Close()
	origStdin := os.Stdin
	os.Stdin = stdin
	cli.CmdPurge(deps, []string{})
	os.Stdin = origStdin

	mems, _ := deps.MemoryRepo.List(project, 10)
	if len(mems) != 1 {
		t.Fatalf("cancelar no debio borrar nada, got %d memorias", len(mems))
	}

	cli.CmdPurge(deps, []string{"--yes"})
	mems, _ = deps.MemoryRepo.List(project, 10)
	if len(mems) != 0 {
		t.Fatalf("expected 0 memorias tras confirmar purge --yes, got %d", len(mems))
	}
}

func TestCmdGCEndToEnd(t *testing.T) {
	deps, target, cleanup := chdirTemp(t)
	defer cleanup()

	project := deps.ProjectRepo.Key(target)
	fixture := domainMemoryFixture(project)
	if _, err := deps.MemoryRepo.Insert(&fixture); err != nil {
		t.Fatalf("insert: %v", err)
	}

	cli.CmdGC(deps, []string{"--yes", "--older-than-days", "0"})

	mems, _ := deps.MemoryRepo.List(project, 10)
	if len(mems) != 0 {
		t.Fatalf("expected 0 memorias tras gc --older-than-days 0 --yes, got %d", len(mems))
	}
}

func TestCmdCompactEndToEnd(t *testing.T) {
	deps, _, cleanup := chdirTemp(t)
	defer cleanup()

	cli.CmdCompact(deps, []string{})
	// Si no entro en panic/os.Exit, el flujo basico de mem compact funciona.
}

func TestCmdUninstallAcceptsYesFlagInAnyPosition(t *testing.T) {
	deps := &cli.Deps{ProjectRepo: persistence.NewProjectRepository()}

	for _, args := range [][]string{
		{"--yes", "TARGETDIR"},
		{"TARGETDIR", "--yes"},
	} {
		target := t.TempDir()
		if err := os.WriteFile(filepath.Join(target, "mem"), []byte("fake"), 0755); err != nil {
			t.Fatalf("write fake mem: %v", err)
		}

		resolved := make([]string, len(args))
		for i, a := range args {
			if a == "TARGETDIR" {
				resolved[i] = target
			} else {
				resolved[i] = a
			}
		}

		cli.CmdUninstall(deps, resolved)

		if _, err := os.Stat(filepath.Join(target, "mem")); err == nil {
			t.Fatalf("esperaba que --yes en args=%v se respetara y se eliminara el binario sin pedir confirmacion", args)
		}
	}
}

func TestFormatCompactResultReportsBeforeAfter(t *testing.T) {
	msg := cli.FormatCompactResult(2_000_000, 500_000)
	if !strings.Contains(msg, "liber") {
		t.Fatalf("expected mensaje que mencione espacio liberado, got %q", msg)
	}
}

func TestFormatCompactResultNoSpaceToReclaim(t *testing.T) {
	msg := cli.FormatCompactResult(500_000, 500_000)
	if !strings.Contains(strings.ToLower(msg), "nada que liberar") && !strings.Contains(strings.ToLower(msg), "sin espacio") {
		t.Fatalf("expected mensaje explicito de 'nada que liberar', got %q", msg)
	}
}
