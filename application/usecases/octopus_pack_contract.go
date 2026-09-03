package usecases

import (
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

// --- Octopus AAR: paquete de contexto y contrato de lo delegado (feature 027) ---
//
// Aquí NO se escribe un seleccionador de contexto nuevo. BuildContextPack
// (feature 015) ya resuelve exactamente este problema: prioriza, filtra por
// relevancia, comprime lo no crítico, deduplica y nunca excede el presupuesto.
// Octopus solo aporta dos cosas: el objetivo de la unidad como consulta, y su
// presupuesto de contexto como techo. Duplicar esa lógica la haría divergir.

// DefaultDelegationMinRelevance es el umbral de relevancia posicional que
// Octopus pasa a BuildContextPack. Sigue al de la feature 015.
const DefaultDelegationMinRelevance = 0.65

// minSolapamientoDelegado es el solapamiento léxico mínimo entre el objetivo de
// la unidad y un item para que ese item viaje al subagente.
//
// Hace falta un segundo filtro, distinto del anterior, porque la "relevancia"
// de BuildContextPack es POSICIONAL: vale 1 - índice/total sobre los resultados
// de búsqueda. Cuando la búsqueda devuelve pocas filas —o cae al listado
// completo, que es lo que hace con consultas sin coincidencias fuertes— el
// segundo resultado puntúa 0,75 sea lo que sea. Con cuatro memorias, una
// decisión sobre la estrategia de releases entraba con nota alta en el paquete
// de una investigación sobre expiración de memorias.
//
// El umbral es bajo a propósito: descarta lo ajeno sin exigir que el objetivo
// repita el vocabulario exacto de la memoria.
const minSolapamientoDelegado = 0.05

// DelegationPackage es lo que se entrega para ejecutar una unidad delegada.
type DelegationPackage struct {
	Contract domain.ExecutionContract
	Pack     domain.ContextPack
}

// PackContractRequest pide el paquete de una unidad ya enrutada como delegada.
type PackContractRequest struct {
	Unit     domain.WorkUnit
	Decision domain.RouteDecision
	Project  string
	Root     string
	// ParentPermissions son los permisos del flujo principal. El contrato nunca
	// puede excederlos (INV-AAR-014).
	ParentPermissions domain.Permissions
	MaxDepth          int
	IncludeSpecKit    bool
	MinRelevance      float32
	MaxItems          int
	// ExtraContextTokens amplía el presupuesto de contexto en una sola ocasión,
	// tras un INSUFFICIENT_CONTEXT (FR-042). Cero en el caso normal.
	ExtraContextTokens int
	// MinRelevanceExplicita distingue "no opiné" de "quiero 0". Sin ella, un
	// MinRelevance en cero sería indistinguible del valor por defecto y no
	// habría forma de pedir deliberadamente un paquete sin filtrar.
	MinRelevanceExplicita bool
}

// PackContractUseCase arma paquete y contrato.
type PackContractUseCase struct {
	memRepo    ports.MemoryRepository
	compressor ports.Compressor
	counter    ports.TokenCounter
	specKit    ports.SpecKitReader
}

func NewPackContractUseCase(
	memRepo ports.MemoryRepository,
	compressor ports.Compressor,
	counter ports.TokenCounter,
	specKit ports.SpecKitReader,
) *PackContractUseCase {
	return &PackContractUseCase{memRepo: memRepo, compressor: compressor, counter: counter, specKit: specKit}
}

// Build arma el contrato y el paquete de contexto de una unidad delegada.
func (uc *PackContractUseCase) Build(req PackContractRequest) (DelegationPackage, error) {
	if !req.Decision.Route.Delegada() {
		return DelegationPackage{}, fmt.Errorf(
			"%w: la unidad %s no está enrutada como delegada (%s)",
			domain.ErrValidation, req.Unit.ID, req.Decision.Route)
	}

	maxDepth := req.MaxDepth
	if maxDepth <= 0 {
		maxDepth = domain.DefaultMaxDelegationDepth
	}

	contrato, err := domain.NewExecutionContract(req.Unit, req.Decision, req.ParentPermissions, maxDepth)
	if err != nil {
		return DelegationPackage{}, err
	}
	presupuesto := contrato.ContextBudget + req.ExtraContextTokens
	contrato.ContextBudget = presupuesto

	pack, err := uc.construirPack(req, presupuesto)
	if err != nil {
		return DelegationPackage{}, err
	}

	return DelegationPackage{Contract: contrato, Pack: pack}, nil
}

// construirPack delega en BuildContextPack y redacta los secretos del material
// resultante. Sin repositorio de memorias devuelve un paquete vacío: el contrato
// por sí solo ya basta para que el subagente termine, y quedarse sin memorias no
// es motivo para no delegar.
func (uc *PackContractUseCase) construirPack(req PackContractRequest, presupuesto int) (domain.ContextPack, error) {
	if uc.memRepo == nil || req.Project == "" {
		return domain.ContextPack{Budget: presupuesto}, nil
	}

	pack, err := BuildContextPack(uc.memRepo, uc.compressor, uc.counter, uc.specKit, ContextRequest{
		// El objetivo de la unidad es la consulta: es lo que define qué es
		// relevante para ESTA unidad y no para la conversación del padre. Ahí
		// está el aislamiento de contexto (AC-006).
		Task:           req.Unit.Objective,
		Project:        req.Project,
		MaxTokens:      presupuesto,
		MinRelevance:   req.minRelevanceEfectiva(),
		MaxItems:       req.MaxItems,
		IncludeSpecKit: req.IncludeSpecKit,
		Root:           req.Root,
		Compression:    ports.CompressionStructural,
	})
	if err != nil {
		return domain.ContextPack{}, fmt.Errorf("construir contexto de %s: %w", req.Unit.ID, err)
	}

	// Delegar minimiza tokens Y exposición (FR-027, §52 de la especificación).
	// La redacción va DESPUÉS de armar el paquete y sobre cada item, porque un
	// secreto podría venir de cualquier fuente: memoria, artefacto de Spec Kit
	// o dependencia. Filtrar en el origen dejaría huecos.
	for i := range pack.Items {
		pack.Items[i].Content = domain.RedactSecrets(pack.Items[i].Content)
	}

	return excluirLoNoRelacionado(pack, req.Unit.Objective), nil
}

// excluirLoNoRelacionado descarta del paquete lo que no habla del objetivo.
//
// Hace falta porque BuildContextPack PRIORIZA por relevancia, no EXCLUYE: su
// MinRelevance degrada un candidato a PriorityOptional, y los tipos Decision,
// Architecture y Bugfix entran SIEMPRE como PriorityCritical sin mirar la
// relevancia. Ese comportamiento es correcto para get_context —donde el techo es
// el presupuesto y más contexto del proyecto ayuda— pero es justo lo contrario
// de lo que necesita una unidad delegada.
//
// La diferencia de fondo: para el agente principal, contexto de más es ruido;
// para un subagente, contexto de más ANULA la razón de haberlo creado.
//
// El criterio es solapamiento léxico con el objetivo, el mismo idioma que ya usa
// DetectDuplicateGroups, en vez del rango de búsqueda: el rango dice "resultado
// 2 de 4", no "trata de esto".
func excluirLoNoRelacionado(pack domain.ContextPack, objetivo string) domain.ContextPack {
	palabrasObjetivo := tokenize(objetivo)
	if len(palabrasObjetivo) == 0 {
		return pack
	}

	conservados := make([]domain.ContextItem, 0, len(pack.Items))
	var tokens, descartados int
	for _, item := range pack.Items {
		if tokenCoverage(palabrasObjetivo, tokenize(item.Content)) < minSolapamientoDelegado {
			descartados++
			continue
		}
		conservados = append(conservados, item)
		tokens += item.Tokens
	}

	pack.Items = conservados
	pack.TokenCount = tokens
	pack.Stats.FinalTokens = tokens
	pack.Stats.ItemsDiscarded += descartados
	pack.Stats.SavedTokens = pack.Stats.RawTokens - tokens
	return pack
}

// tokenCoverage mide qué fracción del objetivo aparece en el candidato. A
// diferencia de Jaccard, no castiga una memoria o artefacto largo por contener
// vocabulario adicional; eso es indispensable al filtrar Spec Kit y memorias.
func tokenCoverage(needles, haystack map[string]struct{}) float64 {
	if len(needles) == 0 {
		return 1
	}
	var shared int
	for token := range needles {
		if _, ok := haystack[token]; ok {
			shared++
		}
	}
	return float64(shared) / float64(len(needles))
}

// minRelevanceEfectiva aplica el umbral de aislamiento salvo que el llamador
// haya declarado explícitamente el suyo.
func (r PackContractRequest) minRelevanceEfectiva() float32 {
	if r.MinRelevanceExplicita {
		return r.MinRelevance
	}
	if r.MinRelevance > 0 {
		return r.MinRelevance
	}
	return DefaultDelegationMinRelevance
}
