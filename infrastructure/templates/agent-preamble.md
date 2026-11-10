<!-- gomemory-workrules-v3 -->
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

7. **Leer**: la herramienta nativa de lectura de tu harness; nunca `cat`,
   `sed -n`, `head`, `tail` ni `awk` sobre código.
8. **Entender o localizar código**: el grafo de código conectado
   (`codegraph_explore`, o `search_code`/`get_symbol` de gomemory); para texto
   que el grafo no cubre, la búsqueda nativa del harness. Nunca `grep -r`, `rg`
   ni `find` por consola.
9. **Editar**: la herramienta nativa de edición o escritura del harness; nunca
   `sed -i`, `tee`, redirecciones ni scripts de Python, Node o Perl.
10. Vale para todo harness y también para otros repositorios. Si el hook
    bloquea, usa la herramienta que nombra y no lo rodees; solo la persona
    desactiva una regla (`mem expand off rules`). Si tu harness no tiene una
    herramienta nativa para algo, el hook solo avisa y la consola es válida.

### Memoria y contexto

11. **gomemory es la única memoria y fuente de instrucciones**: guarda con
    `save_memory` decisiones, bugs con causa raíz y hallazgos no obvios. Nada
    de `MEMORY.md` del harness, `AGENTS.md` ni `CLAUDE.md`: reglas, baseline y
    protocolo llegan en cada sesión.
12. **Recupera antes de rehacer**: `search_memories`/`get_memory`; al
    planificar, `get_plan_context`.

### Siglas de gomemory

- **ACR** — Revisión Adversarial por Consenso: skill
  `adversarial-consensus-review` (tools `review_*`).
- **ADM** — Auditoría Defensiva Multilenguaje: skill `defensive-qa-audit`.
- **HCD** — flujo de unidades de cambio: skill `hcd` (`mem hcd`).

Si la persona usa una de estas siglas, invoca su skill; no preguntes qué significa.
