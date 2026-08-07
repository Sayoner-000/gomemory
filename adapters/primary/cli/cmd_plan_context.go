package cli

import (
	"flag"
	"os"

	"mem/application/usecases"
)

// planMethod es el método de descomposición atómica embebido en el binario
// (feature 013). Lo inyecta el composition root vía SetPlanMethod, igual que
// TemplatesFS: la capa de aplicación no debe leer del sistema de archivos, y
// mantener una sola copia en el binario evita que método y envoltorios
// distribuidos diverjan. Vacío (p. ej. en algunos tests) degrada con gracia:
// se emite solo el contexto.
var planMethod string

// SetPlanMethod inyecta el método embebido. Lo llama el composition root al
// arrancar.
func SetPlanMethod(method string) { planMethod = method }

// PlanMethod devuelve el método inyectado. Lo usan las rutas que necesitan
// distribuirlo (envoltorios nativos por agente) para no reabrir la plantilla.
func PlanMethod() string { return planMethod }

// CmdPlanContext implementa `mem plan-context`: entrega el método de
// descomposición atómica junto con el contexto histórico del proyecto, en una
// sola llamada, para que el agente lo invoque al entrar en modo plan.
//
// Es la vía de línea de comandos del mismo contrato que expone la tool MCP
// get_plan_context — ambas devuelven el mismo documento; solo cambia el
// transporte. Existe para los agentes que no tienen el servidor MCP conectado.
//
// El código de salida es SIEMPRE 0: ninguna condición puede interrumpir el modo
// plan del agente (feature 013, FR-034).
func CmdPlanContext(deps *Deps, args []string) {
	fs := flag.NewFlagSet("plan-context", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return
	}

	out, err := buildPlanContextDoc(deps)
	if err != nil {
		// Inalcanzable con la implementación actual (Build no propaga errores
		// ambientales), pero si alguna vez lo hiciera, salir en silencio sigue
		// siendo preferible a romper el modo plan.
		return
	}
	if out == "" {
		return
	}
	os.Stdout.WriteString(out + "\n")
}

// buildPlanContextDoc arma el documento aplicando el gate de configuración.
// Compartida por el comando y por la tool MCP para que ambos no puedan divergir.
func buildPlanContextDoc(deps *Deps) (string, error) {
	disabled := deps.SettingsRepo.Read(deps.Root).AtomicPlanDisabled
	return usecases.NewPlanContext(planMethod, deps.ContextBuilder).Build(disabled)
}
