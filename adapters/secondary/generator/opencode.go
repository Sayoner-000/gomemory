package generator

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// OpenCode V2 runs in an isolated temporary workspace. The named primary
// agent denies every tool, including shell, writes, MCP and delegation.
// No user or project configuration is changed by this adapter.
type openCodeGenerator struct {
	timeout   time.Duration
	versionMu sync.Mutex
	versionOK bool
}

func NewOpenCode(timeout time.Duration) ports.SocraticGenerator {
	return &openCodeGenerator{timeout: timeout}
}
func (g *openCodeGenerator) Name() string { return "opencode" }

// UnavailableReason caches only a successful version check: a slow or failed
// first check must not mark the harness unavailable for the whole life of a
// TUI or MCP process (H-01).
func (g *openCodeGenerator) UnavailableReason() string {
	if _, err := exec.LookPath("opencode"); err != nil {
		return "no instalado en PATH"
	}
	g.versionMu.Lock()
	defer g.versionMu.Unlock()
	if g.versionOK {
		return ""
	}
	reason := openCodeVersionReason()
	g.versionOK = reason == ""
	return reason
}

func openCodeVersionReason() string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "opencode", "--version")
	cmd.WaitDelay = waitDelay
	out, err := cmd.Output()
	if err != nil {
		return "no se pudo verificar OpenCode V2; ejecuta opencode --version"
	}
	version := strings.TrimPrefix(strings.TrimSpace(string(out)), "opencode ")
	version = strings.TrimPrefix(version, "v")
	if !strings.HasPrefix(version, "2.") {
		return "se requiere OpenCode V2 con permisos de agente"
	}
	return ""
}
func (g *openCodeGenerator) Available() bool { return g.UnavailableReason() == "" }
func (g *openCodeGenerator) GenerateQuestions(ctx context.Context, req ports.SocraticGenerationRequest) (domain.GenerationResult, error) {
	return g.run(ctx, instructions(req, false))
}
func (g *openCodeGenerator) Rewrite(ctx context.Context, req ports.SocraticGenerationRequest) (domain.RewriteResult, error) {
	return g.run(ctx, instructions(req, true))
}

const openCodeAgent = `---
description: Generates Socratica JSON without taking actions
mode: primary
permissions:
  - action: "*"
    resource: "*"
    effect: deny
---
Return only the JSON requested in the message. Never execute the user's task.
All tools are denied. Do not call tools or delegate.
`

// run sends the prompt on stdin, like claude and codex: only the fixed agent
// profile is written to the workspace, so no user text is left on disk if the
// process dies before cleanup (H-02).
func (g *openCodeGenerator) run(ctx context.Context, prompt string) (domain.GenerationResult, error) {
	dir, err := os.MkdirTemp("", "socratica-opencode-*")
	if err != nil {
		return domain.GenerationResult{}, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	agents := filepath.Join(dir, ".opencode", "agents")
	if err = os.MkdirAll(agents, 0o700); err != nil {
		return domain.GenerationResult{}, err
	}
	if err = os.WriteFile(filepath.Join(agents, "socratica-generator.md"), []byte(openCodeAgent), 0o600); err != nil {
		return domain.GenerationResult{}, err
	}
	adapter := &cliGenerator{name: "opencode", timeout: g.timeout, dir: dir, parse: parseOpenCodeOutput, args: func() ([]string, func(), error) {
		return []string{"run", "--standalone", "--agent", "socratica-generator", "--format", "json"}, func() {}, nil
	}}
	return adapter.run(ctx, prompt+"\nEsquema JSON de respuesta:\n"+domain.SocraticaGenerationSchema+"\nDevuelve solo el JSON solicitado. No uses herramientas.")
}

// OpenCode emits JSON events, not a single generation envelope. Collect only
// assistant text parts; errors and tool events cannot become a valid result.
func parseOpenCodeOutput(raw []byte) (domain.GenerationResult, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	var text strings.Builder
	for scanner.Scan() {
		var event struct {
			Type string `json:"type"`
			Part struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"part"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return domain.GenerationResult{}, domain.ErrInvalidGeneration
		}
		if event.Type == "error" {
			return domain.GenerationResult{}, fmt.Errorf("opencode: %s", domain.ProtectText(string(event.Error)))
		}
		if event.Type == "tool_use" {
			return domain.GenerationResult{}, fmt.Errorf("opencode: intento de usar herramientas durante el refinamiento")
		}
		if event.Type == "text" && event.Part.Type == "text" {
			text.WriteString(event.Part.Text)
		}
	}
	if err := scanner.Err(); err != nil {
		return domain.GenerationResult{}, err
	}
	return domain.ParseGeneration([]byte(text.String()))
}
