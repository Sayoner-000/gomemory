# Feature Specification: Señal de grafo de código en Retrieval de ContextPack

**Feature Branch**: `018-codegraph-pack-retrieval`

**Created**: 2026-08-15

**Status**: Draft

**Input**: User description: "Integrar el grafo de código externo como señal de Retrieval en mem pack — hoy `mem context` ya usa el grafo externo (codebase-memory-mcp) como brazo extensor enchufable: lo embebe como sección informativa y usa ImpactFor(filepath) para resaltar memorias conectadas a hotspots vigentes. `mem pack build` (el pipeline Retrieval → Dedup → Priority → Compression → Token Budget → ContextPack) NO conoce el grafo externo hoy: sus candidatos vienen solo de memorias y Spec Kit. Cerrar esa brecha llevando el mismo patrón ya probado (snapshot cacheado, no-bloqueante, degradación silenciosa) al pipeline ContextPack, para que el grafo actúe como señal real de Retrieval/Ranking."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Memorias ligadas a código activo se priorizan al construir el ContextPack (Priority: P1)

Un AI Agent invoca `mem pack build --task "..."` (o la tool MCP equivalente `pack_build`) para obtener el contexto mínimo necesario para una tarea. Entre los candidatos recuperados hay una memoria cuyo archivo asociado (`Filepath`) es, según el grafo de código externo ya indexado, un punto de alto impacto (hotspot: muchas otras partes del código dependen de él). Hoy esa memoria compite por el presupuesto de tokens con la misma prioridad que cualquier otra memoria "Optional" — puede quedar afuera aunque sea la más relevante para tocar código con seguridad. Con esta feature, su prioridad sube un escalón (de Optional a Relevant) antes de repartir el presupuesto, así que tiene más chances de entrar en el ContextPack final.

**Why this priority**: Es el valor central de la feature — sin esto, el grafo de código sigue siendo un "brazo extensor" que solo `mem context` aprovecha, y `mem pack build` (el pipeline pensado específicamente para presupuestar tokens por tarea) sigue ciego a la estructura real del código.

**Independent Test**: Con un proveedor de grafo de código configurado y una memoria cuyo `Filepath` coincide con un hotspot vigente, correr `mem pack build --task "..." --max-tokens N` con un presupuesto que alcance para esa memoria en prioridad Relevant pero no en Optional, y confirmar que aparece en `pack.Items` con `Priority: Relevant`. Se puede probar sin ningún otro componente de la feature.

**Acceptance Scenarios**:

1. **Given** un proveedor de grafo de código con snapshot disponible y una memoria candidata cuyo archivo es un hotspot vigente, **When** se construye el ContextPack para una tarea que recupera esa memoria, **Then** su prioridad pasa de Optional a Relevant antes de repartir el presupuesto de tokens.
2. **Given** la misma situación pero la memoria candidata ya es Critical (por su tipo, p. ej. Bugfix o Decision), **When** se construye el ContextPack, **Then** su prioridad no cambia — el grafo nunca degrada ni le agrega valor a un ítem que ya está garantizado.
3. **Given** ningún proveedor de grafo de código configurado o disponible, **When** se construye el ContextPack, **Then** todas las prioridades se calculan exactamente igual que hoy, sin ningún cambio de comportamiento.

---

### User Story 2 - Orientación arquitectónica compacta disponible dentro del mismo presupuesto (Priority: P2)

Cuando hay un snapshot de grafo de código disponible, el ContextPack puede incluir, como un candidato más (no forzado), el mismo resumen compacto que hoy solo se ve en `mem context`: totales del grafo, lenguajes, módulos de facto (clusters) y hotspots más referenciados. Este candidato entra a competir por el presupuesto de tokens como cualquier otro — con prioridad baja (Optional) — así que solo aparece si sobra espacio después de cubrir lo crítico y relevante de la tarea.

**Why this priority**: Da valor agregado (orientación estructural sin tener que correr `mem context` aparte) pero no es indispensable para el objetivo central de la feature (la Historia 1); por eso puede entregarse después sin bloquear el MVP.

**Independent Test**: Con un proveedor de grafo de código con snapshot disponible y un presupuesto de tokens amplio, correr `mem pack build` y confirmar que aparece un ítem de arquitectura de código en el ContextPack. Con un presupuesto muy ajustado, confirmar que ese ítem es el primero en descartarse (es Optional).

**Acceptance Scenarios**:

1. **Given** un snapshot de grafo de código disponible y presupuesto suficiente, **When** se construye el ContextPack, **Then** incluye un ítem con el resumen de arquitectura del grafo (nodos, lenguajes, clusters, hotspots).
2. **Given** ningún snapshot disponible (proveedor no instalado, no indexado, o snapshot vencido sin refrescar aún), **When** se construye el ContextPack, **Then** no aparece ningún ítem de arquitectura de código — cero error, cero mención.
3. **Given** un snapshot disponible pero presupuesto insuficiente para incluir ese ítem junto con lo crítico/relevante de la tarea, **When** se construye el ContextPack, **Then** el ítem de arquitectura queda descartado y contabilizado en las estadísticas de descarte, sin afectar al resto del paquete.

---

### User Story 3 - Desactivar la señal de grafo de código por invocación (Priority: P3)

Quien invoca `mem pack build` (persona o agente) puede desactivar explícitamente la consulta al grafo de código para una invocación puntual, de forma independiente a si incluye o no Spec Kit. Por defecto queda activada (mismo criterio que Spec Kit hoy).

**Why this priority**: Es una válvula de escape para depurar o comparar comportamiento, no algo que la mayoría de las invocaciones necesite tocar — de ahí la prioridad más baja.

**Independent Test**: Correr `mem pack build --task "..." --max-tokens N --no-code-graph` con un proveedor de grafo configurado y confirmar que ningún ítem ni ajuste de prioridad proviene del grafo, aunque el proveedor esté disponible.

**Acceptance Scenarios**:

1. **Given** un proveedor de grafo disponible, **When** se invoca con la señal de exclusión activada, **Then** el ContextPack resultante es idéntico al que se obtendría sin proveedor configurado.

---

### Edge Cases

- ¿Qué pasa si el proveedor de grafo de código está configurado pero el snapshot cacheado nunca se generó (primera vez, sin refresco todavía)? → Se trata igual que "no disponible": cero impacto, cero error.
- ¿Qué pasa si hay más de un proveedor de grafo de código configurado (feature 010, Historia 3, múltiples candidatos)? → Se usa el mismo criterio de selección ya establecido en el proyecto (primer proveedor disponible) para el ítem de arquitectura; el boost de prioridad por hotspot puede consultarse contra todos.
- ¿Qué pasa si el snapshot está disponible pero corresponde a un proyecto/root distinto del que pidió el ContextPack? → No aplica ningún ajuste (mismo aislamiento por proyecto que ya rige el resto del pipeline).
- ¿Qué pasa si ninguna memoria candidata tiene `Filepath` asociado? → El boost de prioridad simplemente no encuentra nada que ajustar; el resto del pipeline sigue igual.
- ¿Qué pasa si el candidato de arquitectura de código, aun en prioridad Optional, desplazaría a una memoria Optional más relevante para la tarea? → Compite en igualdad de condiciones por orden de prioridad/relevancia ya establecido; no tiene trato preferencial.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: El sistema DEBE permitir que la construcción del ContextPack consulte, de forma opcional, un grafo de código externo ya indexado, cuando hay uno configurado y con snapshot disponible.
- **FR-002**: El sistema NUNCA DEBE bloquear, retrasar de forma perceptible, ni hacer fallar la construcción del ContextPack por causa del grafo de código externo — proveedor ausente, snapshot vencido, o cualquier error de ese componente se degrada en silencio.
- **FR-003**: El sistema DEBE elevar la prioridad de una memoria candidata (de Optional a Relevant) cuando su archivo asociado coincide con un punto de alto impacto (hotspot) vigente en el grafo de código.
- **FR-004**: El sistema NUNCA DEBE reducir la prioridad de una memoria candidata, ni quitarle valor, por causa del grafo de código — la señal solo puede aumentar la chance de inclusión, nunca disminuirla.
- **FR-005**: Cuando hay snapshot de grafo de código disponible y el presupuesto de tokens lo permite, el sistema DEBE ofrecer un ítem compacto de orientación arquitectónica (totales, lenguajes, módulos de facto, hotspots) como un candidato más, sujeto a las mismas reglas de prioridad y presupuesto que el resto — nunca forzado.
- **FR-006**: Quien invoca la construcción del ContextPack DEBE poder desactivar la consulta al grafo de código para una invocación puntual, de forma independiente a la inclusión de Spec Kit.
- **FR-007**: Por defecto (sin desactivación explícita), la consulta al grafo de código DEBE estar activada — mismo criterio de "activado por defecto" que ya rige la inclusión de Spec Kit.
- **FR-008**: El sistema DEBE reutilizar exclusivamente el mismo contrato de lectura de solo-snapshot-cacheado que ya usa el resto del proyecto para el grafo de código externo — ninguna llamada en vivo al proveedor durante la construcción del ContextPack.
- **FR-009**: El comportamiento y la salida de la construcción del ContextPack para quienes no tienen un grafo de código configurado, o lo desactivan, DEBE permanecer idéntico al comportamiento anterior a esta feature.

### Key Entities

- **Snapshot de grafo de código**: el estado cacheado (nodos, lenguajes, módulos de facto, hotspots) de un proveedor externo de grafo de código, ya existente en el proyecto — esta feature solo lo consume, no lo redefine.
- **Candidato de contexto**: unidad intermedia ya existente en el pipeline de construcción del ContextPack (memoria o artefacto de Spec Kit) con prioridad/relevancia asignada; esta feature agrega una nueva fuente de candidatos (arquitectura de código) y un nuevo ajustador de prioridad (boost por hotspot) sobre los ya existentes.
- **Anotación de impacto**: el resultado, ya existente, de consultar el snapshot cacheado por un archivo puntual, indicando si es un hotspot vigente y con qué fan-in.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Cuando hay un grafo de código configurado y disponible, el 100% de las memorias candidatas cuyo archivo es un hotspot vigente quedan en prioridad Relevant (o superior) antes de repartir el presupuesto de tokens — nunca en Optional.
- **SC-002**: El 100% de las construcciones de ContextPack completan sin error, con una sobrecarga de latencia menor a 50 ms atribuible a la consulta del grafo de código, tanto con proveedor instalado como sin él.
- **SC-003**: Con la señal de grafo de código desactivada explícitamente, el ContextPack resultante es indistinguible, ítem por ítem, del que se obtenía antes de esta feature.
- **SC-004**: Cuando el presupuesto de tokens alcanza, el ítem de orientación arquitectónica aparece en el ContextPack en el 100% de las construcciones con snapshot disponible; cuando no alcanza, queda correctamente contabilizado como descartado en las estadísticas, sin afectar el resto del paquete.

## Assumptions

- El proveedor externo de grafo de código (cuando está configurado) ya mantiene un snapshot cacheado localmente y refrescado en background — esta feature solo lee ese snapshot, con el mismo contrato de no-bloqueo que ya usa `mem context` hoy.
- Las definiciones de "hotspot" y "resumen de arquitectura" ya existen en el modelo del grafo de código del proyecto; esta feature no las redefine, solo las conecta a un pipeline adicional.
- El comportamiento por defecto (sin desactivación explícita) es "consultar el grafo de código", igual que Spec Kit ya se incluye por defecto.
- Esta feature aplica a la construcción de ContextPack (`mem pack build` y su tool MCP equivalente); no modifica el comportamiento de `mem context`.
- Con más de un proveedor de grafo de código configurado, el ítem de orientación arquitectónica usa el primero disponible (mismo criterio de selección ya establecido en el proyecto para casos análogos); el boost de prioridad por hotspot puede considerar todos los proveedores configurados.
