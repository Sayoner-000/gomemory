# Fase 1 — Modelo de datos

La matriz es **declaración de dominio**: sin I/O, sin rutas absolutas, sin conocimiento del
sistema de archivos. Extiende el vocabulario que `domain/activation.go` y `domain/agents.go` ya
aportan.

## Entidad: `MatrixCell`

La unidad de la declaración. Une un agente, un tipo de canal y un ámbito con el artefacto que le
corresponde.

| Campo | Tipo | Descripción |
|---|---|---|
| `Agent` | `string` | Nombre del agente. Debe existir en `KnownAgents`. |
| `Kind` | `ChannelKind` | Tipo de canal. Vocabulario ya existente. |
| `Scope` | `AgentScope` | `ScopeProject` o `ScopeUser`. Determina quién puede tocar la celda. |
| `Path` | `[]string` | Ruta **relativa** del artefacto, por segmentos. Vacía si la celda no materializa un archivo. |
| `ConfigKey` | `string` | Clave bajo la que ese agente registra servidores en su configuración. Vacía si no aplica. |
| `Managed` | `bool` | `true` si gomemory escribe el artefacto; `false` si solo lo observa. |
| `Legacy` | `bool` | `true` si el artefacto lo generaban versiones anteriores y hoy solo se retira. |
| `NotApplicableReason` | `string` | Por qué esta celda no aplica a este agente. Excluyente con `Path`. |

### Invariantes

- **INV-1**: una celda tiene `Path` o `NotApplicableReason`, nunca ambos ni ninguno.
  Es la regla que convierte la ausencia silenciosa en fallo de verificación (FR-004, FR-005).
- **INV-2**: `Path` es siempre relativa. La resolución contra un directorio concreto pertenece a
  los adaptadores, no al dominio (principio I).
- **INV-3**: `Agent` existe en `KnownAgents`. Una celda huérfana es un error de declaración.
- **INV-4**: una celda con `Scope == ScopeUser` solo puede ser escrita o retirada por una
  actividad cuyo alcance declarado sea `ScopeUser` (FR-013).
- **INV-5**: una celda con `Legacy == true` no se escribe nunca; solo se retira.

## Entidad: `LifecycleActivity`

Las cinco actividades que operan sobre la matriz. Cada una declara su alcance, y ese alcance es
lo que hace verificable la contención.

| Actividad | Alcance | Qué hace con una celda |
|---|---|---|
| `ActivityInstall` | `ScopeProject` | Escribe las celdas de proyecto con `Managed`. |
| `ActivityInstallGlobal` | `ScopeUser` | Escribe las celdas de usuario con `Managed`, solo de los agentes solicitados. |
| `ActivityUninstall` | `ScopeProject` | Retira las celdas de proyecto con `Managed` o `Legacy`. |
| `ActivityCleanup` | `ScopeProject` | Retira las celdas de proyecto con `Legacy`. |
| `ActivityInspect` | ambos | Solo lee. Alimenta el diagnóstico. |

### Reglas

- **REG-1**: `ActivityUninstall` retira exactamente lo que `ActivityInstall` escribe, en el
  ámbito de proyecto (FR-008, FR-012).
- **REG-2**: ninguna actividad con alcance `ScopeProject` opera sobre celdas `ScopeUser`
  (FR-013). Cuando existe una celda de usuario relacionada, se informa (FR-014).
- **REG-3**: `ActivityInstallGlobal` limita sus efectos a los agentes recibidos (FR-017) y no
  crea el directorio de un agente no solicitado (FR-018).
- **REG-4**: `ActivityInspect` enumera exactamente las celdas de la matriz, sin lista propia
  (FR-007, SC-007).

## Relación con el registro de capacidades

`KnownAgents` sigue declarando **qué puede sostener** cada agente: los niveles y el dialecto. La
matriz declara **dónde vive** cada canal. Son dos preguntas distintas sobre el mismo eje y no se
fusionan:

- Un agente puede declarar el nivel de entrada y aun así no materializar archivo en el ámbito de
  proyecto, porque su mecanismo es global. Esa es una celda con motivo declarado.
- Una celda puede existir para un agente que no declara ningún nivel, si solo recibe el registro
  del servidor.

La verificación cruza ambos: un agente que declara un nivel con ámbito de usuario debe tener
celda para ese canal en ese ámbito, o motivo declarado (FR-005, FR-019).

## Transiciones de estado

La matriz no tiene estado propio: es declaración estática. El estado observado de cada celda lo
produce `ActivityInspect` y ya está modelado por `ChannelState` en `domain/activation.go`
(`ok`, `outdated`, `duplicated`, `missing`, `not_applicable`). Una celda con
`NotApplicableReason` produce siempre `StateNotApplicable` con ese motivo como detalle.
