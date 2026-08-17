# Fase 1 — Modelo de datos

Esta feature **no cambia el esquema de SQLite**. Sus entidades son de dominio (una función pura y sus
valores), de estado efímero en disco (un contador por sesión) y de diagnóstico (el reporte de
cobertura, que se compone en memoria a partir de lo que hay instalado).

## 1. `PlanShapeVerdict` — veredicto sobre la forma de un plan

Valor de dominio devuelto por la función pura de evaluación. Sin I/O, sin dependencias.

| Valor | Significado | Efecto en el borde de salida |
|---|---|---|
| `ShapeOK` | El plan presenta estructura de árbol reconocible | Permitir |
| `ShapeMissing` | El plan supera el umbral de tamaño y **no** presenta ninguna señal de estructura | Devolver con motivo (si el episodio lo permite) |
| `ShapeNotApplicable` | El plan es demasiado corto para exigir descomposición, o no hay texto que evaluar | Permitir |

**Señales de estructura** (cualquiera basta para `ShapeOK`, todas independientes del idioma):

- glifos de árbol: `├─`, `└─`, `│`
- identificadores jerárquicos de dos o más niveles: `[1.2]`, `1.2`, `1.2.3`
- marcadores del formato del método: `✓`, `⚠`, `dep:`, `∥`

**Reglas de validación**:

- El umbral de tamaño se declara como constante única de dominio, con un valor conservador (un plan
  corto nunca se bloquea). Cambiarlo es un cambio de dominio con su prueba, no un ajuste de
  configuración: no debe poder aflojarse por descuido en producción.
- La flecha de resultado (`→`) **no** participa del veredicto; solo se consulta para enriquecer el
  motivo que se devuelve al agente.
- Entrada vacía o solo espacios → `ShapeNotApplicable`. Nunca `ShapeMissing`.

**Estados de error**: la función no falla. No hay rama de error posible: cualquier entrada produce uno
de los tres veredictos.

## 2. `PlanEpisodeState` — contador de devoluciones por episodio

Estado efímero en disco, bajo `.memory/`, del mismo tipo que los marcadores ya existentes
(`.session-tools-injected`, debounce de los nudges). No entra a la base de datos: se lee y escribe en
el camino crítico de un hook que debe resolverse en menos de 50 ms.

| Campo | Tipo | Descripción |
|---|---|---|
| `denials` | entero | Devoluciones ya emitidas en el episodio en curso. Solo interesa `0` frente a `≥ 1`. |

### Transiciones

```text
        plan-entered (entrada al modo plan)
                 │  denials := 0
                 ▼
        ┌──────────────────┐
        │  denials == 0    │  plan-guard con ShapeMissing
        │  (puede devolver)├──────────────────────────────► deny + motivo
        └──────────────────┘                                denials := 1
                 │                                                │
                 │ plan-guard con ShapeOK / NotApplicable         │
                 │ (permitir, sin cambio de estado)               ▼
                 │                                        ┌──────────────────┐
                 │                                        │  denials >= 1    │
                 │                                        │  (siempre permite)│
                 ▼                                        └──────────────────┘
        plan-approved (salida aprobada)  ─────────────────────────┘
                    denials := 0   (episodio cerrado)
```

**Invariantes**:

- Nunca más de una devolución entre dos reinicios del contador. La persona no puede quedar bloqueada
  dos veces por el mismo plan.
- Si `plan-entered` no está disponible en el agente, el ciclo sigue cerrando con `plan-approved`. El
  caso degradado —la persona rechaza el plan, así que no hay aprobación— deja el contador en `1` y el
  siguiente plan **pasa sin evaluar**: se peca de permisivo, nunca de bloqueante.
- Estado ilegible, ausente o corrupto → se trata como `denials = 0` solo si el archivo no existe; ante
  contenido inválido se trata como `≥ 1` (permitir). La duda siempre se resuelve permitiendo.
- Se limpia junto con los demás marcadores de sesión en el arranque de sesión.

## 3. `ActivationChannel` — canal de activación inspeccionable

Entidad de dominio que describe una vía concreta por la que una garantía del modo plan llega a un
agente. Es lo que el reporte de cobertura enumera.

| Campo | Tipo | Descripción |
|---|---|---|
| `Arm` | enum | `gomemory` \| `codegraph` — a qué brazo pertenece el canal |
| `Agent` | texto | Agente al que sirve (`claude`, `opencode`, …) |
| `Scope` | enum | `project` \| `user` |
| `Kind` | enum | `plan_entry` \| `plan_guard` \| `turn_reminder` \| `instructions` \| `native_wrapper` \| `mcp_instructions` |
| `State` | enum | `ok` \| `outdated` \| `duplicated` \| `missing` \| `not_applicable` |
| `Detail` | texto | Versión de protocolo encontrada, ruta relativa o motivo de la degradación |

**Reglas de validación**:

- `not_applicable` está reservado a "el agente no está instalado en esta máquina" o "el agente no
  soporta este tipo de canal". Nunca se usa para ocultar un canal roto.
- `duplicated` es un estado propio y no un caso de `ok`: es precisamente la regresión que introduce
  una reinstalación con el filtro de idempotencia incompleto.
- Los canales del brazo `codegraph` son de **solo lectura**: se inspeccionan y se reportan, nunca se
  escriben ni se corrigen (INV-1).

## 4. `CoverageReport` — reporte de cobertura de los dos brazos

Agregado que compone el caso de uso de diagnóstico. No se persiste: se calcula al invocarlo.

| Campo | Tipo | Descripción |
|---|---|---|
| `Channels` | lista de `ActivationChannel` | Todos los canales inspeccionados, de ambos brazos |
| `Degradations` | lista de texto | Garantías que **no** están disponibles y por qué (p. ej. "OpenCode: sin devolución en el borde de salida") |
| `Problems` | entero | Cantidad de canales en `outdated`, `duplicated` o `missing` |

**Reglas**:

- `Problems == 0` es la única condición de éxito en modo estricto.
- Una degradación declarada **no** cuenta como problema: es información, no fallo.
- El reporte debe ser estable entre ejecuciones (orden determinista) para que un script pueda
  compararlo.

## 5. `AgentCapability` — registro único de capacidades por agente

Tabla de dominio: la **única** fuente sobre qué puede hacer cada agente. Añadir un agente es añadir
una entrada aquí; el reporte de cobertura y la verificación de regresión se alimentan de ella.

| Campo | Tipo | Descripción |
|---|---|---|
| `Name` | texto | Identificador del agente (`claude`, `opencode`, `codex`, `cursor`, `windsurf`, `cline`, …) |
| `Dialect` | enum | `neutral` \| `json` \| `claude` \| `text` — cómo se le habla a este agente |
| `Levels` | conjunto | `guard` (nivel 1) \| `entry` (nivel 2) \| `text_floor` (nivel 3) |
| `Scopes` | conjunto | `project` \| `user` — ámbitos donde su configuración existe |
| `InstructionFiles` | lista | Archivos de instrucciones que lee, por ámbito |
| `NativeWrapper` | opcional | Ruta y encabezado de su formato propio de habilidad o comando |

**Reglas de validación**:

- `text_floor` es **obligatorio** en toda entrada: un agente sin piso textual no está soportado
  (FR-A5).
- `guard` solo se declara si el agente puede invocar un comando **antes** de presentar el plan y
  respetar su decisión. Declararlo sin esa capacidad produciría un fallo silencioso.
- `Dialect` describe una **traducción**, nunca la definición de la capacidad. Un agente sin dialecto
  declarado se atiende en `neutral`.
- Un agente **ausente del registro** no se rechaza: si invoca la capacidad, se le responde en
  `neutral`. El registro sirve para instalar y reportar, no para autorizar.

**Alcance declarado**: este registro nace como fuente única de lo que introduce esta feature
(niveles, dialectos, reporte). Las tablas por agente que ya existen dispersas en el instalador no se
migran aquí todavía — ver [research.md](./research.md) §13.3.

## 6. `ProtocolBlock` — bloque de protocolo administrado

No es una entidad nueva, pero su contrato cambia y de ahí depende FR-015.

| Campo | Descripción |
|---|---|
| Marcador de inicio | `<!-- gomemory-protocol-v<N> -->`, ya existente |
| Marcador de fin | **nuevo desde v8**; delimita el bloque de forma explícita |
| Límite para bloques legados | v1..v7 no tienen fin: el bloque termina en el siguiente encabezado de **nivel 2** o al final del archivo |

**Invariantes**:

- Todo lo anterior y todo lo posterior al bloque pertenece a la persona y se conserva byte a byte.
- Tras una actualización no queda ningún resto de la versión anterior ni aparece el bloque dos veces.
- La operación es idempotente: repetirla con la misma versión no cambia el archivo.
