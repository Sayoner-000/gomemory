package usecases

import (
	"fmt"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// --- Octopus AAR: enrutar un grafo de tareas (feature 027) ---

// RoutePlanRequest es la petición de enrutamiento de un plan completo.
type RoutePlanRequest struct {
	PlanID       string
	Units        []domain.WorkUnit
	Resolved     map[string]bool
	Capabilities domain.RuntimeCapabilities
	Budget       domain.Budget
	Policy       domain.PolicyOverrides
	Evidence     map[domain.TaskClass]*domain.ClassEvidence
	Depth        int
	// Root habilita la lectura del grafo desde Spec Kit cuando Units viene
	// vacío. Sin él, o sin funcionalidad activa, el llamador debe aportar Units.
	Root string
	// Project habilita la consulta de evidencia histórica.
	Project string
	// ContextMaterial permite a los adaptadores que sí pueden leer el alcance
	// entregar la medición real por tarea; la capa de aplicación no hace I/O.
	ContextMaterial map[string]string
}

// RoutePlanUseCase enruta un grafo de tareas.
type RoutePlanUseCase struct {
	counter  ports.TokenCounter
	specKit  ports.SpecKitReader
	routeOne *RouteTaskUseCase
	repo     ports.OctopusRepository
}

// WithEvidence conecta el historial del proyecto. Opcional: sin él, el plan se
// enruta igual con las reglas deterministas (AC-015).
func (uc *RoutePlanUseCase) WithEvidence(repo ports.OctopusRepository) *RoutePlanUseCase {
	uc.repo = repo
	return uc
}

func (uc *RoutePlanUseCase) WithMemoryRepository(repo ports.MemoryRepository) *RoutePlanUseCase {
	uc.routeOne.WithMemoryRepository(repo)
	return uc
}

// NewRoutePlanUseCase admite counter y specKit nil. Sin specKit, el grafo tiene
// que venir en la petición; sin counter, se usan las cifras que traiga el
// llamador. Ninguna de las dos ausencias impide enrutar (INV-AAR-016).
func NewRoutePlanUseCase(counter ports.TokenCounter, specKit ports.SpecKitReader) *RoutePlanUseCase {
	return &RoutePlanUseCase{
		counter:  counter,
		specKit:  specKit,
		routeOne: NewRouteTaskUseCase(counter),
	}
}

// Route enruta el plan. Devuelve error solo por entrada inválida.
func (uc *RoutePlanUseCase) Route(req RoutePlanRequest) (domain.RoutingPlan, error) {
	unidades := req.Units
	if len(unidades) == 0 {
		desdeSpecKit, err := uc.unidadesDesdeSpecKit(req.Root)
		if err != nil {
			return domain.RoutingPlan{}, err
		}
		unidades = desdeSpecKit
	}
	if len(unidades) == 0 {
		return domain.RoutingPlan{}, fmt.Errorf(
			"%w: no hay unidades de trabajo que enrutar (aporta el grafo o abre una funcionalidad de Spec Kit)",
			domain.ErrValidation)
	}

	// Medir aquí lo que el llamador no trajo medido: el dominio no mide texto.
	duplicadas := make(map[string]bool)
	for i := range unidades {
		material := req.ContextMaterial[unidades[i].ID]
		if material == "" {
			material = unidades[i].Objective
		}
		if unidades[i].ContextNeed.EstimatedTokens == 0 && uc.counter != nil {
			unidades[i].ContextNeed.EstimatedTokens = uc.counter.Count(material)
		}
		// InlineCostTokens NO se deriva de "material" aquí — ver la nota extensa
		// en RouteTaskUseCase.buildInput (octopus_route_task.go): reusar el
		// mismo texto que ya cuenta como componente de costo.Total() hace que la
		// regla 8 rechace TODA delegación, siempre, verificado empíricamente.
		// Sigue en 0 hasta que exista una medición honesta y distinta.
		if unidades[i].ContractTokens == 0 && uc.counter != nil {
			unidades[i].ContractTokens = uc.counter.Count(estimarContractMaterial(unidades[i]))
		}
		duplicadas[unidades[i].ID] = uc.routeOne.trabajoCubierto(req.Project, unidades[i].Objective)
	}

	planID := req.PlanID
	if planID == "" {
		planID = "plan"
	}

	evidencia := req.Evidence
	if evidencia == nil && uc.repo != nil && req.Project != "" {
		evidencia = uc.repo.Evidence(req.Project)
	}

	return domain.RoutePlan(domain.PlanInput{
		PlanID:        planID,
		Units:         unidades,
		Resolved:      req.Resolved,
		Capabilities:  req.Capabilities,
		Budget:        req.Budget,
		Policy:        req.Policy,
		Evidence:      evidencia,
		Depth:         req.Depth,
		DuplicateWork: duplicadas,
	})
}

// unidadesDesdeSpecKit lee el grafo de tareas de la funcionalidad activa. Degrada
// en silencio: sin lector, sin funcionalidad activa o sin dependencias
// declaradas, devuelve vacío sin error — Spec Kit no es una dependencia dura
// (FR-054).
func (uc *RoutePlanUseCase) unidadesDesdeSpecKit(root string) ([]domain.WorkUnit, error) {
	if uc.specKit == nil || root == "" {
		return nil, nil
	}
	feature, err := uc.specKit.ActiveFeature(root)
	if err != nil || feature == "" {
		return nil, nil
	}
	ctx, err := uc.specKit.Read(root, feature, "")
	if err != nil {
		return nil, nil
	}

	unidades := make([]domain.WorkUnit, 0, len(ctx.TaskDependencies))
	for _, linea := range ctx.TaskDependencies {
		if u, ok := parsearTareaDeSpecKit(linea); ok {
			unidades = append(unidades, u)
		}
	}
	return unidades, nil
}

// parsearTareaDeSpecKit traduce una línea de tasks.md a una unidad de trabajo.
// Formato esperado: `- [ ] T042 [P] [US3] descripción con ruta/al/archivo.go`.
//
// Es deliberadamente tolerante: una línea que no encaje se descarta sin error.
// El grafo de Spec Kit es una comodidad, no un contrato — quien necesite control
// fino entrega las unidades explícitamente.
func parsearTareaDeSpecKit(linea string) (domain.WorkUnit, bool) {
	resto := strings.TrimSpace(linea)
	for _, prefijo := range []string{"- [ ] ", "- [x] ", "- [X] "} {
		if s, ok := strings.CutPrefix(resto, prefijo); ok {
			resto = s
			break
		}
	}

	campos := strings.Fields(resto)
	if len(campos) < 2 || !esIDDeTarea(campos[0]) {
		return domain.WorkUnit{}, false
	}
	id := campos[0]

	// Las etiquetas [P] y [USn] son metadatos del formato, no parte del objetivo.
	descripcion := campos[1:]
	for len(descripcion) > 0 && strings.HasPrefix(descripcion[0], "[") {
		descripcion = descripcion[1:]
	}
	objetivo := strings.TrimSpace(strings.Join(descripcion, " "))
	if objetivo == "" {
		return domain.WorkUnit{}, false
	}

	return domain.WorkUnit{
		ID:         id,
		Objective:  objetivo,
		Scope:      domain.Scope{Files: rutasEnTexto(objetivo)},
		Complexity: domain.LevelMedium,
	}, true
}

// esIDDeTarea reconoce la forma Tnnn del formato de tasks.md.
func esIDDeTarea(s string) bool {
	if len(s) < 2 || s[0] != 'T' {
		return false
	}
	for _, r := range s[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// rutasEnTexto extrae los tokens que parecen rutas de archivo. Sirve para poblar
// el alcance sin pedirle al usuario que lo repita: las tareas del formato ya
// llevan la ruta exacta en la descripción.
func rutasEnTexto(s string) []string {
	var out []string
	for _, campo := range strings.Fields(s) {
		campo = strings.Trim(campo, "`.,;:()")
		if strings.Contains(campo, "/") && strings.Contains(campo, ".") {
			out = append(out, campo)
		}
	}
	return out
}
