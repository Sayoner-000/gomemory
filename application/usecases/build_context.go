package usecases

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	sb.WriteString(formatCodeArchitecture(snap))
}

// formatCodeArchitecture arma el mismo resumen compacto que
// writeCodeProviderSection embebe en get_context, como texto reusable —
// feature 018 lo reusa también para el candidato de arquitectura de
// BuildContextPack (mem pack build), en vez de duplicar el formato.
func formatCodeArchitecture(snap domain.CodeProviderSnapshot) string {
	var sb strings.Builder
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
	return sb.String()
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
	// discardedChars acumula, en CARACTERES, todo lo que acota()/fits() cortan
	// del contenido íntegro (feature 020). La línea base se deriva al final de
	// Build() como discardedChars + len(salida): así rawChars >= finalChars se
	// cumple SIEMPRE por construcción, incluso sin presupuesto (Budget<=0,
	// donde nada se descarta y raw==final) — no por sumar el contenido íntegro
	// de cada memoria, que podría contradecir la invariante cuando el
	// documento final lleva envoltorio (encabezados, viñetas) más grande que
	// el contenido crudo de una entrada corta.
	discardedChars int
	// Counter convierte caracteres a tokens al reportar. Opcional: sin él,
	// Build() no puede convertir y no reporta (nil-safe, igual que Recorder).
	Counter ports.TokenCounter
	// Recorder es el grabador de uso (feature 020, ports.UsageRecorder).
	// Opcional: con nil, Build() no registra nada y funciona exactamente
	// igual — mismo patrón nil-safe que Graph/CodeProviders.
	Recorder ports.UsageRecorder
	// Topics resuelve memorias por su clave de tópico (feature 021). Opcional y
	// nil-safe, igual que Graph/Counter/Recorder: sin él, la sección de reglas
	// fijadas simplemente no se emite.
	//
	// Es un fetch DEDICADO y no una búsqueda dentro de mems a propósito: la
	// lista viene acotada por recencia, y la semilla —creada una sola vez al
	// principio de la vida del proyecto— quedaría enterrada bajo los
	// checkpoints automáticos, desapareciendo del contexto sin error ni aviso
	// (feature 021, FR-031).
	Topics ports.MemoryTopicQuerier
	// IndexMode (feature 020, fase C): cuando está activo, el CUERPO de cada
	// memoria colapsa a un puntero `get_memory <id>` — nunca se emite
	// contenido, en ningún punto de Build(). El resto de la estructura
	// (encabezados de sección, conflictos, sinapsis, resumen de grafo de
	// código) no pasa por acota() y por tanto queda intacta en ambos modos
	// (FR-032). Valor por defecto false = modo completo, el comportamiento
	// histórico (FR-034).
	IndexMode bool
}

const (
	// entryExtractChars es el largo del extracto por entrada bajo presupuesto.
	entryExtractChars = 200
	// budgetReserve deja margen para notas de cierre y secciones finales, de modo
	// que la salida total no supere Budget.
	budgetReserve = 300
)

// reglasFijadas resuelve la memoria de reglas de trabajo por su clave de
// tópico. Devuelve nil —sin error— cuando no hay colaborador o no hay semilla:
// la ausencia de la sección es un estado válido, no un fallo.
func (b *Builder) reglasFijadas() *domain.Memory {
	if b.Topics == nil {
		return nil
	}
	m, err := b.Topics.ByTopicKey(b.Project, domain.TopicWorkRules)
	if err != nil || m == nil {
		return nil
	}
	return m
}

// cuerpoFijado emite el contenido de una memoria fijada: íntegro en modo
// completo, colapsado a puntero en modo índice. La contabilidad de la feature
// 020 se mantiene por construcción — lo que se emite entero no incrementa
// discardedChars, y la línea base sigue siendo discardedChars + len(salida).
func (b *Builder) cuerpoFijado(m domain.Memory) string {
	if b.IndexMode {
		b.discardedChars += len(m.Content)
		return fmt.Sprintf("→ `get_memory %d`", m.ID)
	}
	return strings.TrimRight(m.Content, "\n")
}

// acota devuelve el contenido de la memoria acotado al presupuesto: si hay techo
// y el contenido excede el extracto, lo trunca y adjunta el puntero al detalle.
// Sin techo (Budget<=0) devuelve el contenido íntegro.
func (b *Builder) acota(m domain.Memory) string {
	if b.IndexMode {
		// Índice puro: nunca contenido, con independencia del presupuesto.
		// Cuenta como línea base completa (nada se descarta silenciosamente:
		// se sabe exactamente lo que se dejó fuera).
		b.discardedChars += len(m.Content)
		return fmt.Sprintf("→ `get_memory %d`", m.ID)
	}
	if b.Budget <= 0 {
		return m.Content
	}
	ex := domain.Extract(m.Content, entryExtractChars)
	if ex != strings.TrimSpace(m.Content) {
		// El punto de descarte 1/2 (feature 020, research.md §2): lo cortado
		// del contenido íntegro por el extracto de 200 caracteres.
		if cut := len(m.Content) - len(ex); cut > 0 {
			b.discardedChars += cut
		}
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
	if sb.Len()+n <= b.Budget-budgetReserve {
		return true
	}
	// El punto de descarte 2/2 (feature 020, research.md §2): lo que no cupo
	// no se emite, pero cuenta como línea base — es justo lo que se ahorró.
	b.discardedChars += n
	return false
}

// descarta contabiliza como línea base el contenido de las memorias que NO se
// emiten. Lo que no se muestra debe seguir constando en el ahorro medido
// (feature 020): un descarte silencioso inflaría el ahorro reportado.
func (b *Builder) descarta(mems []domain.Memory) {
	for _, m := range mems {
		b.discardedChars += len(m.Content)
	}
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

	// Reglas de trabajo fijadas (feature 021, FR-007/FR-008): se emiten ÍNTEGRAS,
	// sin pasar por acota() ni fits(). Es una excepción declarada al presupuesto,
	// la misma que ya tienen los conflictos sin resolver y el bloque de protocolo
	// (ver el comentario del campo Budget): unas reglas recortadas a 200
	// caracteres con un puntero no las aplicaría nadie, que es justo el fallo que
	// esta sección existe para evitar.
	//
	// En IndexMode sí colapsan: ese modo es un índice PURO por contrato (feature
	// 020, FR-032) y acota() lo resuelve antes de mirar el presupuesto.
	pinnedRules := b.reglasFijadas()
	if pinnedRules != nil {
		sb.WriteString("## Reglas de trabajo (memoria fijada)\n\n")
		sb.WriteString(b.cuerpoFijado(*pinnedRules))
		sb.WriteString("\n\n")
	}

	if b.IndexMode && len(mems) == 0 {
		// Índice vacío EXPLÍCITO (feature 020, caso borde de la spec): sin
		// memorias, ninguna sección de tipo aparecería igualmente en modo
		// completo — esta línea es lo único que distingue "vacío a
		// propósito" de "una sección ausente por error".
		sb.WriteString("_Índice vacío: no hay memorias en este proyecto._\n\n")
	}

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
			// La semilla de reglas ya se emitió íntegra en su propia sección
			// (FR-009). Repetirla aquí recortada sería emitir el mismo texto
			// dos veces. Esta comparación EXIGE que ListMemories proyecte
			// topic_key (FR-030): con la columna ausente daba siempre falso.
			if pinnedRules != nil && m.ID == pinnedRules.ID {
				continue
			}
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
				// Tercer punto de descarte (feature 020, research.md §2): el
				// mayor de los tres — descarta hasta 75 de 80 registros de
				// actividad cargados.
				b.descarta(checkpoints[i:])
				break
			}
			if b.IndexMode {
				b.discardedChars += len(m.Content)
				sb.WriteString(fmt.Sprintf("- %s → `get_memory %d`\n", displayTitle(m), m.ID))
				continue
			}
			// El cuerpo pasa por acota() y fits() igual que el de cualquier otra
			// sección. Sin ellos, un checkpoint es un volcado literal del turno
			// (heredocs incluidos): esta sección sola llegó a ocupar el 68 % de un
			// documento que duplicaba con creces el techo de settings.Budget.
			line := fmt.Sprintf("- %s\n", b.acota(m))
			if !b.fits(&sb, len(line)) {
				b.descarta(checkpoints[i+1:])
				break
			}
			sb.WriteString(line)
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

	// Memoria conectada a código activo (dinámica): a diferencia de la
	// anotación estática que InsertMemory pega al content al guardar
	// (annotateImpact), esto recalcula la relación CADA VEZ que se arma el
	// contexto contra el snapshot vigente del grafo — si el código se
	// reindexó y cambiaron los hotspots, la relevancia se actualiza sola, sin
	// tocar la memoria ya guardada. Reusa ImpactFor tal como está: cero
	// cambios de puerto.
	if len(b.CodeProviders) > 0 && b.fits(&sb, 80) {
		type hotMemory struct {
			mem   domain.Memory
			fanIn int
		}
		bestByID := make(map[int64]hotMemory)
		for _, m := range mems {
			if m.Filepath == "" {
				continue
			}
			for _, cp := range b.CodeProviders {
				if cp == nil {
					continue
				}
				if ann, ok := cp.ImpactFor(m.Filepath); ok && ann.Hotspot {
					if prev, seen := bestByID[m.ID]; !seen || ann.FanIn > prev.fanIn {
						bestByID[m.ID] = hotMemory{mem: m, fanIn: ann.FanIn}
					}
				}
			}
		}
		if len(bestByID) > 0 {
			hot := make([]hotMemory, 0, len(bestByID))
			for _, h := range bestByID {
				hot = append(hot, h)
			}
			sort.Slice(hot, func(i, j int) bool {
				if hot[i].fanIn != hot[j].fanIn {
					return hot[i].fanIn > hot[j].fanIn
				}
				return hot[i].mem.ID < hot[j].mem.ID
			})
			sb.WriteString("## 🔥 Memoria conectada a código activo\n\n")
			for i, h := range hot {
				if i >= 8 {
					break
				}
				line := fmt.Sprintf("- **%s** — `%s` (fan-in %d, hotspot vigente)\n", displayTitle(h.mem), h.mem.Filepath, h.fanIn)
				if !b.fits(&sb, len(line)) {
					break
				}
				sb.WriteString(line)
			}
			sb.WriteString("\n")
		}
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

	output := sb.String()

	// Registro de uso (feature 020): raw = final + lo descartado, así que
	// raw >= final se cumple SIEMPRE, incluso en modo sin presupuesto (nada
	// descartado ⇒ raw == final). El registro ocurre DENTRO de Build() —no se
	// amplía ports.ContextBuilder, que solo expone Build()/WriteFile()— y es
	// nil-safe: sin Counter o sin Recorder, no se registra nada.
	if b.Recorder != nil && b.Counter != nil {
		finalTokens := b.Counter.Count(output)
		// El puerto ports.TokenCounter solo expone Count(text string): para
		// mantener el cómputo desacoplado de cualquier adaptador concreto (no
		// se puede importar tokens.ApproximateTokenCounter desde aquí sin
		// violar la arquitectura hexagonal), se cuenta un texto sintético de
		// la longitud correcta en vez de reimplementar la fórmula del
		// adaptador — funciona para cualquier implementación determinista en
		// función del largo del texto, que es el único contrato del puerto.
		rawTokens := b.Counter.Count(strings.Repeat("x", b.discardedChars+len(output)))
		b.Recorder.Record(domain.OpBuildContext, rawTokens, finalTokens)
	}

	return output, nil
}

func (b *Builder) WriteFile() error {
	content, err := b.Build()
	if err != nil {
		return err
	}
	path := filepath.Join(b.Root, memDir, "context.md")
	return os.WriteFile(path, []byte(content), 0644)
}
