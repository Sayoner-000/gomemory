package usecases

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

const memDir = ".memory"

// relTitle formatea el extremo de una sinapsis: el título de la memoria si está
// entre las cargadas, o un marcador con su id si quedó fuera de la ventana.
func relTitle(titleByID map[int64]string, id int64) string {
	if t := titleByID[id]; t != "" {
		return fmt.Sprintf("%q", t)
	}
	return "(memoria previa)"
}

// writeCodeProviderSection embebe el resumen compacto del grafo externo y la
// división de trabajo (Track A): el proveedor responde el QUÉ/CÓMO del código;
// gomemory guarda el PORQUÉ. Es agnóstico al agente: va en el contexto que
// todos consumen (get_context / mem context), no en un hook de un agente.
func writeCodeProviderSection(sb *strings.Builder, snap domain.CodeProviderSnapshot) {
	a := snap.Architecture
	sb.WriteString(fmt.Sprintf("## Grafo de código externo (%s)\n\n", snap.Provider))
	sb.WriteString(fmt.Sprintf("Grafo estructural indexado: %d nodos, %d relaciones.", a.TotalNodes, a.TotalEdges))
	if len(a.Languages) > 0 {
		parts := make([]string, 0, len(a.Languages))
		for _, l := range a.Languages {
			parts = append(parts, fmt.Sprintf("%s (%d)", l.Language, l.FileCount))
		}
		sb.WriteString(" Lenguajes: " + strings.Join(parts, ", ") + ".")
	}
	sb.WriteString("\n\n")

	if len(a.Clusters) > 0 {
		sb.WriteString("Módulos de facto (clusters):\n")
		for _, c := range a.Clusters {
			line := fmt.Sprintf("- **%s** — %d símbolos, cohesión %.2f", c.Label, c.Members, c.Cohesion)
			if len(c.TopNodes) > 0 {
				line += " · " + strings.Join(c.TopNodes, ", ")
			}
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}
	if len(a.Hotspots) > 0 {
		names := make([]string, 0, len(a.Hotspots))
		for _, h := range a.Hotspots {
			names = append(names, fmt.Sprintf("%s (fan-in %d)", h.Name, h.FanIn))
		}
		sb.WriteString("Hotspots (más referenciados): " + strings.Join(names, ", ") + ".\n\n")
	}
	sb.WriteString("> Para consultas estructurales profundas (quién llama a qué, trazas de " +
		"llamadas, impacto de un diff) usa las tools del proveedor externo: search_graph, " +
		"trace_path, query_graph, get_architecture, detect_changes. gomemory guarda el PORQUÉ " +
		"(decisiones, sinapsis); el grafo externo responde el QUÉ/CÓMO del código.\n\n")
}

func displayTitle(m domain.Memory) string {
	if m.Title != "" {
		return m.Title
	}
	r := []rune(m.Content)
	if len(r) > 60 {
		return string(r[:57]) + "..."
	}
	return string(r)
}

type Builder struct {
	Lister    ports.MemoryLister
	Session   ports.SessionQuerier
	Relations ports.RelationLister
	// Graph es opcional: si está seteado (ver infrastructure/container.go) y
	// el proyecto tiene código indexado, Build() agrega un resumen del grafo.
	// nil-checked para no romper wiring/tests existentes que no lo setean.
	Graph ports.GraphStatusQuerier
	// CodeProviders son proveedores EXTERNOS de grafo de código, opcionales y
	// provider-agnósticos (ver ports.CodeGraphProvider). nil/vacío = desactivado:
	// el contexto se arma igual con el grafo propio. Cada uno solo aporta un
	// resumen leído de su snapshot cacheado (hot path, instantáneo); el refresco
	// ocurre en background. gomemory nunca depende de ellos.
	CodeProviders []ports.CodeGraphProvider
	Project       string
	Root          string
	// Budget es el techo blando (en CARACTERES) de la salida de Build(). <=0 =
	// sin límite (comportamiento histórico). Con techo, cada entrada larga se
	// acota a un extracto con puntero `get_memory <id>` y las secciones de lista
	// dejan de crecer al acercarse al techo; protocolo (lo añade el llamador) y
	// conflictos NUNCA se recortan. La contabilidad es en bytes emitidos.
	Budget int
}

const (
	// entryExtractChars es el largo del extracto por entrada bajo presupuesto.
	entryExtractChars = 200
	// budgetReserve deja margen para notas de cierre y secciones finales, de modo
	// que la salida total no supere Budget.
	budgetReserve = 300
)

// acota devuelve el contenido de la memoria acotado al presupuesto: si hay techo
// y el contenido excede el extracto, lo trunca y adjunta el puntero al detalle.
// Sin techo (Budget<=0) devuelve el contenido íntegro.
func (b *Builder) acota(m domain.Memory) string {
	if b.Budget <= 0 {
		return m.Content
	}
	ex := domain.Extract(m.Content, entryExtractChars)
	if ex != strings.TrimSpace(m.Content) {
		return fmt.Sprintf("%s → `get_memory %d`", ex, m.ID)
	}
	return ex
}

// fits indica si aún cabe una línea de n bytes bajo el techo (con reserva).
// Sin techo siempre cabe.
func (b *Builder) fits(sb *strings.Builder, n int) bool {
	if b.Budget <= 0 {
		return true
	}
	return sb.Len()+n <= b.Budget-budgetReserve
}

func New(lister ports.MemoryLister, session ports.SessionQuerier, relations ports.RelationLister, root, project string) *Builder {
	return &Builder{Lister: lister, Session: session, Relations: relations, Project: project, Root: root}
}

func (b *Builder) Build() (string, error) {
	mems, err := b.Lister.List(b.Project, 100)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("# Memoria del Proyecto\n\n")

	byType := make(map[domain.MemoryType][]domain.Memory)
	titleByID := make(map[int64]string, len(mems))
	for _, m := range mems {
		byType[m.Type] = append(byType[m.Type], m)
		titleByID[m.ID] = displayTitle(m)
	}

	if b.Relations != nil {
		if rels, err := b.Relations.List(b.Project, 200); err == nil {
			var conflicts, synapses []domain.Relation
			for _, r := range rels {
				switch r.Relation {
				case domain.ConflictsWith:
					conflicts = append(conflicts, r)
				case domain.Related, domain.Supersedes:
					synapses = append(synapses, r)
				}
			}
			if len(conflicts) > 0 {
				sb.WriteString("## ⚠ Conflictos sin resolver\n\n")
				for _, r := range conflicts {
					titleA := titleByID[r.MemoryIDA]
					titleB := titleByID[r.MemoryIDB]
					sb.WriteString(fmt.Sprintf("- [%d] %q ↔ [%d] %q — relee el código actual y llama a judge_memories para resolverlo\n",
						r.MemoryIDA, titleA, r.MemoryIDB, titleB))
				}
				sb.WriteString("\n")
			}
			if len(synapses) > 0 {
				sb.WriteString("## 🔗 Sinapsis (memorias enlazadas)\n\n")
				for i, r := range synapses {
					if i >= 12 {
						break
					}
					link := "↔"
					if r.Relation == domain.Supersedes {
						link = "⇒ supera a"
					}
					sb.WriteString(fmt.Sprintf("- [%d] %s %s [%d] %s\n",
						r.MemoryIDA, relTitle(titleByID, r.MemoryIDA), link, r.MemoryIDB, relTitle(titleByID, r.MemoryIDB)))
				}
				sb.WriteString("\n")
			}
		}
	}

	if prefs, ok := byType[domain.Preference]; ok && b.fits(&sb, 40) {
		sb.WriteString("## Preferencias del Usuario\n\n")
		for i, m := range prefs {
			line := fmt.Sprintf("- **%s**: %s\n", displayTitle(m), b.acota(m))
			if !b.fits(&sb, len(line)) {
				sb.WriteString(fmt.Sprintf("- (+%d memorias; usa search_memories/get_memory)\n", len(prefs)-i))
				break
			}
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	if arch, ok := byType[domain.Architecture]; ok && b.fits(&sb, 40) {
		sb.WriteString("## Decisiones de Arquitectura\n\n")
		for i, m := range arch {
			line := fmt.Sprintf("- **%s**: %s\n", displayTitle(m), b.acota(m))
			if m.Filepath != "" {
				line += fmt.Sprintf("  → `%s`\n", m.Filepath)
			}
			if !b.fits(&sb, len(line)) {
				sb.WriteString(fmt.Sprintf("- (+%d memorias; usa search_memories/get_memory)\n", len(arch)-i))
				break
			}
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	if dec, ok := byType[domain.Decision]; ok && b.fits(&sb, 40) {
		sb.WriteString("## Decisiones Técnicas\n\n")
		for i, m := range dec {
			line := fmt.Sprintf("- **%s**: %s\n", displayTitle(m), b.acota(m))
			if !b.fits(&sb, len(line)) {
				sb.WriteString(fmt.Sprintf("- (+%d memorias; usa search_memories/get_memory)\n", len(dec)-i))
				break
			}
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	if pat, ok := byType[domain.Pattern]; ok && b.fits(&sb, 40) {
		sb.WriteString("## Patrones y Convenciones\n\n")
		for i, m := range pat {
			line := fmt.Sprintf("- **%s**: %s\n", displayTitle(m), b.acota(m))
			if !b.fits(&sb, len(line)) {
				sb.WriteString(fmt.Sprintf("- (+%d memorias; usa search_memories/get_memory)\n", len(pat)-i))
				break
			}
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	if bugs, ok := byType[domain.Bugfix]; ok && b.fits(&sb, 40) {
		sb.WriteString("## Bugfixes\n\n")
		for i, m := range bugs {
			line := fmt.Sprintf("- **%s**: %s\n", displayTitle(m), b.acota(m))
			if m.Filepath != "" {
				line += fmt.Sprintf("  → `%s`\n", m.Filepath)
			}
			if !b.fits(&sb, len(line)) {
				sb.WriteString(fmt.Sprintf("- (+%d memorias; usa search_memories/get_memory)\n", len(bugs)-i))
				break
			}
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	if b.fits(&sb, 60) {
		sb.WriteString("## Aprendizajes Recientes\n\n")
		count := 0
		for _, m := range mems {
			if m.Type == domain.Architecture || m.Type == domain.Decision || m.Type == domain.Pattern || m.Type == domain.Bugfix || m.Type == domain.Preference || m.Type == domain.Checkpoint {
				continue
			}
			if count >= 15 {
				break
			}
			line := fmt.Sprintf("- %s", b.acota(m))
			if m.Title != "" {
				line = fmt.Sprintf("- **%s**: %s", m.Title, b.acota(m))
			}
			if m.Filepath != "" {
				line += fmt.Sprintf(" (`%s`)", m.Filepath)
			}
			line += "\n"
			if !b.fits(&sb, len(line)) {
				break
			}
			sb.WriteString(line)
			count++
		}
		sb.WriteString("\n")
	}

	if checkpoints, ok := byType[domain.Checkpoint]; ok && b.fits(&sb, 60) {
		sb.WriteString("## Actividad Reciente (auto)\n\n")
		for i, m := range checkpoints {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("- %s\n", m.Content))
		}
		sb.WriteString("\n")
	}

	if b.Graph != nil && b.fits(&sb, 120) {
		if status, err := b.Graph.Status(b.Project); err == nil && status.Nodes > 0 {
			sb.WriteString("## Código indexado\n\n")
			sb.WriteString(fmt.Sprintf("%d archivos, %d símbolos, %d relaciones.", status.Files, status.Nodes, status.Edges))
			if len(status.TopPackages) > 0 {
				names := make([]string, 0, len(status.TopPackages))
				for _, p := range status.TopPackages {
					names = append(names, p.Package)
				}
				sb.WriteString(" Paquetes principales: " + strings.Join(names, ", ") + ".")
			}
			sb.WriteString(" Usa search_code/get_symbol/list_dependencies para consultarlo.\n\n")
		}
	}

	// Grafo de código EXTERNO (opcional). Solo lee el snapshot cacheado de cada
	// proveedor (instantáneo, nunca bloquea) y, si está viejo, dispara un
	// refresco en background para la próxima vez. Sin proveedor/snapshot: nada.
	for _, cp := range b.CodeProviders {
		if cp == nil {
			continue
		}
		if snap := cp.Snapshot(); snap.Available && snap.Architecture != nil && b.fits(&sb, 200) {
			writeCodeProviderSection(&sb, snap)
		}
		cp.MaybeRefresh()
	}

	sess, _ := b.Session.Active(b.Project)
	if sess != nil {
		sb.WriteString(fmt.Sprintf("## Sesión Activa\n\n- Iniciada: %s\n", sess.CreatedAt))
		sb.WriteString("\n")
	}

	sessions, _ := b.Session.Recent(b.Project, 5)
	if len(sessions) > 0 && b.fits(&sb, 80) {
		sb.WriteString("## Sesiones Recientes\n\n")
		for _, s := range sessions {
			if s.EndedAt == nil {
				continue
			}
			summary := strings.TrimSpace(s.Summary)
			if summary == "" {
				summary = "(sin resumen)"
			}
			if b.Budget > 0 {
				summary = domain.Extract(summary, entryExtractChars)
			}
			line := fmt.Sprintf("- %s → %s: %s\n", s.CreatedAt, *s.EndedAt, summary)
			if !b.fits(&sb, len(line)) {
				break
			}
			sb.WriteString(line)
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func (b *Builder) WriteFile() error {
	content, err := b.Build()
	if err != nil {
		return err
	}
	path := filepath.Join(b.Root, memDir, "context.md")
	return os.WriteFile(path, []byte(content), 0644)
}
