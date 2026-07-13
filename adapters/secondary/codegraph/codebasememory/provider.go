// Package codebasememory adapta el CLI de codebase-memory-mcp (DeusData) como
// un CodeGraphProvider OPCIONAL de gomemory. Trae la fuerza de su grafo ya
// indexado (clusters, hotspots, lenguajes) al contexto de sesión, sin acoplar
// gomemory a su esquema interno: se habla por su CLI (`... cli <tool> <json>`,
// salida JSON por stdout) y todo fallo degrada en silencio.
//
// No-bloqueo (patrón engram): el hot path solo lee el snapshot cacheado;
// MaybeRefresh lanza un proceso detached (`mem code-refresh`) que hace el
// trabajo lento fuera del camino de decisión. gomemory NUNCA dispara el
// indexado del proveedor (eso puede tardar minutos): si el repo no está
// indexado, el proveedor se reporta simplemente como no disponible.
package codebasememory

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"mem/domain"
)

const (
	// ProviderName identifica a este proveedor en el snapshot y el contexto.
	ProviderName = "codebase-memory-mcp"
	binaryName   = "codebase-memory-mcp"
	snapshotFile = "code_provider_snapshot.json"

	// probeTimeout acota cada llamada al CLI del proveedor (fuera del hot path).
	probeTimeout = 2 * time.Second
	// snapshotTTL: pasado este tiempo, MaybeRefresh dispara un refresco.
	snapshotTTL = 60 * time.Second

	maxClusters = 6
	maxHotspots = 6
	maxLangs    = 6
	maxTopNodes = 5
)

// Provider implementa ports.CodeGraphProvider sobre el CLI de codebase-memory-mcp.
type Provider struct {
	root    string
	memDir  string
	binPath string // ruta al binario, "" si no está en PATH

	mu          sync.Mutex
	lastTrigger time.Time
}

// New resuelve el binario del proveedor (best-effort) y arma el adaptador.
// memDir es el directorio `.memory/` absoluto donde vive el snapshot.
// binOverride (opcional, de settings) permite apuntar a otro binario; vacío =
// se busca "codebase-memory-mcp" en el PATH.
func New(root, memDir, binOverride string) *Provider {
	bin := binOverride
	switch {
	case bin == "":
		bin, _ = exec.LookPath(binaryName)
	case !filepath.IsAbs(bin):
		if resolved, err := exec.LookPath(bin); err == nil {
			bin = resolved
		}
	}
	return &Provider{root: root, memDir: memDir, binPath: bin}
}

// Name identifica al proveedor.
func (p *Provider) Name() string { return ProviderName }

func (p *Provider) snapshotPath() string { return filepath.Join(p.memDir, snapshotFile) }

// Snapshot lee el estado cacheado en disco. INSTANTÁNEO: nunca invoca al
// proveedor. Si no hay snapshot o no parsea, devuelve uno vacío (Available=false).
func (p *Provider) Snapshot() domain.CodeProviderSnapshot {
	empty := domain.CodeProviderSnapshot{Provider: ProviderName, RootPath: p.root}
	data, err := os.ReadFile(p.snapshotPath())
	if err != nil {
		return empty
	}
	var snap domain.CodeProviderSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return empty
	}
	return snap
}

// MaybeRefresh dispara un refresco detached si el snapshot está viejo. Nunca
// bloquea: retorna apenas lanza (o no) el proceso de fondo. Debounce en memoria
// evita tormentas de refrescos en el proceso largo (servidor MCP).
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
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	detach(cmd) // desacopla el hijo del proceso padre (ver proc_*.go)
	if err := cmd.Start(); err != nil {
		return
	}
	// Reap sin bloquear: evita zombies cuando el padre es largo (servidor MCP).
	go func() { _ = cmd.Wait() }()
}

// Refresh sondea el proveedor y reescribe el snapshot. Corre FUERA del hot path
// (proceso detached `mem code-refresh`), así que aquí sí bloquea con su propio
// timeout. Best-effort: cualquier fallo → snapshot Available=false. NUNCA
// dispara el indexado del proveedor.
func (p *Provider) Refresh(ctx context.Context) {
	snap := domain.CodeProviderSnapshot{
		Provider:  ProviderName,
		RootPath:  p.root,
		CheckedAt: time.Now(),
		Available: false,
	}
	// El snapshot se escribe SIEMPRE, con lo que se haya logrado (closure para
	// capturar el valor final de snap, no el inicial).
	defer func() { p.writeSnapshot(snap) }()

	if p.binPath == "" {
		return // sin binario → no disponible
	}
	project, ok := p.resolveProject(ctx)
	if !ok {
		return // repo no indexado o CLI falló → no disponible (sin indexar)
	}
	arch, ok := p.fetchArchitecture(ctx, project)
	if !ok {
		return
	}
	snap.Available = true
	snap.Architecture = arch
}

func (p *Provider) runCLI(ctx context.Context, tool, argsJSON string) ([]byte, error) {
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	// stdout lleva el JSON; stderr lleva la línea de log del proveedor.
	return exec.CommandContext(cctx, p.binPath, "cli", tool, argsJSON).Output()
}

func (p *Provider) resolveProject(ctx context.Context) (string, bool) {
	out, err := p.runCLI(ctx, "list_projects", "{}")
	if err != nil {
		return "", false
	}
	return parseProjectName(out, p.root)
}

func (p *Provider) fetchArchitecture(ctx context.Context, project string) (*domain.CodeArchitecture, bool) {
	args, _ := json.Marshal(map[string]string{"project": project})
	out, err := p.runCLI(ctx, "get_architecture", string(args))
	if err != nil {
		return nil, false
	}
	return parseArchitecture(out)
}

func (p *Provider) writeSnapshot(snap domain.CodeProviderSnapshot) {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(p.memDir, 0o755)
	_ = os.WriteFile(p.snapshotPath(), data, 0o644)
}

// parseProjectName casa el proyecto de codebase-memory-mcp cuyo root_path
// coincide con el root de gomemory (evita slugificar rutas a mano).
func parseProjectName(out []byte, root string) (string, bool) {
	var resp struct {
		Projects []struct {
			Name     string `json:"name"`
			RootPath string `json:"root_path"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", false
	}
	for _, pr := range resp.Projects {
		if pr.RootPath == root {
			return pr.Name, true
		}
	}
	return "", false
}

// parseArchitecture condensa la salida (grande) de get_architecture en el
// resumen compacto que se embebe en el contexto. Tolerante: JSON inválido → false.
func parseArchitecture(out []byte) (*domain.CodeArchitecture, bool) {
	var raw struct {
		TotalNodes int `json:"total_nodes"`
		TotalEdges int `json:"total_edges"`
		Languages  []struct {
			Language  string `json:"language"`
			FileCount int    `json:"file_count"`
		} `json:"languages"`
		Clusters []struct {
			Label    string   `json:"label"`
			Members  int      `json:"members"`
			Cohesion float64  `json:"cohesion"`
			TopNodes []string `json:"top_nodes"`
		} `json:"clusters"`
		Hotspots []struct {
			Name  string `json:"name"`
			FanIn int    `json:"fan_in"`
		} `json:"hotspots"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, false
	}
	arch := &domain.CodeArchitecture{TotalNodes: raw.TotalNodes, TotalEdges: raw.TotalEdges}
	for i, l := range raw.Languages {
		if i >= maxLangs {
			break
		}
		arch.Languages = append(arch.Languages, domain.CodeLangStat{Language: l.Language, FileCount: l.FileCount})
	}
	sort.SliceStable(raw.Clusters, func(i, j int) bool { return raw.Clusters[i].Members > raw.Clusters[j].Members })
	for i, c := range raw.Clusters {
		if i >= maxClusters {
			break
		}
		top := c.TopNodes
		if len(top) > maxTopNodes {
			top = top[:maxTopNodes]
		}
		arch.Clusters = append(arch.Clusters, domain.CodeCluster{Label: c.Label, Members: c.Members, Cohesion: c.Cohesion, TopNodes: top})
	}
	for i, h := range raw.Hotspots {
		if i >= maxHotspots {
			break
		}
		arch.Hotspots = append(arch.Hotspots, domain.CodeHotspot{Name: h.Name, FanIn: h.FanIn})
	}
	return arch, true
}
