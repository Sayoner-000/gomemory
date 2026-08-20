package usecases

import (
	"strconv"

	"mem/application/ports"
	"mem/domain"
)

// defaultCandidateLimit es el tope de candidatos a recuperar de
// MemoryRepository.Search cuando ContextRequest.MaxItems no especifica uno
// (feature 015). Vive aquí, no en un puerto, para no acoplar el caso de uso
// a un valor de configuración concreto — el llamador (CLI/MCP) es quien
// traduce SettingsData.ContextMaxItems a MaxItems antes de invocar
// BuildContextPack.
const defaultCandidateLimit = 20

// ContextRequest es la entrada de BuildContextPack (data-model.md).
type ContextRequest struct {
	Task           string
	Project        string
	Namespace      string
	MaxTokens      int
	MinRelevance   float32
	MaxItems       int
	IncludeSpecKit bool
	Compression    ports.CompressionLevel
	// Root es la raíz del proyecto en disco — necesaria solo cuando
	// IncludeSpecKit=true, para que SpecKitReader ubique .specify/feature.json
	// y specs/<feature>/. Vacío si no se usa Spec Kit (feature 015, Historia 4).
	Root string
	// IncludeCodeGraph y CodeProviders (feature 018): señal opcional del grafo
	// de código externo — boost de prioridad por hotspot y candidato de
	// arquitectura compacto. CodeProviders vacío/nil o IncludeCodeGraph=false
	// degrada en silencio a exactamente el comportamiento anterior a esta
	// feature (FR-002, FR-009).
	IncludeCodeGraph bool
	CodeProviders    []ports.CodeGraphProvider
	// Recorder (feature 020): opcional, admite nil. BuildContextPack ya
	// calcula RawTokens/FinalTokens en pack.Stats — este campo solo decide si
	// esas cifras, además de quedar en el ContextPack devuelto, se persisten
	// como registro de uso.
	Recorder ports.UsageRecorder
}

// BuildContextPack recupera memorias relevantes a Task dentro de Project,
// las clasifica por prioridad, comprime lo no crítico y arma un
// domain.ContextPack que nunca excede req.MaxTokens. Nunca degrada un
// desbordamiento de contenido crítico a un paquete parcial (FR-008):
// devuelve domain.ErrCriticalContextOverflow y ningún ContextPack.
func BuildContextPack(
	memRepo ports.MemoryRepository,
	compressor ports.Compressor,
	counter ports.TokenCounter,
	specKit ports.SpecKitReader,
	req ContextRequest,
) (domain.ContextPack, error) {
	if req.Task == "" || req.Project == "" || req.MaxTokens <= 0 {
		return domain.ContextPack{}, domain.ErrInvalidContextRequest
	}

	limit := req.MaxItems
	if limit <= 0 {
		limit = defaultCandidateLimit
	}
	candidates, err := memRepo.Search(req.Project, req.Task, limit)
	if err != nil {
		return domain.ContextPack{}, err
	}

	// Deduplicación (feature 015, Historia 2): reusa DetectDuplicateGroups ya
	// existente (detect_duplicates.go) sobre el subconjunto de candidatos de
	// esta solicitud, no sobre todo el proyecto (research.md §3). Conserva
	// SuggestedKeepID por grupo; el resto cuenta como duplicado descartado.
	groups := DetectDuplicateGroups(candidates, duplicateSimilarityThreshold)
	duplicateIDs := make(map[int64]bool)
	for _, g := range groups {
		for _, m := range g.Memories {
			if m.ID != g.SuggestedKeepID {
				duplicateIDs[m.ID] = true
			}
		}
	}

	items := make([]contextCandidate, 0, len(candidates))
	var retrieved int
	for i, m := range candidates {
		// Checkpoint es un log automático de actividad, nunca conocimiento
		// curado — nunca candidato (mismo criterio que DetectDuplicateGroups).
		if m.Type == domain.Checkpoint {
			continue
		}
		retrieved++
		if duplicateIDs[m.ID] {
			continue
		}
		items = append(items, newContextCandidate(m, i, len(candidates), req.MinRelevance))
	}

	// Spec Kit (feature 015, Historia 4): acotado SIEMPRE a la feature activa
	// (.specify/feature.json) — nunca mezcla otras features del proyecto
	// (FR-015). Un proyecto sin Spec Kit, o sin feature activa, simplemente no
	// aporta candidatos aquí; no es un error.
	if req.IncludeSpecKit && specKit != nil && req.Root != "" {
		if feature, ferr := specKit.ActiveFeature(req.Root); ferr == nil && feature != "" {
			if skCtx, rerr := specKit.Read(req.Root, feature, req.Task); rerr == nil {
				skItems := specKitCandidates(skCtx)
				items = append(items, skItems...)
				retrieved += len(skItems)
			}
		}
	}

	// Grafo de código externo (feature 018): brazo extensor opcional, mismo
	// contrato de no-bloqueo que ya usa build_context.go — solo lee snapshots
	// ya cacheados, nunca invoca al proveedor en vivo. req.CodeProviders
	// vacío/nil o IncludeCodeGraph=false degrada en silencio a exactamente el
	// comportamiento anterior a esta feature (FR-002, FR-009).
	if req.IncludeCodeGraph {
		boostHotspotCandidates(items, req.CodeProviders)
		if archCandidate, ok := codeGraphArchitectureCandidate(req.CodeProviders); ok {
			items = append(items, archCandidate)
			retrieved++
		}
	}

	// req.Compression zero value == ports.CompressionStructural (FR-009: la
	// compresión determinista es el default, no un opt-in).
	compressOpts := ports.CompressionOptions{
		Level:          req.Compression,
		PreserveCode:   true,
		PreserveURLs:   true,
		PreservePaths:  true,
		PreserveErrors: true,
	}

	pack := domain.ContextPack{Budget: req.MaxTokens}
	pack.Stats.ItemsRetrieved = retrieved
	pack.Stats.ItemsDuplicate = len(duplicateIDs)

	var criticalSum int
	for _, c := range items {
		if c.priority == domain.PriorityCritical {
			criticalSum += counter.Count(c.content)
		}
	}
	if criticalSum > req.MaxTokens {
		return domain.ContextPack{}, domain.ErrCriticalContextOverflow
	}

	remaining := req.MaxTokens
	for _, c := range items {
		rawTokens := counter.Count(c.content)
		pack.RawTokenCount += rawTokens

		content := c.content
		compressed := false
		finalTokens := rawTokens
		if c.priority != domain.PriorityCritical && compressOpts.Level != ports.CompressionNone {
			res, cerr := compressor.Compress(c.content, compressOpts)
			if cerr == nil {
				// FR-011: un fallo de compresión sigue con el original; un
				// resultado más largo que el original también se descarta
				// (invariante Tokens<=RawTokens, data-model.md).
				candidateTokens := counter.Count(res.Content)
				if candidateTokens <= rawTokens {
					content = res.Content
					finalTokens = candidateTokens
					compressed = res.Content != c.content
				}
			}
		}

		item := domain.ContextItem{
			ID:         c.id,
			Content:    content,
			Source:     c.source,
			Priority:   c.priority,
			Relevance:  c.relevance,
			Importance: c.importance,
			Confidence: c.confidence,
			RawTokens:  rawTokens,
			Tokens:     finalTokens,
			Compressed: compressed,
		}

		switch {
		case c.priority == domain.PriorityCritical:
			// Ya reservado en criticalSum: siempre entra completo.
			pack.Items = append(pack.Items, item)
			pack.TokenCount += finalTokens
			remaining -= finalTokens
			pack.Stats.ItemsCritical++
		case finalTokens <= remaining:
			pack.Items = append(pack.Items, item)
			pack.TokenCount += finalTokens
			remaining -= finalTokens
			if c.priority == domain.PriorityRelevant {
				pack.Stats.ItemsRelevant++
			} else {
				pack.Stats.ItemsOptional++
			}
		default:
			pack.Stats.ItemsDiscarded++
		}
	}

	if pack.RawTokenCount > 0 {
		pack.CompressionRate = 1 - float64(pack.TokenCount)/float64(pack.RawTokenCount)
	}
	pack.Stats.RawTokens = pack.RawTokenCount
	pack.Stats.FinalTokens = pack.TokenCount
	pack.Stats.SavedTokens = pack.RawTokenCount - pack.TokenCount
	pack.Stats.CompressionRatio = pack.CompressionRate

	if req.Recorder != nil {
		req.Recorder.Record(domain.OpBuildPack, pack.Stats.RawTokens, pack.Stats.FinalTokens)
	}

	return pack, nil
}

// contextCandidate es el estado intermedio de un domain.Memory mientras se
// arma el ContextPack: ya tiene Priority/Relevance/Importance resueltos,
// pero todavía no el conteo de tokens final (depende del TokenCounter
// inyectado, no de este paquete).
type contextCandidate struct {
	id         string
	content    string
	source     string
	priority   domain.Priority
	relevance  float32
	importance float32
	confidence float32
}

// newContextCandidate deriva relevancia/importancia/prioridad de señales que
// YA existen en domain.Memory, sin depender de campos importance/confidence
// que hoy no existen en el dominio (research.md §2):
//   - Relevance: posición en los resultados de Search (FTS5/BM25), que ya
//     devuelve mejor coincidencia primero.
//   - Importance: MemoryType.
//   - Priority: MemoryType, salvo que la relevancia caiga por debajo de
//     minRelevance — en ese caso un item que sería Relevant baja a Optional.
//     Un item Critical por tipo NUNCA se degrada por relevancia baja: FR-005
//     lo define como "debe preservarse", no como condicional a qué tan bien
//     matcheó la búsqueda de esta tarea puntual.
func newContextCandidate(m domain.Memory, index, total int, minRelevance float32) contextCandidate {
	relevance := float32(1)
	if total > 1 {
		relevance = 1 - float32(index)/float32(total)
	}

	var importance float32
	priority := domain.PriorityOptional
	switch m.Type {
	case domain.Decision, domain.Architecture, domain.Bugfix:
		importance = 1
		priority = domain.PriorityCritical
	case domain.Pattern, domain.Discovery, domain.Learning:
		importance = 0.6
		priority = domain.PriorityRelevant
		if relevance < minRelevance {
			priority = domain.PriorityOptional
		}
	default: // Preference y cualquier tipo futuro no listado arriba.
		importance = 0.3
		priority = domain.PriorityOptional
	}

	content := m.Content
	if m.Title != "" {
		content = m.Title + ": " + m.Content
	}

	return contextCandidate{
		id:         "memory:" + strconv.FormatInt(m.ID, 10),
		content:    content,
		source:     m.Filepath,
		priority:   priority,
		relevance:  relevance,
		importance: importance,
		confidence: 1,
	}
}

// specKitCandidates convierte un domain.SpecKitFeatureContext (ya recortado
// por relevancia por SpecKitReader.Read) en candidatos del mismo pipeline
// que las memorias. Requirements/Constraints son Critical (FR-005: "explicit
// user requirements, acceptance criteria, hard constraints"); Decisions/
// TaskDependencies son Relevant (spec.md: "architecture decisions, previous
// implementation details").
func specKitCandidates(ctx domain.SpecKitFeatureContext) []contextCandidate {
	source := "specs/" + ctx.Feature
	var out []contextCandidate

	add := func(kind string, lines []string, priority domain.Priority, importance float32) {
		for i, line := range lines {
			out = append(out, contextCandidate{
				id:         "speckit:" + ctx.Feature + "/" + kind + "/" + strconv.Itoa(i),
				content:    line,
				source:     source,
				priority:   priority,
				relevance:  1,
				importance: importance,
				confidence: 1,
			})
		}
	}

	add("requirements", ctx.Requirements, domain.PriorityCritical, 1)
	add("constraints", ctx.Constraints, domain.PriorityCritical, 1)
	add("decisions", ctx.Decisions, domain.PriorityRelevant, 0.6)
	add("tasks", ctx.TaskDependencies, domain.PriorityRelevant, 0.5)

	return out
}

// boostHotspotCandidates sube la prioridad de items[i] de PriorityOptional a
// PriorityRelevant cuando items[i].source coincide con un hotspot vigente
// según CUALQUIERA de los proveedores dados — mismo criterio que ya usa
// build_context.go en "Memoria conectada a código activo" (itera todos los
// proveedores, no solo el primero disponible; research.md §2, feature 018).
// Nunca toca PriorityCritical y nunca baja una prioridad — la señal del
// grafo de código solo puede ayudar (FR-004, research.md §6). Muta items
// in-place, mismo patrón que el resto del ensamblado de candidatos en
// BuildContextPack.
// codeGraphArchitectureCandidate arma, como máximo, un contextCandidate con
// el resumen compacto de arquitectura (formatCodeArchitecture,
// build_context.go) del primer proveedor con snapshot disponible
// (FirstAvailable — research.md §2, no itera todos como el boost de
// hotspots, para no duplicar contenido dentro de un presupuesto acotado).
// Segundo valor false si no hay ningún proveedor disponible: cero impacto,
// no es un error (feature 018, Historia 2).
func codeGraphArchitectureCandidate(providers []ports.CodeGraphProvider) (contextCandidate, bool) {
	cp := FirstAvailable(providers)
	if cp == nil {
		return contextCandidate{}, false
	}
	snap := cp.Snapshot()
	if !snap.Available || snap.Architecture == nil {
		return contextCandidate{}, false
	}
	return contextCandidate{
		id:         "codegraph:architecture",
		content:    formatCodeArchitecture(snap),
		source:     snap.Provider,
		priority:   domain.PriorityOptional,
		relevance:  1,
		importance: 0.4,
		confidence: 1,
	}, true
}

func boostHotspotCandidates(items []contextCandidate, providers []ports.CodeGraphProvider) {
	for i := range items {
		if items[i].source == "" || items[i].priority != domain.PriorityOptional {
			continue
		}
		for _, cp := range providers {
			if cp == nil {
				continue
			}
			if ann, ok := cp.ImpactFor(items[i].source); ok && ann.Hotspot {
				items[i].priority = domain.PriorityRelevant
				break
			}
		}
	}
}
