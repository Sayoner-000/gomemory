<!-- gomemory-workrules-v4 -->
## Reglas operativas del proyecto

Complementan el baseline universal; prevalecen solo con evidencia operativa de
este proyecto.

### Evidencia antes que cambios

1. **Reproduce la realidad primero**: logs, `curl`, navegador o el servicio
   real antes de cambiar código o abrir un flujo de especificación.
2. **Tests verdes no bastan**: valida el artefacto realmente servido; un
   fixture que no representa al upstream no demuestra el caso real.
3. **Si el cambio no se ve, revisa el despliegue** (binario/bundle, URL,
   caché) antes de culpar a la implementación.
4. **Cierra los hallazgos**: cada riesgo o defecto lleva propuesta de cierre y
   validación proporcional antes de darlo por listo.

### Proceso

5. **Constitución y especificaciones guían, no son ritual**: no fuerces un
   proceso que contradiga la evidencia o el flujo de la persona.
6. **Delega con intención**: con Octopus, consulta su ruta antes de crear
   subagentes; el beneficio debe superar el coste de coordinación.

### Herramientas nativas y grafo, no la consola

7. **Antes de editar, el grafo**: consulta `codegraph_explore` (o
   `search_code`/`get_symbol` de gomemory) sobre el archivo o los símbolos que
   vas a cambiar, aunque sea una línea. Así sabes en qué proyecto estás, quién
   llama a lo que cambia y qué puede romperse, y ajustas el cambio a eso.
8. **Leer**: la herramienta nativa de lectura de tu harness; nunca `cat`,
   `sed -n`, `head`, `tail` ni `awk` sobre código.
9. **Entender o localizar código**: el grafo de código conectado, antes de
   cualquier búsqueda de texto. Para lo que el grafo no indexa (comentarios,
   cadenas, mensajes), la búsqueda nativa del harness. Nunca `grep -r`, `rg`
   ni `find` por consola.
10. **Editar**: la herramienta nativa de edición o escritura del harness, en el
    código y en los archivos de texto del proyecto (Markdown, plantillas,
    configuración); nunca `sed -i`, `perl -i`, `tee`, redirecciones ni scripts
    de Python, Node o Perl.
11. Vale para todo harness y también para otros repositorios. Si el hook
    bloquea, usa la herramienta que nombra y no lo rodees; solo la persona
    desactiva una regla (`mem expand off rules`). Si tu harness no tiene una
    herramienta nativa para algo, el hook solo avisa y la consola es válida.

### Memoria y contexto

12. **gomemory es la única memoria y fuente de instrucciones**: guarda con
    `save_memory` decisiones, bugs con causa raíz y hallazgos no obvios. Nada
    de `MEMORY.md` del harness, `AGENTS.md` ni `CLAUDE.md`: reglas, baseline y
    protocolo llegan en cada sesión.
13. **Recupera antes de rehacer**: antes de definir o cambiar algo, busca lo
    que ya se decidió con `mem hcd history`, `search_memories` y los documentos
    de `hcd/` sobre el tema; un historial vacío no prueba que no haya
    decisiones. Si una unidad anterior decidió lo contrario, cítala y explica
    por qué cambia. Al planificar, `get_plan_context`.

### Siglas de gomemory

- **ACR** — Revisión Adversarial por Consenso: skill
  `adversarial-consensus-review` (tools `review_*`).
- **ADM** — Auditoría Defensiva Multilenguaje: skill `defensive-qa-audit`.
- **HCD** — flujo de unidades de cambio: skill `hcd` (`mem hcd`).

Si la persona usa una de estas siglas, invoca su skill; no preguntes qué significa.
