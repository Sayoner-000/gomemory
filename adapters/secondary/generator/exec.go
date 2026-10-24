package generator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// waitDelay bounds how long Wait keeps reading pipes after the timeout kills
// the agent, in case a grandchild still holds them open.
const waitDelay = 200 * time.Millisecond

// Topes de lo que se captura del agente: la respuesta es un JSON acotado y
// stderr solo alimenta el mensaje de error. Son variables para que las
// pruebas los bajen.
var (
	maxGeneratorStdout = 8 << 20
	maxGeneratorStderr = 64 << 10
)

// boundedBuffer guarda hasta max bytes. Al superarlo descarta el resto y
// cancela el proceso de inmediato, sin esperar al timeout. Cada flujo tiene
// su buffer, así que exceeded solo se lee cuando cmd.Run ya terminó de copiar.
// buf no va embebido: bytes.Buffer trae ReadFrom, e io.Copy lo usaría en
// lugar de Write, saltándose el tope.
type boundedBuffer struct {
	buf      bytes.Buffer
	max      int
	exceeded bool
	cancel   func()
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.buf.Len(); len(p) > room {
		if room > 0 {
			b.buf.Write(p[:room])
		}
		if !b.exceeded {
			b.exceeded = true
			b.cancel()
		}
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *boundedBuffer) Bytes() []byte  { return b.buf.Bytes() }
func (b *boundedBuffer) String() string { return b.buf.String() }
func (b *boundedBuffer) Len() int       { return b.buf.Len() }

// cliGenerator runs one agent binary per request with the prompt on stdin.
type cliGenerator struct {
	name    string
	timeout time.Duration
	// args returns the command-line arguments and a cleanup for any
	// temporary file they reference.
	args  func() ([]string, func(), error)
	dir   string
	parse func([]byte) (domain.GenerationResult, error)
}

func (g *cliGenerator) Name() string { return g.name }

// Available reports whether the binary is on PATH; the agent's own sign-in is
// its responsibility and surfaces as a generation error.
func (g *cliGenerator) Available() bool {
	_, err := exec.LookPath(g.name)
	return err == nil
}

func (g *cliGenerator) GenerateQuestions(ctx context.Context, req ports.SocraticGenerationRequest) (domain.GenerationResult, error) {
	return g.run(ctx, instructions(req, false))
}

func (g *cliGenerator) Rewrite(ctx context.Context, req ports.SocraticGenerationRequest) (domain.RewriteResult, error) {
	return g.run(ctx, instructions(req, true))
}

func (g *cliGenerator) run(ctx context.Context, prompt string) (domain.GenerationResult, error) {
	args, cleanup, err := g.args()
	if err != nil {
		return domain.GenerationResult{}, err
	}
	defer cleanup()
	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, g.name, args...)
	cmd.Dir = g.dir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.WaitDelay = waitDelay
	stdout := &boundedBuffer{max: maxGeneratorStdout, cancel: cancel}
	stderr := &boundedBuffer{max: maxGeneratorStderr, cancel: cancel}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err = cmd.Run()
	// Antes que el timeout: la cancelación por exceso también deja ctx.Err().
	if stdout.exceeded || stderr.exceeded {
		return domain.GenerationResult{}, fmt.Errorf("%s: %w: la salida supera el límite (stdout %d, stderr %d bytes)", g.name, domain.ErrInvalidGeneration, maxGeneratorStdout, maxGeneratorStderr)
	}
	if err != nil {
		if ctx.Err() != nil {
			return domain.GenerationResult{}, fmt.Errorf("%s: %w (%s)", g.name, domain.ErrGenerationTimeout, g.timeout)
		}
		if stderr.Len() == 0 {
			parse := g.parse
			if parse == nil {
				parse = parseOutput
			}
			if _, parseErr := parse(stdout.Bytes()); parseErr != nil && !errors.Is(parseErr, domain.ErrInvalidGeneration) {
				return domain.GenerationResult{}, fmt.Errorf("%s: %w: %v", g.name, err, parseErr)
			}
		}
		return domain.GenerationResult{}, fmt.Errorf("%s: %w: %s", g.name, err, domain.ProtectText(strings.TrimSpace(stderr.String())))
	}
	if g.parse != nil {
		return g.parse(stdout.Bytes())
	}
	return parseOutput(stdout.Bytes())
}

// NewClaude: `claude -p --output-format json --json-schema <schema> --tools ""`.
// An empty tool list leaves the model unable to act on the task.
func NewClaude(timeout time.Duration) ports.SocraticGenerator {
	return &cliGenerator{name: "claude", timeout: timeout, args: func() ([]string, func(), error) {
		schema, err := compactSchema()
		return []string{"-p", "--output-format", "json", "--json-schema", schema, "--tools", ""}, func() {}, err
	}}
}

// NewCodex: `codex exec --ephemeral --sandbox read-only --skip-git-repo-check
// --output-schema <file> -`. The schema goes through a temporary file that is
// removed after the run.
func NewCodex(timeout time.Duration) ports.SocraticGenerator {
	return &cliGenerator{name: "codex", timeout: timeout, args: func() ([]string, func(), error) {
		schema, err := codexSchema()
		if err != nil {
			return nil, func() {}, err
		}
		f, err := os.CreateTemp("", "socratica-schema-*.json")
		if err != nil {
			return nil, func() {}, err
		}
		path := f.Name()
		cleanup := func() { _ = os.Remove(path) }
		if _, err = f.WriteString(schema); err != nil {
			_ = f.Close()
			cleanup()
			return nil, func() {}, err
		}
		if err = f.Close(); err != nil {
			cleanup()
			return nil, func() {}, err
		}
		return []string{"exec", "--ephemeral", "--sandbox", "read-only", "--skip-git-repo-check", "--output-schema", path, "-"}, cleanup, nil
	}}
}
