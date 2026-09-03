package usecases

import (
	"fmt"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// --- Octopus AAR: enrutar una unidad de trabajo (feature 027) ---
//
// El reparto de responsabilidades entre esta capa y el dominio es deliberado y
// no es negociable: AQUÍ se MIDE, en el dominio se DECIDE.
//
// `ports.TokenCounter` vive en `application/ports`, que importa `mem/domain`.
// Si la política contara tokens habría un ciclo de imports, y además dejaría de
// ser una función pura verificable con tablas de casos. Por eso el caso de uso
// convierte texto en cifras y le entrega a `domain.RouteTask` una entrada que ya
// no contiene nada que medir.

// RouteTaskRequest es la petición de enrutamiento de UNA unidad de trabajo.
type RouteTaskRequest struct {
	Unit domain.WorkUnit
	// ContextMaterial es el texto que la unidad necesitaría como contexto. Se
	// mide aquí; si viene vacío se respeta lo que el llamador ya puso en
	// Unit.ContextNeed.EstimatedTokens.
	ContextMaterial string
	// ContractMaterial es el texto del contrato de ejecución, si ya se armó.
	ContractMaterial string
	// InlineMaterial es el contexto que el trabajo consumiría hecho inline. Sin
	// él, la regla 8 de la política no aplica.
	InlineMaterial string
	Resolved       map[string]bool
	Capabilities   domain.RuntimeCapabilities
	Budget         domain.Budget
	Policy         domain.PolicyOverrides
	Evidence       *domain.ClassEvidence
	Depth          int
	DuplicateWork  bool
	DelegatedSoFar int
	// Project habilita la consulta de evidencia histórica. Vacío = arranque en
	// frío, que es una decisión válida, no una carencia.
	Project string
}

// RouteTaskUseCase enruta una unidad de trabajo.
type RouteTaskUseCase struct {
	counter ports.TokenCounter
	repo    ports.OctopusRepository
	memRepo ports.MemoryRepository
}

// NewRouteTaskUseCase admite un contador nil: sin él, la estimación usa lo que
// el llamador ya haya medido. Medir mejora la estimación, no la habilita
// (INV-AAR-016).
func NewRouteTaskUseCase(counter ports.TokenCounter) *RouteTaskUseCase {
	return &RouteTaskUseCase{counter: counter}
}

// WithEvidence conecta el historial del proyecto para que alimente el desempate.
// Es OPCIONAL a propósito: sin repositorio, la política decide igual con sus
// reglas deterministas (FR-048, AC-015). El aprendizaje es una optimización, no
// un requisito para funcionar.
func (uc *RouteTaskUseCase) WithEvidence(repo ports.OctopusRepository) *RouteTaskUseCase {
	uc.repo = repo
	return uc
}

// WithMemoryRepository habilita detectar trabajo ya cubierto por memoria
// curada. Es opcional: sin repositorio la decisión sigue siendo válida, pero
// con él la regla 5 deja de depender de que cada adaptador la inyecte a mano.
func (uc *RouteTaskUseCase) WithMemoryRepository(repo ports.MemoryRepository) *RouteTaskUseCase {
	uc.memRepo = repo
	return uc
}

// evidencia busca el historial de la clase, o nil si no hay repositorio ni datos.
func (uc *RouteTaskUseCase) evidencia(project string, class domain.TaskClass) *domain.ClassEvidence {
	if uc.repo == nil || project == "" || class == "" {
		return nil
	}
	return uc.repo.Evidence(project)[class]
}

func (uc *RouteTaskUseCase) medir(texto string) int {
	if uc.counter == nil || texto == "" {
		return 0
	}
	return uc.counter.Count(texto)
}

// Route valida en el borde y devuelve la decisión.
func (uc *RouteTaskUseCase) Route(req RouteTaskRequest) (domain.RouteDecision, error) {
	if err := req.Unit.Validate(); err != nil {
		return domain.RouteDecision{}, fmt.Errorf("enrutar unidad: %w", err)
	}
	if !req.DuplicateWork {
		req.DuplicateWork = uc.trabajoCubierto(req.Project, req.Unit.Objective)
	}
	return domain.RouteTask(uc.buildInput(req)), nil
}

// trabajoCubierto exige que al menos el 80 % de los términos significativos del
// objetivo esté presente en una memoria encontrada. No usa Jaccard: una memoria
// extensa que contiene el objetivo completo no debe perder por su longitud.
func (uc *RouteTaskUseCase) trabajoCubierto(project, objetivo string) bool {
	if uc.memRepo == nil || project == "" || objetivo == "" {
		return false
	}
	mems, err := uc.memRepo.Search(project, objetivo, 10)
	if err != nil {
		return false
	}
	objetivoTokens := tokenize(objetivo)
	for _, m := range mems {
		if m.Type != domain.Checkpoint && tokenCoverage(objetivoTokens, tokenize(m.Title+" "+m.Content)) >= 0.80 {
			return true
		}
	}
	return false
}

// buildInput traduce la petición a la entrada del dominio, midiendo lo que haya
// que medir. Es el único punto donde texto se convierte en cifras.
func (uc *RouteTaskUseCase) buildInput(req RouteTaskRequest) domain.RouteInput {
	unit := req.Unit

	// Solo se mide lo que el llamador no trajo ya medido: una cifra explícita
	// del llamador siempre gana a nuestra aproximación.
	if unit.ContextNeed.EstimatedTokens == 0 {
		unit.ContextNeed.EstimatedTokens = uc.medir(req.ContextMaterial)
	}

	// Mismo criterio que ContextNeed.EstimatedTokens arriba: una cifra que el
	// llamador ya trae puesta en el propio WorkUnit (InlineCostTokens/
	// ContractTokens, ver domain/octopus_workunit.go) SIEMPRE gana y nunca se
	// descarta — antes de esta corrección, esta función recalculaba ambas
	// cifras incondicionalmente desde el material de texto, tirando cualquier
	// valor ya medido que el llamador hubiera puesto directamente en la
	// unidad (hallazgo real de un re-juicio posterior, no teórico: cualquier
	// llamador que reutilice el mismo WorkUnit entre route_task y route_plan,
	// o que mida el costo inline por su cuenta, perdía esa cifra en silencio).
	contractTokens := unit.ContractTokens
	if contractTokens == 0 {
		// ContractMaterial es opcional para el llamador: ningún adaptador de
		// CLI/MCP lo arma hoy, y sin este fallback el componente "contrato"
		// del costo quedaba permanentemente en 0.
		contractMaterial := req.ContractMaterial
		if contractMaterial == "" {
			contractMaterial = estimarContractMaterial(unit)
		}
		contractTokens = uc.medir(contractMaterial)
	}

	// InlineMaterial (y por tanto InlineCostTokens/regla 8) NO tiene fallback
	// automático a partir de OTRO material a propósito — ver nota en
	// RouteTaskRequest.InlineMaterial: reusar ContextMaterial aquí
	// demostrablemente inutiliza la delegación entera (verificado
	// empíricamente antes de este commit), porque costo.Total() de delegar
	// SIEMPRE incluye ContextTokens como componente propio más ~650 tokens de
	// sobrecosto fijo: cualquier InlineCostTokens que sea un subconjunto de
	// los componentes de Total() pierde la comparación de la regla 8 siempre,
	// no según el caso. Sigue siendo 0 ("desconocido") salvo que el llamador
	// declare explícitamente un costo inline — ya sea por InlineMaterial, ya
	// sea puesto directamente en unit.InlineCostTokens — pendiente de
	// decisión de diseño sobre CÓMO medirlo, ver tasks/lessons.md.
	inlineTokens := unit.InlineCostTokens
	if inlineTokens == 0 {
		inlineTokens = uc.medir(req.InlineMaterial)
	}

	evidencia := req.Evidence
	if evidencia == nil {
		evidencia = uc.evidencia(req.Project, unit.Class)
	}

	return domain.RouteInput{
		Unit:             unit,
		Resolved:         req.Resolved,
		Capabilities:     req.Capabilities,
		Budget:           req.Budget,
		Policy:           req.Policy,
		Evidence:         evidencia,
		Depth:            req.Depth,
		DuplicateWork:    req.DuplicateWork,
		DelegatedSoFar:   req.DelegatedSoFar,
		InlineCostTokens: inlineTokens,
		ContractTokens:   contractTokens,
	}
}

// estimarContractMaterial arma un proxy liviano del contrato de ejecución para
// estimar su tamaño ANTES de decidir la ruta. El contrato real
// (domain.ExecutionContract) solo puede construirse después de la decisión —
// necesita el RouteDecision, que es justo lo que la regla 8 está calculando —
// así que este proxy no pretende ser el contrato final: es una estimación
// honesta con lo único que ya se conoce antes de decidir (INV-AAR-016 acepta
// cifras groseras, no exige conteo exacto).
func estimarContractMaterial(u domain.WorkUnit) string {
	var b strings.Builder
	b.WriteString(u.Objective)
	for _, f := range u.Scope.Files {
		b.WriteString(f)
	}
	for _, r := range u.ExpectedOutput.Required {
		b.WriteString(r)
	}
	return b.String()
}
