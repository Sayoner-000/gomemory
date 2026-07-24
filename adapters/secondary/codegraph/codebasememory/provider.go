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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// Provider implementa tanto CodeGraphProvider (grafo de código, Historia 1)
// como ADRSyncProvider (documento único de ADR, Historia 2): ambos hablan
// con el mismo binario y resuelven el mismo proyecto, así que comparten
// implementación en vez de duplicar resolveProject/runCLI en un adaptador
// aparte.
var (
	_ ports.CodeGraphProvider = (*Provider)(nil)
	_ ports.ADRSyncProvider   = (*Provider)(nil)
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
	// identity es binOverride TAL COMO SE PIDIÓ (antes de resolver contra el
	// PATH): identifica el snapshot de ESTE proveedor entre varios candidatos
	// (feature 010, Historia 3), sin depender de si el binario existe.
	identity string

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
	return &Provider{root: root, memDir: memDir, binPath: bin, identity: binOverride}
}

// Name identifica al proveedor.
func (p *Provider) Name() string { return ProviderName }

// snapshotPath: con el proveedor "por defecto" (sin binOverride,
// autodetección en PATH) usa el nombre de archivo LEGADO, para no invalidar
// el cache de una base existente de un solo proveedor. Con un binOverride
// explícito (custom o uno de varios candidatos, Historia 3), cada identidad
// distinta usa su propio archivo — así dos proveedores configurados nunca se
// pisan el snapshot entre sí.
func (p *Provider) snapshotPath() string {
	if p.identity == "" {
		return filepath.Join(p.memDir, snapshotFile)
	}
	sum := sha256.Sum256([]byte(p.identity))
	return filepath.Join(p.memDir, fmt.Sprintf("code_provider_snapshot_%x.json", sum[:6]))
}

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
	arch, qualifiedNames, ok := p.fetchArchitecture(ctx, project)
	if !ok {
		return
	}
	// Resolución de archivo por hotspot (feature 010, Historia 1): corre acá
	// —dentro del proceso detached de Refresh, NUNCA en el hot path— porque
	// get_architecture no expone "file" y hay que pedirlo aparte por
	// hotspot. Best-effort: un hotspot sin archivo resuelto simplemente no
	// participa del match por filepath, no aborta el resto del refresco.
	p.resolveHotspotFiles(ctx, project, arch.Hotspots, qualifiedNames)
	snap.Available = true
	snap.Architecture = arch
}

// ImpactFor resuelve el impacto de un filepath contra el snapshot YA
// cacheado (instantáneo, nunca invoca al proveedor — mismo contrato de
// no-bloqueo que Snapshot()). false si no hay snapshot disponible o ningún
// hotspot casa por archivo.
func (p *Provider) ImpactFor(filepath string) (domain.CodeImpactAnnotation, bool) {
	snap := p.Snapshot()
	if !snap.Available || snap.Architecture == nil {
		return domain.CodeImpactAnnotation{}, false
	}
	for _, h := range snap.Architecture.Hotspots {
		if h.File != "" && h.File == filepath {
			return domain.CodeImpactAnnotation{Hotspot: true, Symbol: h.Name, FanIn: h.FanIn}, true
		}
	}
	return domain.CodeImpactAnnotation{}, false
}

// resolveHotspotFiles completa CodeHotspot.File por cada hotspot, vía un
// search_code por qualified_name (acotado a los hotspots ya condensados por
// parseArchitecture, máx. maxHotspots). Best-effort: sin binario, sin
// qualified_name conocido, o sin match → ese hotspot queda con File vacío y
// se sigue con el resto. Muta hotspots in-place.
func (p *Provider) resolveHotspotFiles(ctx context.Context, project string, hotspots []domain.CodeHotspot, qualifiedNames map[string]string) {
	if p.binPath == "" {
		return
	}
	for i := range hotspots {
		qn := qualifiedNames[hotspots[i].Name]
		if qn == "" {
			continue
		}
		args, err := json.Marshal(map[string]any{"pattern": qn, "project": project, "regex": false, "limit": 5})
		if err != nil {
			continue
		}
		out, err := p.runCLI(ctx, "search_code", string(args))
		if err != nil {
			continue
		}
		if file, ok := parseSearchCodeFile(out, qn); ok {
			hotspots[i].File = file
		}
	}
}

// parseSearchCodeFile toma la salida de search_code y devuelve el "file" del
// resultado cuyo qualified_name matchea exacto — search_code busca por texto
// libre, así que puede traer más de un resultado; solo el match exacto es
// confiable para no anotar impacto sobre el archivo equivocado.
func parseSearchCodeFile(out []byte, qualifiedName string) (string, bool) {
	var resp struct {
		Results []struct {
			QualifiedName string `json:"qualified_name"`
			File          string `json:"file"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", false
	}
	for _, r := range resp.Results {
		if r.QualifiedName == qualifiedName && r.File != "" {
			return r.File, true
		}
	}
	return "", false
}

// GetDocument lee el documento único de ADR del proveedor (manage_adr,
// mode=get) para este proyecto. A diferencia de Snapshot()/ImpactFor, SÍ
// invoca al proveedor (no hay snapshot cacheado de esto) — por eso solo se
// llama desde flujos ya fuera del hot path: export post-guardado
// (fire-and-forget) e import en el ciclo de refresco detached.
func (p *Provider) GetDocument(ctx context.Context) (string, error) {
	project, ok := p.resolveProject(ctx)
	if !ok {
		return "", fmt.Errorf("manage_adr: proveedor no disponible o proyecto no indexado")
	}
	args, _ := json.Marshal(map[string]string{"project": project, "mode": "get"})
	out, err := p.runCLI(ctx, "manage_adr", string(args))
	if err != nil {
		return "", fmt.Errorf("manage_adr get: %w", err)
	}
	content, ok := parseGetADRResponse(out)
	if !ok {
		return "", fmt.Errorf("manage_adr get: respuesta inesperada")
	}
	return content, nil
}

// UpdateDocument reemplaza el documento único de ADR del proveedor
// (manage_adr, mode=update) con el content ya reserializado (ver
// domain.ADRDocument.Render). Mismo criterio de "fuera del hot path" que
// GetDocument.
func (p *Provider) UpdateDocument(ctx context.Context, content string) error {
	project, ok := p.resolveProject(ctx)
	if !ok {
		return fmt.Errorf("manage_adr: proveedor no disponible o proyecto no indexado")
	}
	args, err := json.Marshal(map[string]string{"project": project, "mode": "update", "content": content})
	if err != nil {
		return err
	}
	if _, err := p.runCLI(ctx, "manage_adr", string(args)); err != nil {
		return fmt.Errorf("manage_adr update: %w", err)
	}
	return nil
}

// parseGetADRResponse extrae "content" de la respuesta de manage_adr(mode=
// get). Tolerante: "status":"no_adr" (verificado en vivo) trae content=""
// y es válido (documento vacío, no un error).
func parseGetADRResponse(out []byte) (string, bool) {
	var resp struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", false
	}
	return resp.Content, true
}

// hotspotQualifiedNames relee el mismo JSON crudo de get_architecture (el
// que ya consume parseArchitecture, sin tocar esa función ni su contrato ya
// probado) para obtener el qualified_name de cada hotspot — dato que el tipo
// de dominio no necesita conservar una vez resuelto el archivo, pero que
// hace falta para pedirlo a search_code con precisión. JSON inválido o sin
// hotspots → mapa vacío (tolerante, mismo criterio que parseArchitecture).
func hotspotQualifiedNames(out []byte) map[string]string {
	var raw struct {
		Hotspots []struct {
			Name          string `json:"name"`
			QualifiedName string `json:"qualified_name"`
		} `json:"hotspots"`
	}
	qn := make(map[string]string)
	if err := json.Unmarshal(out, &raw); err != nil {
		return qn
	}
	for _, h := range raw.Hotspots {
		if h.Name != "" && h.QualifiedName != "" {
			qn[h.Name] = h.QualifiedName
		}
	}
	return qn
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

func (p *Provider) fetchArchitecture(ctx context.Context, project string) (*domain.CodeArchitecture, map[string]string, bool) {
	args, _ := json.Marshal(map[string]string{"project": project})
	out, err := p.runCLI(ctx, "get_architecture", string(args))
	if err != nil {
		return nil, nil, false
	}
	arch, ok := parseArchitecture(out)
	if !ok {
		return nil, nil, false
	}
	return arch, hotspotQualifiedNames(out), true
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
