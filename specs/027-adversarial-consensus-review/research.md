# Investigación (Fase 0): Revisión Adversarial por Consenso

Este documento resuelve las decisiones técnicas necesarias antes del diseño detallado. La especificación (`spec.md`) no dejó marcadores `[NEEDS CLARIFICATION]`, pero sí una tensión de diseño explícita que hay que resolver antes de definir el esquema de datos: **quién razona semánticamente y quién impone las reglas**.

## Decisión 1 — GoMemory no orquesta modelos ni juzga equivalencia semántica de hallazgos

**Decisión**: GoMemory nunca ejecuta revisores ni un agente de corrección, y tampoco decide por sí sola si dos hallazgos en lenguaje natural describen "el mismo comportamiento y mecanismo causal" (FR-015). Esa evaluación semántica la hace el agente/skill orquestador (que sí tiene acceso a un modelo). GoMemory recibe esa propuesta ya evaluada — qué hallazgo de A corresponde a cuál de B, con qué estado propuesto (CONFIRMED/SUSPECT/CONTRADICTION) — y actúa como el único punto que **valida y persiste** esa clasificación contra las reglas estructurales del protocolo: que ambos hallazgos vengan de revisores distintos del mismo target congelado (INV-002), que un CONFIRMED tenga en efecto dos fuentes independientes (INV-004), que un SUSPECT nunca se cuele como CONFIRMED, y que la clasificación final que persiste sea la de GoMemory, no la que el agente afirme.

**Justificación**: es exactamente el patrón que ya usa `judge_memories` → `RecordVerdict` (`application/usecases/record_verdict.go`) en este mismo repositorio: el agente calcula la relación entre dos memorias y su confianza; GoMemory no re-evalúa el contenido, solo garantiza la identidad de las partes (`idA != idB`), persiste el veredicto como el estado más reciente de esa relación y lo hace disponible para consultas futuras. Aplicar la evaluación semántica de equivalencia dentro de GoMemory exigiría que GoMemory ejecute o llame a un modelo — lo que contradice el requisito explícito de independencia de proveedor (sección 3, Non-Goals: "depender de un proveedor específico de IA") y el principio operativo existente ("MCP como integración primaria", sin lógica de inferencia embebida). Mantener a GoMemory como state machine + validador es lo que hace posible el diseño principal de la spec ("GoMemory owns invariants") sin que GoMemory se convierta en un cliente de LLM.

**Alternativas consideradas**:
- *GoMemory implementa matching por similitud textual/embeddings propio*: rechazada — la spec prohíbe explícitamente basarse solo en similitud textual (FR-015), y añadir un motor de embeddings sería una dependencia nueva no justificada para una decisión que el agente orquestador ya puede tomar con su propio razonamiento.
- *El propio revisor decide unilateralmente el consenso*: rechazada — rompería FR-006 (aislamiento entre revisores) y el principio de que ningún reviewer es la única autoridad.

## Decisión 2 — Identidad y congelamiento del target

**Decisión**: la CLI (`mem review --diff|--commit|--file`) resuelve la identidad del target localmente antes de llamar a `start_review`: para `--commit`/`--diff` usa el SHA que ya provee git; para `--file`/conjuntos de archivos sin identificador natural, GoMemory calcula un digest SHA-256 determinista sobre `tipo + rutas ordenadas + contenido` de los archivos en el alcance declarado. `start_review` persiste `(target_type, revision, digest, scope, created_at)` una sola vez por revisión y la expone igual a ambos revisores; cualquier envío posterior (`submit_reviewer_result`, `build_consensus`) que declare un `digest` distinto al congelado se rechaza con "target changed" (FR-004, AC-003), sin que GoMemory necesite vigilar el sistema de archivos.

**Justificación**: evita que GoMemory dependa de git como librería (no es una dependencia del proyecto hoy) y reutiliza que la CLI ya corre en el working tree del usuario. El rechazo por digest distinto es una comparación de igualdad de strings — trivialmente verificable por código, sin ambigüedad de prompt.

**Alternativas consideradas**: que GoMemory shell-ee `git` internamente — rechazada por acoplar el core del protocolo a la presencia de git incluso para targets que no son código (una especificación, un plan).

## Decisión 3 — Independencia de revisores es declarada, no detectada

**Decisión**: el llamador declara `reviewer_a`/`reviewer_b` (proveedor+modelo, si el runtime lo permite) y si la ejecución fue paralela/aislada. `start_review` calcula el `independence.level` con una regla determinista: mismo proveedor+modelo declarado para A y B ⇒ `degraded` con motivo `same-model`; proveedor o modelo distintos ⇒ `full`. GoMemory nunca infiere aislamiento real del runtime (no tiene visibilidad de ese proceso) — solo impide que se declare `full` cuando los datos aportados no lo sustentan (FR-009).

**Justificación**: cumple el requisito de que el sistema "detecte o reciba" las capacidades del runtime sin inventar telemetría que GoMemory no puede observar; la validación (no presentar como pleno lo que es degradado) sí es una regla verificable en código.

**Alternativas consideradas**: exigir siempre proveedores distintos — rechazado explícitamente por la spec (Non-Goals: "no exigir obligatoriamente dos modelos diferentes").

## Decisión 4 — Promoción de memoria reutiliza `memories` + `topic_key`, no crea un almacén paralelo

**Decisión**: `promote_review_memory` construye una memoria de tipo `review_learning` con problema/causa raíz/resolución/verificación (sin transcript ni cadena de razonamiento) y la guarda con el `SaveMemory` existente usando `topic_key = "review-learning:{categoria}:{componente}"`. El mecanismo de dedup por `topic_key` ya implementado (feature 008-reduce-context-footprint) actualiza la memoria existente en vez de duplicarla (FR-034, AC-009) sin lógica nueva. Se añade una columna aditiva `memories.source_review_id` (nullable) para la trazabilidad de linaje (FR-038) hacia la revisión de origen.

**Justificación**: es la aplicación directa de la suposición ya registrada en `spec.md` ("GoMemory ya expone un mecanismo de sesión y contexto que esta funcionalidad reutiliza... sin introducir un sistema de memoria paralelo") y evita reimplementar deduplicación semántica que ya existe y está probada.

**Alternativas consideradas**: tabla `review_memories` independiente con su propio dedup — rechazada por duplicar lógica ya existente y correcta.

## Decisión 5 — Presupuesto de rondas y estado terminal son funciones puras de dominio

**Decisión**: `domain.DeriveVerdict(review, consensusFindings, fixRounds)` es una función pura (sin I/O) que, dado el estado persistido, calcula el veredicto: `INCOMPLETE` si algún resultado de revisor de la ronda activa falló; si no, `APPROVED` cuando no quedan hallazgos CONFIRMED de severidad CRITICAL/HIGH sin resolver; `ESCALATED` cuando se alcanzó `max_fix_rounds` con hallazgos sin resolver/regresados, o hay una contradicción de severidad alta sin resolver. Ningún parámetro de entrada del agente puede pedir una ronda adicional más allá de `max_fix_rounds` (INV-009): `record_fix` rechaza registrar una ronda con número mayor al presupuesto configurado.

**Justificación**: es precisamente la lógica que la spec exige que NO dependa solo del prompt (sección 44). Al ser una función pura, se prueba exhaustivamente con tests unitarios de tabla (aprobado / escalado por agotamiento / escalado por contradicción / incompleto por fallo de revisor) sin infraestructura, cumpliendo Testing First de la constitución.

**Alternativas consideradas**: dejar que el agente orquestador decida cuándo detenerse — rechazada explícitamente por la spec (INV-009, "no debe excederse de forma automática ni silenciosa").
