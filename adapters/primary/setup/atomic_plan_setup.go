package setup

import (
	"os"
	"path/filepath"
	"strings"
)

// atomicPlanWrapper describe un envoltorio nativo del método de planificación
// atómica para un agente concreto: dónde vive y qué encabezado necesita para que
// ese agente lo descubra.
type atomicPlanWrapper struct {
	path        []string // relativo a la raíz donde se instala
	frontmatter string
}

// atomicPlanWrappers son los envoltorios nativos que gomemory genera (feature
// 013, FR-028). Son una capa OPCIONAL: la funcionalidad opera igual sin ellos,
// porque el disparador viaja en el bloque de protocolo que todos los agentes
// leen. Aportan ergonomía —la persona puede invocarlos a mano— y aprovechan la
// carga diferida nativa de cada agente.
var atomicPlanWrappers = []atomicPlanWrapper{
	{
		path: []string{".claude", "skills", "atomic-decomposition", "SKILL.md"},
		frontmatter: "---\n" +
			"name: atomic-decomposition\n" +
			"description: Descompone una solicitud grande, multi-paso o de resultado incierto en tareas atómicas verificables antes de planificar. Úsala en modo plan.\n" +
			"---\n\n",
	},
	{
		path: []string{".opencode", "commands", "atomic-decomposition.md"},
		frontmatter: "---\n" +
			"description: Descompone el objetivo en tareas atómicas verificables antes de planificar\n" +
			"---\n\n",
	},
}

// InstallAtomicPlanWrappers escribe los envoltorios nativos del método bajo
// root, generándolos SIEMPRE desde el método recibido — que es el mismo texto
// embebido que devuelve get_plan_context. Esa es la mitigación del riesgo de que
// método y envoltorios diverjan: no hay copia editable, se regeneran en cada
// instalación.
//
// Idempotente: solo reescribe un archivo si su contenido difiere, mismo criterio
// que InstallPlugin ya aplica en producción.
//
// Un método vacío (p. ej. si la plantilla embebida no pudo cargarse) es un
// no-op silencioso: es preferible no dejar envoltorio a dejar uno vacío que el
// agente tomaría por válido.
func InstallAtomicPlanWrappers(root, method string) error {
	if strings.TrimSpace(method) == "" {
		return nil
	}

	for _, w := range atomicPlanWrappers {
		dest := filepath.Join(append([]string{root}, w.path...)...)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}

		content := []byte(w.frontmatter + strings.TrimSpace(method) + "\n")
		if previo, err := os.ReadFile(dest); err == nil && string(previo) == string(content) {
			continue
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// GeneratedWrapperPaths devuelve las rutas relativas de todos los envoltorios
// nativos que la instalación genera, por segmentos.
//
// Existe para que la desinstalación retire exactamente lo que la instalación
// escribe, derivándolo de las mismas tablas en vez de una lista paralela. Sin
// esto, los cuatro envoltorios sobrevivían a toda desinstalación: son archivos
// generados por completo, en directorios que también contienen artefactos de
// otras herramientas y de la persona, así que no se pueden retirar borrando el
// directorio.
func GeneratedWrapperPaths() [][]string {
	var out [][]string
	for _, w := range atomicPlanWrappers {
		out = append(out, w.path)
	}
	for _, w := range constitutionWrappers {
		out = append(out, w.path)
	}
	return out
}
