package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// Entorno aislado para las pruebas del ciclo de vida (feature 034): un HOME y
// un almacén global temporales, el binario en $HOME/.local/bin como instalación
// global y un PATH acotado. Nunca toca el HOME ni el almacén reales de quien
// ejecuta la suite (quickstart, preparación común).

var (
	lifecycleBinOnce sync.Once
	lifecycleBinPath string
	lifecycleBinErr  error
)

// buildLifecycleBinary compila el binario una sola vez por paquete.
func buildLifecycleBinary(t *testing.T) string {
	t.Helper()
	lifecycleBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "gomemory-lifecycle-bin-*")
		if err != nil {
			lifecycleBinErr = err
			return
		}
		lifecycleBinPath = filepath.Join(dir, exeName("mem"))
		args := []string{"build"}
		if os.Getenv("GOMEMORY_COVERAGE_DIR") != "" {
			args = append(args, "-cover", "-coverpkg=./...")
		}
		args = append(args, "-o", lifecycleBinPath, "./infrastructure")
		cmd := exec.Command("go", args...)
		cmd.Dir = repoRootContract(t)
		if out, err := cmd.CombinedOutput(); err != nil {
			lifecycleBinErr = errors.New(string(out))
		}
	})
	if lifecycleBinErr != nil {
		t.Fatalf("compilar binario: %v", lifecycleBinErr)
	}
	return lifecycleBinPath
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

type lifecycleSandbox struct {
	t      *testing.T
	Root   string
	Home   string
	Data   string
	BinDir string
	// GlobalBin es el `mem` instalado globalmente en el sandbox.
	GlobalBin string
	// extraEnv sobrescribe o añade variables (clave=valor) a cada ejecución.
	extraEnv map[string]string
}

// newLifecycleSandbox prepara el entorno aislado con el binario global y los
// archivos compartidos sembrados con una entrada ajena `otro` (FR-012).
func newLifecycleSandbox(t *testing.T) *lifecycleSandbox {
	t.Helper()
	root := t.TempDir()
	s := &lifecycleSandbox{
		t:        t,
		Root:     root,
		Home:     filepath.Join(root, "home"),
		Data:     filepath.Join(root, "data"),
		BinDir:   filepath.Join(root, "home", ".local", "bin"),
		extraEnv: map[string]string{"GOMEMORY_NO_UPDATE_CHECK": "1"},
	}
	if coverDir := os.Getenv("GOMEMORY_COVERAGE_DIR"); coverDir != "" {
		s.extraEnv["GOCOVERDIR"] = coverDir
	}
	s.GlobalBin = filepath.Join(s.BinDir, exeName("mem"))
	mustMkdir(t, s.BinDir)
	mustMkdir(t, s.Data)
	copyExecutable(t, buildLifecycleBinary(t), s.GlobalBin)

	mustWrite(t, filepath.Join(s.Home, ".claude.json"), `{"mcpServers":{"otro":{"command":"otro"}}}`)
	mustWrite(t, filepath.Join(s.Home, ".codex", "config.toml"), "[mcp_servers.otro]\ncommand = \"otro\"\n")
	return s
}

// Setenv fija una variable para las ejecuciones siguientes; valor vacío la quita.
func (s *lifecycleSandbox) Setenv(key, value string) {
	if value == "" {
		delete(s.extraEnv, key)
		return
	}
	s.extraEnv[key] = value
}

// Project crea un proyecto git vacío dentro del sandbox.
func (s *lifecycleSandbox) Project(name string) string {
	s.t.Helper()
	dir := filepath.Join(s.Root, name)
	mustMkdir(s.t, filepath.Join(dir, ".git"))
	return dir
}

// Env devuelve el entorno completo y controlado para ejecutar `mem`.
func (s *lifecycleSandbox) Env() []string {
	env := map[string]string{
		"HOME":               s.Home,
		"USERPROFILE":        s.Home,
		"GOMEMORY_DATA_HOME": s.Data,
		"PATH":               s.BinDir + string(os.PathListSeparator) + systemPath(),
		"TERM":               "dumb",
	}
	for k, v := range s.extraEnv {
		env[k] = v
	}
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

type lifecycleResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Run ejecuta bin (vacío = el global) con cwd en dir, stdin vacío y no TTY.
func (s *lifecycleSandbox) Run(bin, dir string, args ...string) lifecycleResult {
	return s.RunInput(bin, dir, "", args...)
}

func (s *lifecycleSandbox) RunInput(bin, dir, stdin string, args ...string) lifecycleResult {
	s.t.Helper()
	if bin == "" {
		bin = s.GlobalBin
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = s.Env()
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		s.t.Fatalf("ejecutar %s %v: %v", bin, args, err)
	}
	return lifecycleResult{ExitCode: code, Stdout: stdout.String(), Stderr: stderr.String()}
}

// systemPath conserva solo los directorios del sistema, para que el `mem`
// instalado en la máquina de quien ejecuta la suite nunca se resuelva.
func systemPath() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("SystemRoot") + `\System32`
	}
	return "/usr/bin:/bin"
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyExecutable(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close() }()
	mustMkdir(t, filepath.Dir(dst))
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestLifecycleSandbox_Aislado verifica la propia ayuda: el binario global del
// sandbox responde y el entorno no apunta al HOME real.
func TestLifecycleSandbox_Aislado(t *testing.T) {
	s := newLifecycleSandbox(t)
	res := s.Run("", s.Root, "version")
	if res.ExitCode != 0 || !strings.HasPrefix(res.Stdout, "gomemory ") {
		t.Fatalf("mem version en el sandbox: code=%d out=%q err=%q", res.ExitCode, res.Stdout, res.Stderr)
	}
	realHome, _ := os.UserHomeDir()
	for _, kv := range s.Env() {
		if strings.HasPrefix(kv, "HOME=") && strings.TrimPrefix(kv, "HOME=") == realHome {
			t.Fatal("el sandbox apunta al HOME real")
		}
	}
}
