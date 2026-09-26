package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// installed prepara un proyecto instalado con una memoria guardada.
func (s *lifecycleSandbox) installed(name string) string {
	s.t.Helper()
	p := s.Project(name)
	if r := s.Run("", p, "install", p); r.ExitCode != 0 {
		s.t.Fatalf("install %s: %s", name, r.Stdout+r.Stderr)
	}
	if r := s.Run("", p, "save", "-t", "memoria de "+name, "-y", "learning", "contenido de "+name); r.ExitCode != 0 {
		s.t.Fatalf("save %s: %s", name, r.Stderr)
	}
	return p
}

func (s *lifecycleSandbox) globalSetup() {
	s.t.Helper()
	if r := s.Run("", s.Root, "setup-mcp", "--scope", "global"); r.ExitCode != 0 {
		s.t.Fatalf("setup-mcp global: %s", r.Stdout+r.Stderr)
	}
}

// storeDirs lista los directorios de proyecto del almacén global.
func (s *lifecycleSandbox) storeDirs() []string {
	entries, _ := os.ReadDir(filepath.Join(s.Data, "projects"))
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// grepGomemory devuelve los archivos bajo root que mencionan gomemory.
func grepGomemory(t *testing.T, root string) []string {
	t.Helper()
	var hits []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err == nil && strings.Contains(strings.ToLower(string(data)), "gomemory") {
			hits = append(hits, p)
		}
		return nil
	})
	return hits
}

// snapshot resume un árbol (ruta → hash del contenido) para detectar cambios.
func snapshot(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				out[p] = "dir"
				return nil
			}
			data, _ := os.ReadFile(p)
			sum := sha256.Sum256(data)
			out[p] = hex.EncodeToString(sum[:])
			return nil
		})
	}
	return out
}

// FR-010: con el alcance de proyecto y la memoria borrada desaparece también
// la base del almacén global; la configuración global de Codex no se toca.
func TestUninstall_ProyectoBorraSuMemoriaDelAlmacen(t *testing.T) {
	s := newLifecycleSandbox(t)
	s.globalSetup()
	p := s.installed("p1")
	if n := len(s.storeDirs()); n != 1 {
		t.Fatalf("almacén antes = %v", s.storeDirs())
	}
	codexAntes := readAll(t, filepath.Join(s.Home, ".codex", "config.toml"))

	r := s.Run("", p, "uninstall", p, "--yes")
	if r.ExitCode != 0 && r.ExitCode != 3 {
		t.Fatalf("uninstall: code=%d\n%s", r.ExitCode, r.Stdout+r.Stderr)
	}
	if dirs := s.storeDirs(); len(dirs) != 0 {
		t.Errorf("la memoria del proyecto sigue en el almacén: %v", dirs)
	}
	if _, err := os.Stat(filepath.Join(p, ".memory")); !os.IsNotExist(err) {
		t.Error(".memory del proyecto sigue en disco")
	}
	if readAll(t, filepath.Join(s.Home, ".codex", "config.toml")) != codexAntes {
		t.Error("el alcance de proyecto no debe tocar la configuración global de Codex")
	}
}

func TestUninstall_ProyectoConservaLaMemoriaConKeep(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.installed("p1")
	r := s.Run("", p, "uninstall", p, "--yes", "--keep-memory")
	if r.ExitCode != 0 && r.ExitCode != 3 {
		t.Fatalf("uninstall: code=%d\n%s", r.ExitCode, r.Stdout+r.Stderr)
	}
	if len(s.storeDirs()) != 1 {
		t.Error("--keep-memory debe conservar la base del almacén")
	}
}

// SC-003, SC-011: el alcance de sistema deja cero rastros, conserva lo ajeno y
// la exportación privada se puede reimportar.
func TestUninstall_SistemaSinRastros(t *testing.T) {
	s := newLifecycleSandbox(t)
	s.globalSetup()
	p1 := s.installed("p1")
	p2 := s.installed("p2")
	export := filepath.Join(s.Root, "export")

	r := s.Run("", p1, "uninstall", "--all", "--yes", "--export", export)
	if r.ExitCode != 0 {
		t.Fatalf("uninstall --all: code=%d\n%s", r.ExitCode, r.Stdout+r.Stderr)
	}

	if hits := grepGomemory(t, s.Home); len(hits) != 0 {
		t.Errorf("quedan rastros de gomemory en HOME: %v", hits)
	}
	if _, err := os.Stat(s.Data); !os.IsNotExist(err) {
		t.Errorf("el almacén global sigue existiendo (%v)", err)
	}
	for _, p := range []string{p1, p2} {
		for _, rel := range []string{".memory", exeName("mem")} {
			if _, err := os.Stat(filepath.Join(p, rel)); !os.IsNotExist(err) {
				t.Errorf("%s/%s sigue en disco", filepath.Base(p), rel)
			}
		}
		if hits := grepGomemory(t, p); len(hits) != 0 {
			t.Errorf("quedan rastros en %s: %v", filepath.Base(p), hits)
		}
	}

	// FR-012: lo ajeno sigue intacto.
	var claude map[string]map[string]any
	if err := json.Unmarshal([]byte(readAll(t, filepath.Join(s.Home, ".claude.json"))), &claude); err != nil {
		t.Fatalf("~/.claude.json: %v", err)
	}
	if _, ok := claude["mcpServers"]["otro"]; !ok {
		t.Error("~/.claude.json perdió el servidor ajeno")
	}
	if !strings.Contains(readAll(t, filepath.Join(s.Home, ".codex", "config.toml")), "[mcp_servers.otro]") {
		t.Error("~/.codex/config.toml perdió el servidor ajeno")
	}

	// FR-009a: exportación privada con índice, reimportable (SC-011).
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(export); info.Mode().Perm() != 0o700 {
			t.Errorf("permisos del directorio de exportación = %o", info.Mode().Perm())
		}
	}
	var index []struct {
		Key, Root, File string
		Memories        int
	}
	if err := json.Unmarshal([]byte(readAll(t, filepath.Join(export, "index.json"))), &index); err != nil {
		t.Fatalf("index.json: %v", err)
	}
	if len(index) != 2 {
		t.Fatalf("índice = %+v", index)
	}
	sort.Slice(index, func(i, j int) bool { return index[i].Root < index[j].Root })
	for _, e := range index {
		// install siembra además las reglas y la constitución: al menos la
		// memoria guardada por la prueba debe estar.
		if e.Memories < 1 {
			t.Errorf("%s: %d memorias exportadas", e.Root, e.Memories)
		}
		if runtime.GOOS != "windows" {
			if info, _ := os.Stat(filepath.Join(export, e.File)); info.Mode().Perm() != 0o600 {
				t.Errorf("%s: permisos %o", e.File, info.Mode().Perm())
			}
		}
	}

	// Reimportación en una instalación limpia (SC-011): todo lo exportado se
	// recupera; lo que ya existía (las semillas) cuenta como omitido.
	fresh := newLifecycleSandbox(t)
	p3 := fresh.installed("p3")
	r = fresh.Run("", p3, "import", filepath.Join(export, index[0].File))
	m := regexp.MustCompile(`(\d+) memorias nuevas \((\d+) omitidas\)`).FindStringSubmatch(r.Stdout)
	if r.ExitCode != 0 || m == nil {
		t.Fatalf("import: code=%d\n%s", r.ExitCode, r.Stdout+r.Stderr)
	}
	nuevas, _ := strconv.Atoi(m[1])
	omitidas, _ := strconv.Atoi(m[2])
	if nuevas < 1 || nuevas+omitidas != index[0].Memories {
		t.Errorf("reimportación: %d nuevas + %d omitidas; exportadas %d", nuevas, omitidas, index[0].Memories)
	}
}

// SC-004: --dry-run no modifica nada.
func TestUninstall_DryRunNoModifica(t *testing.T) {
	s := newLifecycleSandbox(t)
	s.globalSetup()
	p := s.installed("p1")
	antes := snapshot(t, s.Home, s.Data, p)

	r := s.Run("", p, "uninstall", "--all", "--dry-run")
	if r.ExitCode != 0 {
		t.Fatalf("dry-run: code=%d\n%s", r.ExitCode, r.Stdout+r.Stderr)
	}
	despues := snapshot(t, s.Home, s.Data, p)
	for k, v := range antes {
		if despues[k] != v {
			t.Errorf("--dry-run modificó %s", k)
		}
	}
	if len(despues) != len(antes) {
		t.Errorf("--dry-run creó o borró archivos: %d → %d", len(antes), len(despues))
	}
	if !strings.Contains(r.Stdout, s.GlobalBin) {
		t.Errorf("el inventario debe listar el binario global:\n%s", r.Stdout)
	}
}

// FR-018: sin terminal y sin --yes no se borra nada y el código es 1.
func TestUninstall_SinTTYNiYesNoBorra(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.installed("p1")
	r := s.RunInput("", p, "s\n", "uninstall", p)
	if r.ExitCode != 1 {
		t.Fatalf("code=%d; quiero 1\n%s", r.ExitCode, r.Stdout+r.Stderr)
	}
	if _, err := os.Stat(filepath.Join(p, ".memory")); err != nil {
		t.Error("no debía borrarse nada")
	}
	if !strings.Contains(r.Stdout+r.Stderr, "--yes") {
		t.Errorf("debe indicar que hace falta --yes:\n%s", r.Stdout+r.Stderr)
	}
}

func TestUninstall_KeepYExportSonIncompatibles(t *testing.T) {
	s := newLifecycleSandbox(t)
	p := s.Project("p1")
	r := s.Run("", p, "uninstall", p, "--yes", "--keep-memory", "--export", filepath.Join(s.Root, "x"))
	if r.ExitCode != 2 {
		t.Fatalf("code=%d; quiero 2\n%s", r.ExitCode, r.Stdout+r.Stderr)
	}
}

// FR-009: si la exportación falla, esa memoria no se borra.
func TestUninstall_ExportFallidoNoBorraLaMemoria(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permisos de Unix")
	}
	s := newLifecycleSandbox(t)
	p := s.installed("p1")
	cerrado := filepath.Join(s.Root, "cerrado")
	mustMkdir(t, cerrado)
	if err := os.Chmod(cerrado, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cerrado, 0o755) })

	r := s.Run("", p, "uninstall", p, "--yes", "--export", filepath.Join(cerrado, "export"))
	if r.ExitCode != 3 {
		t.Fatalf("code=%d; quiero 3 (terminó con avisos)\n%s", r.ExitCode, r.Stdout+r.Stderr)
	}
	if len(s.storeDirs()) != 1 {
		t.Error("con la exportación fallida la memoria debe conservarse")
	}
}

// Aclaración 2026-09-26: --all escanea ~ (profundidad 6) para encontrar
// proyectos anteriores al registro; --no-scan solo usa los registrados.
func TestUninstall_EscaneoPorDefectoYNoScan(t *testing.T) {
	s := newLifecycleSandbox(t)
	viejo := filepath.Join(s.Home, "trabajo", "viejo")
	mustWrite(t, filepath.Join(viejo, ".memory", "settings.json"), "{}")
	hondo := filepath.Join(s.Home, "a", "b", "c", "d", "e", "f", "g", "hondo")
	mustWrite(t, filepath.Join(hondo, ".memory", "settings.json"), "{}")

	r := s.Run("", s.Root, "uninstall", "--all", "--dry-run", "--no-scan")
	if strings.Contains(r.Stdout, viejo) {
		t.Errorf("--no-scan no debe incluir proyectos sin registrar:\n%s", r.Stdout)
	}

	r = s.Run("", s.Root, "uninstall", "--all", "--dry-run")
	if !strings.Contains(r.Stdout, viejo) {
		t.Errorf("el escaneo por defecto debe encontrar %s:\n%s", viejo, r.Stdout)
	}
	if strings.Contains(r.Stdout, hondo) || !strings.Contains(r.Stdout, "--scan") {
		t.Errorf("más allá de 6 niveles no se encuentra, y el resumen avisa con --scan:\n%s", r.Stdout)
	}

	if r := s.Run("", s.Root, "uninstall", "--all", "--yes"); r.ExitCode != 0 {
		t.Fatalf("uninstall --all: code=%d\n%s", r.ExitCode, r.Stdout+r.Stderr)
	}
	if _, err := os.Stat(filepath.Join(viejo, ".memory")); !os.IsNotExist(err) {
		t.Error("el proyecto encontrado por el escaneo debía desinstalarse")
	}
}

func readAll(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
