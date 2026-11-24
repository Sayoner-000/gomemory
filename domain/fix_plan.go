package domain

// Este archivo modela el plan que ejecuta `mem doctor --fix` (HCD-063).
//
// El doctor ya sabía con qué comando se restablece cada problema
// (CorreccionPara, remedy.go); el plan solo los reúne, quita los repetidos y
// los ordena para pedir confirmación con el alcance correcto.

// AdvertenciaAmbitoUsuario es el aviso que acompaña a toda acción que escribe
// fuera del proyecto. Coincide con el de CorreccionPara.
const AdvertenciaAmbitoUsuario = "afecta a todos los proyectos de esta máquina, no solo a este"

// FixAction es una acción de reparación propuesta por el doctor.
type FixAction struct {
	// Comando es lo que se muestra y, en las acciones por subproceso, lo que
	// se ejecuta. También es la clave para agrupar duplicados.
	Comando string
	// Ambito decide la confirmación: proyecto o usuario (toda la máquina).
	Ambito AgentScope
	// Advertencia describe el alcance cuando sale del proyecto.
	Advertencia string
	// Motivos son los problemas que la acción corrige, para mostrarlos.
	Motivos []string
}

// PlanDeReparacion agrupa las acciones por comando (conservando todos los
// motivos), pone las de proyecto antes que las de usuario sin alterar el orden
// relativo de cada grupo y garantiza la advertencia en las de usuario.
func PlanDeReparacion(acciones []FixAction) []FixAction {
	indice := map[string]int{}
	var unicas []FixAction
	for _, a := range acciones {
		if i, ok := indice[a.Comando]; ok {
			unicas[i].Motivos = append(unicas[i].Motivos, a.Motivos...)
			continue
		}
		a.Motivos = append([]string(nil), a.Motivos...)
		indice[a.Comando] = len(unicas)
		unicas = append(unicas, a)
	}

	plan := make([]FixAction, 0, len(unicas))
	for _, a := range unicas {
		if a.Ambito != ScopeUser {
			a.Advertencia = ""
			plan = append(plan, a)
		}
	}
	for _, a := range unicas {
		if a.Ambito == ScopeUser {
			if a.Advertencia == "" {
				a.Advertencia = AdvertenciaAmbitoUsuario
			}
			plan = append(plan, a)
		}
	}
	return plan
}
