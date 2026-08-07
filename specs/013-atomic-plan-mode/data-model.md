# Modelo de datos — Modo Plan Atómico con Memoria

**Feature**: 013-atomic-plan-mode
**Fecha**: 2026-08-06

Esta feature **no introduce tablas ni migraciones de SQLite**. Todo lo que persiste ya
tiene su lugar: el plan aprobado se guarda como una memoria `type=decision` por la ruta que
ya existe, y el interruptor de apagado vive en el archivo de configuración del proyecto.

Lo que sí se modela es: un campo de configuración nuevo, la forma del documento que
devuelve la ruta de contexto de planificación, y la estructura del método que se distribuye.

---

## 1. Entidades persistentes

### 1.1 `SettingsData` — campo nuevo

**Ubicación**: `application/ports/settings_repository.go`
**Archivo físico**: `.memory/settings.json` del proyecto

| Campo | Tipo | Etiqueta JSON | Default | Significado |
|-------|------|---------------|---------|-------------|
| `AtomicPlanDisabled` | `bool` | `atomic_plan_disabled,omitempty` | `false` (ausente) | Apaga la planificación atómica en este proyecto |

**Reglas de validación**:

- Ausente o `false` ⇒ funcionalidad **activa**. Es la convención ya establecida por
  `SpeckitContextDisabled`, `SynapseDisabled` y `CodeGraphDisabled`.
- La retrocompatibilidad es automática: un `settings.json` escrito antes de esta feature no
  tiene el campo, se deserializa como `false`, y la funcionalidad queda encendida — que es
  lo que FR-025 exige.
- `omitempty` evita ensuciar el archivo de los proyectos que nunca tocan el ajuste.

**Sin transiciones de estado**: es un interruptor de dos posiciones sin estados
intermedios. Se escribe desde la pantalla de configuración de la interfaz de texto y se
lee en cada invocación (nunca se cachea en proceso — la constitución prohíbe cachear
valores que cambian en caliente).

---

### 1.2 Plan aprobado — sin cambios de esquema

**Se reutiliza tal cual.** Un plan aprobado se persiste como una memoria existente:

| Atributo | Valor |
|----------|-------|
| `type` | `decision` |
| `title` | Derivado por `planTitle()` de la primera línea con contenido |
| `content` | El texto íntegro del plan, con su árbol de descomposición |
| Prompt originante | Lo adjunta `InsertMemory` automáticamente |

**Regla que hay que preservar**: solo se registran planes **aprobados**. En Claude Code eso
lo garantiza el propio disparador (`PostToolUse` con matcher `ExitPlanMode` no se ejecuta
si la persona rechaza el plan). En los demás agentes, quien invoca `mem hook plan-approved`
debe hacerlo únicamente tras la aprobación.

**Consecuencia para el árbol atómico**: como el contenido se guarda como texto sin
transformar, la jerarquía, los identificadores `[1.2.1]`, las marcas `✓`, las dependencias
`dep: 1.2` y las marcas de paralelismo `∥` sobreviven intactos. FR-035 se cumple sin
esquema nuevo.

---

## 2. Entidades de solo lectura (no persistidas)

Estas estructuras se construyen en memoria y se emiten; no se guardan.

### 2.1 `PlanContext` — el documento que devuelve la ruta de planificación

Es el resultado de `get_plan_context()` / `mem plan-context`. Un documento Markdown con
dos partes rotuladas:

```
┌─ PlanContext ─────────────────────────────────┐
│                                               │
│  Parte 1: Método de descomposición atómica    │
│    (siempre presente, salvo apagado)          │
│                                               │
│  Parte 2: Contexto histórico del proyecto     │
│    (presente solo si hay memoria y no falla)  │
│    · Memorias por tipo                        │
│    · Preferencias del usuario                 │
│    · Relaciones entre memorias                │
│    · Resumen del grafo de código (si existe)  │
│                                               │
└───────────────────────────────────────────────┘
```

**Composición**:

| Parte | Origen | Presupuesto |
|-------|--------|-------------|
| Método | Plantilla embebida en el binario | Fijo, no acotado |
| Contexto | `ContextBuilder.Build()` | Acotado por `SettingsData.Budget` |

**Regla de presupuesto (FR-007)**: la parte de contexto **debe** obtenerse llamando a
`ContextBuilder.Build()`, nunca reconstruyéndola. El techo se aplica dentro de ese caso de
uso (`build_context.go`, con `budgetReserve = 300`), así que reutilizarlo lo hereda;
duplicar la lógica lo rompería.

**Estados posibles del documento** (FR-032, FR-034 — ver D7 de `research.md`):

| Estado | Condición | Contenido | Código de salida |
|--------|-----------|-----------|------------------|
| Completo | Todo normal | Método + contexto | 0 |
| Degradado | Sin memoria inicializada, o `Build()` falla | Solo método | 0 |
| Silenciado | `atomic_plan_disabled: true` | Vacío | 0 |

**Invariante**: el código de salida es **siempre** 0. Ninguna rama interrumpe el modo plan
(FR-034). Es el mismo criterio ya verificado en producción por el script del brazo extensor
de spec-kit.

---

### 2.2 `AtomicMethod` — el método de descomposición

**Origen**: plantilla embebida en `infrastructure/templates/`, ya cubierta por la directiva
`go:embed all:templates` existente. No requiere una directiva nueva.

**Fuente de contenido**: `specs/013-atomic-plan-mode/reference-ads-baseline.md`, la versión
optimizada aportada por el usuario, más las cuatro adiciones que la tabla de brecha de ese
mismo archivo identifica.

**Secciones**:

| Sección | Requisitos que satisface | Estado en la línea base |
|---------|--------------------------|-------------------------|
| Test de atomicidad (5 condiciones) | FR-010, FR-011 | Ya resuelto |
| Procedimiento de descomposición | FR-009, FR-013 | Ya resuelto |
| Nomenclatura `[id] verbo + objeto → resultado` | FR-011 | Ya resuelto |
| Dependencias `dep:` y paralelismo `∥` | FR-014 | Ya resuelto |
| Formato de salida en árbol | FR-012 | Ya resuelto |
| Umbral de 25 hojas y proporcionalidad | FR-015, FR-017 | Ya resuelto |
| **Uso del historial al descomponer** | FR-016 | **Falta añadir** |
| **Autoverificación previa a la entrega** | FR-018 | **Falta añadir** |
| **Marcado de hoja no atómica con motivo** | FR-019 | **Falta añadir** |
| Detenerse en modo plan, sin ejecutar | FR-020, FR-021 | Ya resuelto |

**Fuente única de verdad**: esta plantilla alimenta las tres salidas —la herramienta MCP, el
comando de línea de comandos y los envoltorios nativos por agente—. Ninguna de las tres
lleva su propia copia editable.

---

### 2.3 `AtomicTask` — la tarea atómica

**No es una estructura de datos del programa.** Es una entidad del dominio del método: vive
como texto dentro del plan que produce el agente. Se documenta aquí porque la spec la
declara como entidad y porque los criterios de éxito se miden sobre ella.

| Atributo | Representación en el texto del plan |
|----------|-------------------------------------|
| Identificador jerárquico | `[1.2.1]` |
| Acción + objeto | Verbo de acción seguido de un objeto concreto |
| Resultado esperado | Tras la flecha `→` |
| Es atómica | Marca `✓` en la hoja |
| No atómica, con motivo | Marca explícita y razón declarada (FR-019) |
| Dependencias | `(dep: 1.1)` |
| Paralelizable | `(∥)` |

**Por qué no se modela como struct**: el sistema no analiza ni valida el plan
programáticamente — la decisión D5 de `/speckit-specify` fue autovalidación del agente, sin
compuerta externa. Construir un analizador sintáctico del árbol sería precisamente el
"validador externo" que la spec puso fuera de alcance.

---

### 2.4 `InstallationScope` — ámbito de instalación

**No es un dato persistido**: es la resolución, en tiempo de instalación, de dónde escribir
los artefactos.

| Ámbito | Destino del bloque de protocolo | Destino de los envoltorios nativos |
|--------|--------------------------------|-----------------------------------|
| Global | Archivo de instrucciones de usuario de cada agente | Directorios de usuario del agente |
| Proyecto | `AGENTS.md`, `CLAUDE.md`, `.cursorrules`, `.windsurfrules` del proyecto | Directorios del proyecto |

**Regla de precedencia (FR-025)**: cuando coexisten, gana el ámbito de proyecto. No se
implementa una resolución propia: se apoya en la que cada agente ya aplica al mezclar su
configuración de usuario con la del proyecto — el comportamiento que D1 verificó
empíricamente para OpenCode y que Claude Code aplica a su memoria de usuario.

**Invariante (SC-013)**: exactamente una de las dos versiones del método aplica en cada
sesión, nunca ambas. Se consigue porque el bloque de protocolo es **el mismo bloque
versionado** en los dos ámbitos: `composeAgentFile` reemplaza por marcador de versión, así
que el archivo de proyecto sobrescribe conceptualmente al de usuario en lugar de sumarse.

---

## 3. Relación entre entidades

```
SettingsData.AtomicPlanDisabled
        │
        │ (si true → silenciado)
        ▼
   PlanContext ◄──── AtomicMethod (plantilla embebida)
        │      ◄──── ContextBuilder.Build() [presupuesto aplicado aquí]
        │
        │ el agente lo consume al entrar en modo plan
        ▼
   Plan con AtomicTask (texto)
        │
        │ si la persona aprueba
        ▼
   Memoria type=decision  ─── ruta ya existente, sin cambios
```

---

## 4. Lo que esta feature NO toca

Declarado explícitamente para acotar el riesgo de la implementación:

- **Sin migraciones de SQLite**. No hay tablas, columnas ni índices nuevos.
- **Sin cambios en `ContextBuilder`**. Se consume tal cual; modificarlo afectaría al
  arranque de sesión de todos los proyectos.
- **Sin cambios en el esquema de memorias**. El plan aprobado usa `type=decision`, que ya
  existe.
- **Sin cambios en `hookPlanApproved`**. D9 verificó que ya cumple FR-035 y FR-036; solo
  hay que comprobar que el árbol con caracteres de dibujo sobrevive intacto.
