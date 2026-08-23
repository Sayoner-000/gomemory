package setup

import (
	"os"
	"path/filepath"
)

// constitutionWrappers son los envoltorios nativos que permiten invocar la
// constitución con el atajo propio de cada agente (feature 021, FR-027).
//
// Mismo patrón y mismas rutas que atomicPlanWrappers: cada agente los descubre
// solo, sin registro adicional.
var constitutionWrappers = []atomicPlanWrapper{
	{
		path: []string{".claude", "skills", "constitution", "SKILL.md"},
		frontmatter: "---\n" +
			"name: constitution\n" +
			"description: Aplica la constitución técnica vigente del proyecto, servida desde la memoria de gomemory. Úsala antes de decidir stack, capas o convenciones.\n" +
			"---\n\n",
	},
	{
		path: []string{".opencode", "commands", "constitution.md"},
		frontmatter: "---\n" +
			"description: Aplicar la constitución técnica vigente del proyecto (desde la memoria)\n" +
			"---\n\n",
	},
}

// constitutionWrapperBody es el cuerpo compartido de los envoltorios.
//
// NO contiene el texto de la constitución, y esa es la decisión de diseño que
// importa: el paso que esta feature elimina copiaba 635 líneas a la raíz del
// proyecto, donde quedaban congeladas y divergían de la fuente en cuanto
// alguien editaba una de las dos. El envoltorio resuelve el contenido EN EL
// MOMENTO de la invocación, así que siempre refleja lo que el equipo tiene
// guardado ahora.
const constitutionWrapperBody = `# Constitución del proyecto

La constitución vigente vive en la memoria de gomemory, no en un archivo del
repositorio. Recupérala en el momento de usarla — nunca la reproduzcas de
memoria ni desde una copia antigua.

## Cómo obtenerla

1. Ejecuta ` + "`./mem constitution`" + ` (o ` + "`mem constitution`" + ` si el binario está en PATH).
   Si el servidor MCP ` + "`gomemory`" + ` está conectado, puedes usar en su lugar
   ` + "`search_memories`" + ` con "constitución" y luego ` + "`get_memory`" + ` sobre el resultado.
2. Aplica lo que devuelva a la tarea en curso: stack, capas, convenciones,
   prohibiciones.

Si el proyecto usa spec-kit y quieres reflejarla en su archivo:
` + "`./mem constitution --sync`" + ` escribe ` + "`.specify/memory/constitution.md`" + `.
En un proyecto sin spec-kit no crea nada.

## Si el equipo quiere cambiarla

La constitución es del equipo, no de la herramienta. El contenido que trae
gomemory es solo un punto de partida:

    ./mem docs export constitution -o constitucion.md   # exportar
    # editar constitucion.md
    ./mem docs import constitution constitucion.md      # aplicar

` + "`./mem docs list`" + ` muestra si el contenido actual es el que trae la herramienta
o uno propio, y ` + "`./mem docs reset constitution`" + ` vuelve al de por defecto.

## Referencia de las reglas de trabajo

La constitución dice CÓMO escribir código. Las reglas de trabajo —cuándo
planificar, cómo verificar, cómo tratar un bug— son otro documento, y llegan
solas en ` + "`get_context()`" + ` al inicio de cada sesión. No las confundas: aplicar un
requisito de la constitución que rompa el flujo real de trabajo es justamente lo
que las reglas de trabajo prohíben.
`

// InstallConstitutionWrappers escribe los envoltorios bajo root.
//
// Capa OPCIONAL, igual que InstallAtomicPlanWrappers: la funcionalidad opera sin
// ellos —siempre queda `mem constitution`— y aportan la ergonomía del atajo
// nativo. Idempotente: solo reescribe un archivo si su contenido difiere.
func InstallConstitutionWrappers(root string) error {
	for _, w := range constitutionWrappers {
		dest := filepath.Join(append([]string{root}, w.path...)...)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}

		content := []byte(w.frontmatter + constitutionWrapperBody)
		if previo, err := os.ReadFile(dest); err == nil && string(previo) == string(content) {
			continue
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}
