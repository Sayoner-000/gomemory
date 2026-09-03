package domain

import (
	"fmt"
	"strings"
)

// --- Octopus AAR (feature 027): la unidad de trabajo que se enruta ---
//
// Todo este archivo es dominio puro: sin I/O, sin reloj, sin aleatoriedad y sin
// imports de `application/ports`. La razón no es purismo — es que la política de
// enrutamiento debe ser verificable con tablas de casos y reproducible ante
// entradas idénticas (FR-009, FR-010, SC-006), y una función que mide texto o
// consulta el entorno no puede garantizar ninguna de las dos cosas.
//
// En concreto: el dominio NO cuenta tokens. `ports.TokenCounter` vive en una
// capa que importa `mem/domain`, así que invocarlo desde aquí crearía un ciclo.
// Quien enruta mide primero (en el caso de uso) y deposita las cifras ya
// calculadas en ContextNeed y OutputSpec; aquí solo se hace aritmética.

// Level es la escala común de complejidad y de riesgo de una unidad de trabajo.
type Level int

const (
	LevelTrivial Level = iota
	LevelLow
	LevelMedium
	LevelHigh
)

func (l Level) String() string {
	switch l {
	case LevelTrivial:
		return "trivial"
	case LevelLow:
		return "low"
	case LevelMedium:
		return "medium"
	case LevelHigh:
		return "high"
	default:
		return "unknown"
	}
}

// ParseLevel traduce la forma textual usada por el CLI y las tools MCP. Un valor
// desconocido cae en LevelMedium: no es un error de entrada, es una señal pobre,
// y la política ya sabe decidir con señales pobres.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trivial":
		return LevelTrivial
	case "low", "bajo":
		return LevelLow
	case "high", "alto":
		return LevelHigh
	default:
		return LevelMedium
	}
}

// TaskClass clasifica la unidad de trabajo. El catálogo es EXTENSIBLE por
// diseño (FR-012): un valor desconocido es válido y se trata como sin clasificar.
// Enrutar nunca depende exclusivamente de la clase — es una señal más, no la
// decisión.
type TaskClass string

const (
	ClassUnknown               TaskClass = ""
	ClassTrivial               TaskClass = "trivial"
	ClassLocalChange           TaskClass = "local-change"
	ClassImplementation        TaskClass = "implementation"
	ClassInvestigation         TaskClass = "investigation"
	ClassRepositoryExploration TaskClass = "repository-exploration"
	ClassResearch              TaskClass = "research"
	ClassTesting               TaskClass = "testing"
	ClassDocumentation         TaskClass = "documentation"
	ClassArchitecture          TaskClass = "architecture"
	ClassValidation            TaskClass = "validation"
	ClassReview                TaskClass = "review"
	ClassMigration             TaskClass = "migration"
	ClassIntegration           TaskClass = "integration"
)

// knownClasses es el catálogo conocido. Estar fuera de él NO invalida nada.
var knownClasses = map[TaskClass]bool{
	ClassTrivial: true, ClassLocalChange: true, ClassImplementation: true,
	ClassInvestigation: true, ClassRepositoryExploration: true, ClassResearch: true,
	ClassTesting: true, ClassDocumentation: true, ClassArchitecture: true,
	ClassValidation: true, ClassReview: true, ClassMigration: true,
	ClassIntegration: true,
}

func (c TaskClass) Known() bool { return knownClasses[c] }

// Aislable indica si la clase suele admitir un contexto fuertemente acotado.
// Es una señal de entrada al desempate, nunca una decisión por sí sola.
func (c TaskClass) Aislable() bool {
	switch c {
	case ClassInvestigation, ClassRepositoryExploration, ClassResearch,
		ClassDocumentation, ClassTesting, ClassReview, ClassValidation:
		return true
	default:
		return false
	}
}

// Scope es el alcance de una unidad: qué artefactos toca y si solo los lee.
type Scope struct {
	Files    []string
	ReadOnly bool
}

// WritesIntersect responde si dos alcances compiten por escribir el mismo
// artefacto. Dos unidades en esa situación no pueden compartir grupo paralelo
// aunque no exista dependencia declarada entre ellas: el conflicto es de estado,
// no de orden (FR-015). Dos lecturas nunca compiten.
func (s Scope) WritesIntersect(other Scope) bool {
	if s.ReadOnly && other.ReadOnly {
		return false
	}
	// Si una de las dos solo lee, no hay dos escrituras compitiendo.
	if s.ReadOnly || other.ReadOnly {
		return false
	}
	if len(s.Files) == 0 || len(other.Files) == 0 {
		return false
	}
	set := make(map[string]bool, len(s.Files))
	for _, f := range s.Files {
		set[f] = true
	}
	for _, f := range other.Files {
		if set[f] {
			return true
		}
	}
	return false
}

// ContextNeed son las cifras de contexto YA MEDIDAS por quien enruta. El dominio
// no las calcula: las recibe (ver nota de cabecera del archivo).
type ContextNeed struct {
	// EstimatedTokens es el contexto que la unidad necesitaría para completarse.
	EstimatedTokens int
	// NearlyFullParent marca que la unidad requiere prácticamente todo el
	// contexto del agente principal: delegarla duplicaría ese contexto en vez
	// de aislarlo, así que la política la mantiene inline (FR-015).
	NearlyFullParent bool
}

// OutputSpec describe la forma esperada del resultado de una unidad.
type OutputSpec struct {
	MaxTokens int
	Required  []string
	Format    string
}

// WorkUnit es una tarea acotada que puede ejecutarse inline o delegarse.
type WorkUnit struct {
	ID             string
	Objective      string
	Class          TaskClass
	Dependencies   []string
	Scope          Scope
	Complexity     Level
	Risk           Level
	ContextNeed    ContextNeed
	ExpectedOutput OutputSpec
	// CriticalPath marca las unidades de la ruta crítica del plan (FR-019).
	CriticalPath bool
	// Optional marca las unidades sin las que el plan puede completarse. Es lo
	// que permite proteger la reserva de validación solo frente a delegaciones
	// prescindibles (INV-AAR-006).
	Optional bool
	// InlineCostTokens y ContractTokens son cifras YA MEDIDAS por quien enruta
	// (mismo criterio que ContextNeed.EstimatedTokens): el dominio las deposita
	// en RouteInput sin volver a medir nada. 0 = desconocido.
	InlineCostTokens int
	ContractTokens   int
}

// Validate comprueba en el borde lo que la política da por hecho. Un objetivo
// vacío hace la unidad inenrutable: INV-AAR-003 exige objetivo acotado y una
// cadena vacía no puede acotarse.
func (u WorkUnit) Validate() error {
	if strings.TrimSpace(u.ID) == "" {
		return fmt.Errorf("%w: la unidad de trabajo necesita identificador", ErrValidation)
	}
	if strings.ContainsAny(u.ID, " \t\n") {
		return fmt.Errorf("%w: el identificador %q no puede contener espacios", ErrValidation, u.ID)
	}
	if strings.TrimSpace(u.Objective) == "" {
		return fmt.Errorf("%w: la unidad %s necesita un objetivo", ErrValidation, u.ID)
	}
	return nil
}
