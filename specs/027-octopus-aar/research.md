# Fase 0 — Investigación: Octopus AAR

Decisiones de diseño resueltas antes de escribir código. Cada una se apoya en lo que el repositorio ya hace, no en lo que sería bonito construir de cero.

Al terminar esta fase no queda ningún `NEEDS CLARIFICATION`.

---

## 1. Dónde vive la política de enrutamiento

**Decisión**: en `domain/`, como archivos planos con prefijo `octopus_`, con funciones puras y deterministas.

**Justificación**: la constitución exige que el dominio sea puro, sin I/O ni imports de infraestructura, y la especificación exige que la política sea verificable sin ejecutar ningún modelo (FR-009) y reproducible ante entradas idénticas (FR-010, SC-006). Una función pura cumple las tres cosas por construcción. Además `domain/` ya alberga capacidades completas con esta forma: `plan_shape.go` (evaluación de forma del plan), `channel_matrix.go`, `context_pack.go`.

**Alternativas descartadas**:

- *Subpaquete `internal/octopus/…` como sugería el documento de entrada*: el repositorio no tiene ningún subpaquete bajo `domain/`, y `ContextPack`, `Priority` y `ContextItem` ya viven en `domain`. Un subpaquete obligaría a importar `mem/domain` desde `mem/domain/octopus` — legal, pero introduce una jerarquía que ninguna otra funcionalidad necesitó — o a duplicar tipos.
- *Política en `application/usecases/`*: mezclaría reglas de negocio con orquestación de puertos y haría imposible probar la política sin montar dependencias.

---

## 2. Cómo se construye el contexto mínimo de lo delegado

**Decisión**: reutilizar `usecases.BuildContextPack` con una `ContextRequest` cuyo `MaxTokens` es el presupuesto de contexto de la unidad delegada. No se escribe ningún seleccionador de contexto nuevo.

**Justificación**: la funcionalidad 015 ya resolvió exactamente este problema — priorización (`PriorityCritical/Relevant/Optional`), umbral de relevancia, tope de items, compresión estructural, deduplicación y un reporte de reducción en `ContextStats`. `ContextRequest` ya admite `IncludeSpecKit`, `IncludeCodeGraph` y un `Recorder` opcional. Octopus solo necesita fijar `Task` con el objetivo de la unidad y `MaxTokens` con su presupuesto.

**Consecuencia directa**: FR-024 y FR-025 quedan satisfechos por composición, y SC-012 (reducción de contexto medible) se mide con `ContextStats`, que ya reporta `RawTokens`, `FinalTokens` y `SavedTokens`.

**Alternativas descartadas**: un empaquetador propio de Octopus. Duplicaría la lógica de presupuesto y compresión, y divergiría de ella con el tiempo.

---

## 3. Cómo se estiman los tokens sin API del proveedor

**Decisión**: usar el puerto existente `ports.TokenCounter` (implementación `tokens.ApproximateTokenCounter`) para toda estimación, y marcar como estimada cualquier cifra que no provenga de un reporte del runtime.

**Justificación**: INV-AAR-016 exige funcionar sin conteo exacto, y el proyecto ya definió ese puerto precisamente para poder cambiar la aproximación por un contador específico de proveedor sin tocar los casos de uso. El costo de delegar se estima como la suma de contexto empaquetado + contrato + salida esperada + un recargo fijo de coordinación e integración; los tres primeros son texto medible con el contador, el cuarto es una constante de dominio con nombre.

**Alternativas descartadas**: integrar un tokenizador real por proveedor. Introduce dependencia, peso en el binario y acoplamiento a un proveedor, contra INV-AAR-017 y el requisito de binario autocontenido.

**Dónde se cuenta, y por qué importa**: `ports.TokenCounter` vive en `application/ports`, que importa `mem/domain`. Si el dominio lo invocara, habría ciclo de imports y la política dejaría de ser pura. Por eso el reparto es estricto: **el caso de uso mide** (cuenta el texto del contexto, del contrato y de la salida esperada con el contador) y entrega esas cifras ya calculadas dentro de `WorkUnit`; **el dominio solo hace aritmética** sobre esas cifras y sobre constantes con nombre. La política nunca ve texto que deba medir.

---

## 4. Cómo se persiste la telemetría

**Decisión**: una sola tabla aditiva `octopus_executions`, creada con `CREATE TABLE IF NOT EXISTS` dentro de `migrate()`. Una fila por decisión; las columnas del reporte del runtime empiezan nulas y se completan cuando llega. Los agregados por patrón de tarea se calculan con `SELECT` sobre esa tabla, no se materializan.

**Justificación**: mantiene el esquema aditivo que exige la funcionalidad 009, evita el problema de sincronizar dos tablas (decisión y reporte) y hace trivial responder "qué se decidió y qué costó realmente" con una consulta. La evidencia histórica de FR-049 es un agregado, no un registro nuevo.

**Restricción de privacidad como forma de la tabla**: la tabla no tiene ninguna columna de texto libre alimentada por contenido de contexto. Solo identificadores, enums, cifras y una razón que genera la propia política a partir de un catálogo cerrado. Esto convierte INV-AAR-013, FR-047 y SC-011 en una propiedad del esquema, verificable con una prueba de contrato, en vez de una promesa de quien escribe el código.

**Alternativas descartadas**: reutilizar `usage_records`. Su forma (operación, tokens base, tokens emitidos) no admite ruta, dependencias, grupo paralelo ni resultado, y forzarla ahí ensuciaría el reporte de uso existente.

---

## 5. Cómo se apaga el módulo de verdad

**Decisión**: `OctopusEnabled bool` en la configuración del proyecto, ausente o `false` = apagado. Con el módulo apagado, las tools MCP de Octopus **no se registran**; `domain.MCPAllTools()` conserva su significado actual y se añade `domain.MCPToolsFor(octopusEnabled bool)` para el caso encendido.

**Justificación**: registrar tools que no se van a usar contradice todo el trabajo de economía de contexto del proyecto (funcionalidades 008, 015, 020, 023) y rompería SC-001, porque el esquema de cada tool viaja al agente en cada arranque de sesión. El bloque de protocolo, el bootstrap de ToolSearch y las listas de auto-aprobación derivan de `domain/mcp_tools.go`, así que todos pasan a consultar `MCPToolsFor(...)`; con el módulo apagado producen exactamente el texto que producen hoy.

**Riesgo aceptado y su cierre**: el registro condicional obliga a extender el test de contrato que compara `MCPAllTools()` contra el `tools/list` real del servidor. Se extiende, no se relaja: la aserción actual se conserva íntegra para el caso apagado y se añade una segunda para el encendido. Está declarado como tarea explícita porque la constitución prohíbe tocar tests existentes sin autorización.

**Sobre la polaridad del nombre**: casi todos los ajustes vecinos son `*Disabled` (ausente = activado), porque refinan un flujo que ya existe y debe seguir activo sin que nadie opte por él. `AdrSyncEnabled` es el precedente contrario y exacto: una capacidad grande y opcional, en positivo, apagada por defecto. Octopus sigue ese precedente.

**Alternativas descartadas**:

- *Registrar siempre y responder "módulo desactivado" desde el handler*: mantiene estable el test de contrato, pero paga el costo de contexto que la funcionalidad dice ahorrar. Se descarta por incoherente con el propio producto.
- *Ajuste global de máquina en vez de por proyecto*: la configuración de gomemory es por proyecto y la memoria está aislada por proyecto. Un interruptor global sería el único ajuste con alcance distinto al resto.

---

## 6. De dónde sale el grafo de tareas a enrutar

**Decisión**: tres orígenes, ninguno obligatorio. (a) El llamador entrega el grafo en la petición — es la vía principal y la única que usan las tools MCP. (b) `mem octopus plan` sin argumentos lee la funcionalidad activa vía `ports.SpecKitReader`, que ya expone `TaskDependencies` en `SpecKitFeatureContext`. (c) Un archivo JSON con la misma forma que la petición.

**Justificación**: FR-054 exige poder enrutar cualquier grafo estructurado sin convertir a Spec Kit en dependencia dura. El puerto `SpecKitReader` ya existe y ya está acotado a una sola funcionalidad, así que la integración sale gratis y degrada sola: sin `.specify/feature.json`, `ActiveFeature` devuelve cadena vacía sin error y el comando pide un origen explícito.

---

## 7. Cómo se garantiza el determinismo

**Decisión**: la política nunca recorre un mapa para producir salida ordenada, no consulta el reloj ni usa aleatoriedad. Toda colección de salida se ordena por identificador de tarea. La prueba correspondiente enruta el mismo plan cien veces y compara la serialización completa.

**Justificación**: el orden de iteración de un mapa en Go es deliberadamente aleatorio. Un plan de enrutamiento construido recorriendo un mapa produciría grupos paralelos en orden distinto en cada ejecución: no rompe la corrección, pero sí SC-006 y la confianza en la simulación, y el fallo aparece de forma intermitente. Es más barato prohibirlo por diseño que diagnosticarlo después.

---

## 8. Frontera con la ejecución

**Decisión**: ningún componente de Octopus crea, lanza, espera ni cancela procesos. La superficie de agente devuelve decisiones y contratos; el runtime ejecuta y llama a la operación de reporte cuando termina.

**Justificación**: es INV-AAR-018 y la razón de ser del producto. También es lo que mantiene el binario libre de gestión de procesos de agentes, claves de proveedor y modelos concretos (INV-AAR-017).

**Consecuencia sobre el paralelismo**: `PARALLEL` no significa que Octopus ejecute nada en paralelo. Significa que las tareas de ese grupo pueden ejecutarse a la vez y que el número de miembros respeta el mínimo entre `max_parallel` del runtime y el tope configurado.

---

## 9. Qué queda deliberadamente fuera

- **Enrutamiento asistido por modelo** (§60 del documento de entrada). La primera versión es determinista. Si más adelante se incorpora, la propuesta del modelo pasa por las mismas validaciones de presupuesto, dependencias, capacidades, seguridad, fan-out y recursión; el modelo propone, Octopus valida.
- **Revisión adversarial por consenso como estrategia ejecutable**. Se contempla como ruta seleccionable en el modelo de datos, pero su implementación es otra funcionalidad. Aquí solo se reserva presupuesto de validación.
- **Detección de capacidades del runtime**. Llegan declaradas por quien llama. Octopus no sondea el entorno.
