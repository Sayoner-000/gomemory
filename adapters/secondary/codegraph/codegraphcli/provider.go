// Package codegraphcli adapta la CLI local de colbymchenry/codegraph al brazo
// extensor de gomemory. El hot path solo lee un snapshot; la CLI se sondea en
// `mem code-refresh`, fuera de los hooks y de la construcción de contexto.
package codegraphcli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"mem/application/ports"
	"mem/domain"
)

const (
	ProviderName = "codegraph"
	probeTimeout = 3 * time.Second
	syncTimeout  = 30 * time.Second
	indexTimeout = 10 * time.Minute
	snapshotTTL  = 60 * time.Second
)

var (
	_ ports.CodeGraphProvider = (*Provider)(nil)
	_ ports.CodeGraphIndexer  = (*Provider)(nil)
)

type Provider struct {
	root, memDir, binPath, identity string
	mu                              sync.Mutex
	lastTrigger                     time.Time
}

func New(root, memDir, command string) *Provider {
	bin := command
	if bin == "" {
		bin = ProviderName
	}
	resolved, err := exec.LookPath(bin)
	if err != nil {
		resolved = ""
	}
	return &Provider{root: root, memDir: memDir, binPath: resolved, identity: command}
}

func (p *Provider) Name() string { return ProviderName }

func (p *Provider) snapshotPath() string {
	sum := sha256.Sum256([]byte(ProviderName + ":" + p.identity))
	return filepath.Join(p.memDir, fmt.Sprintf("code_provider_snapshot_%x.json", sum[:6]))
}

func (p *Provider) Snapshot() domain.CodeProviderSnapshot {
	empty := domain.CodeProviderSnapshot{Provider: ProviderName, RootPath: p.root}
	data, err := os.ReadFile(p.snapshotPath())
	if err != nil {
		return empty
	}
	var snap domain.CodeProviderSnapshot
	if json.Unmarshal(data, &snap) != nil || snap.Provider != ProviderName || snap.RootPath != p.root {
		return empty
	}
	return snap
}

func (p *Provider) ImpactFor(string) (domain.CodeImpactAnnotation, bool) {
	// status --json no da fan-in por archivo. No inventar hotspots.
	return domain.CodeImpactAnnotation{}, false
}

func (p *Provider) MaybeRefresh() {
	if !p.Snapshot().Stale(snapshotTTL) {
		return
	}
	p.mu.Lock()
	if !p.lastTrigger.IsZero() && time.Since(p.lastTrigger) < snapshotTTL {
		p.mu.Unlock()
		return
	}
	p.lastTrigger = time.Now()
	p.mu.Unlock()

	self, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(self, "code-refresh")
	cmd.Dir = p.root
	detach(cmd)
	if cmd.Start() == nil {
		go func() { _ = cmd.Wait() }()
	}
}

type status struct {
	Initialized    bool     `json:"initialized"`
	ProjectPath    string   `json:"projectPath"`
	NodeCount      int      `json:"nodeCount"`
	EdgeCount      int      `json:"edgeCount"`
	Languages      []string `json:"languages"`
	PendingChanges struct {
		Added    int `json:"added"`
		Modified int `json:"modified"`
		Removed  int `json:"removed"`
	} `json:"pendingChanges"`
	Index struct {
		State              string `json:"state"`
		ReindexRecommended bool   `json:"reindexRecommended"`
	} `json:"index"`
}

func (s status) pending() int {
	return s.PendingChanges.Added + s.PendingChanges.Modified + s.PendingChanges.Removed
}

func parseStatus(data []byte, root string) (*domain.CodeArchitecture, bool) {
	var s status
	if json.Unmarshal(data, &s) != nil || !s.Initialized || s.ProjectPath != root || s.Index.State != "complete" || s.Index.ReindexRecommended ||
		s.pending() != 0 || s.NodeCount < 0 || s.EdgeCount < 0 {
		return nil, false
	}
	return &domain.CodeArchitecture{TotalNodes: s.NodeCount, TotalEdges: s.EdgeCount, LanguageNames: s.Languages}, true
}

func (p *Provider) status(ctx context.Context) ([]byte, error) {
	if p.binPath == "" {
		return nil, ports.ErrIndexerNotInstalled
	}
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	return exec.CommandContext(cctx, p.binPath, "status", "--json", p.root).Output()
}

func (p *Provider) Refresh(ctx context.Context) {
	snap := domain.CodeProviderSnapshot{
		Provider: ProviderName, RootPath: p.root, CheckedAt: time.Now(),
		Project: p.root, ProjectArg: "projectPath", ToolsHint: "codegraph_explore",
	}
	defer func() { p.writeSnapshot(snap) }()
	data, err := p.status(ctx)
	if err != nil {
		return
	}
	var current status
	if json.Unmarshal(data, &current) != nil || !current.Initialized || current.ProjectPath != p.root {
		return
	}
	if current.pending() > 0 {
		// El watcher solo corre cuando un agente mantiene abierto el MCP de
		// CodeGraph. Fuera de él, la sincronización incremental ocurre aquí,
		// en el proceso detached, antes de publicar un snapshot utilizable.
		cctx, cancel := context.WithTimeout(ctx, syncTimeout)
		_, err = exec.CommandContext(cctx, p.binPath, "sync", p.root).Output()
		cancel()
		if err != nil {
			return
		}
		data, err = p.status(ctx)
		if err != nil {
			return
		}
	}
	if arch, ok := parseStatus(data, p.root); ok {
		snap.Available = true
		snap.Architecture = arch
	}
}

func (p *Provider) IndexRepository(ctx context.Context, _ string) (int, int, error) {
	if p.binPath == "" {
		return 0, 0, ports.ErrIndexerNotInstalled
	}
	cctx, cancel := context.WithTimeout(ctx, indexTimeout)
	defer cancel()
	if out, err := exec.CommandContext(cctx, p.binPath, "index", "--quiet", p.root).CombinedOutput(); err != nil {
		return 0, 0, fmt.Errorf("codegraph index: %w: %s", err, out)
	}
	data, err := p.status(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("codegraph status: %w", err)
	}
	arch, ok := parseStatus(data, p.root)
	if !ok {
		return 0, 0, errors.New("codegraph index: índice incompleto o respuesta inesperada")
	}
	return arch.TotalNodes, arch.TotalEdges, nil
}

func (p *Provider) writeSnapshot(snap domain.CodeProviderSnapshot) {
	data, err := json.Marshal(snap)
	if err != nil || os.MkdirAll(p.memDir, 0o700) != nil {
		return
	}
	tmp, err := os.CreateTemp(p.memDir, ".code_provider_snapshot-*.tmp")
	if err != nil {
		return
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return
	}
	if err = tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(name, p.snapshotPath())
}
