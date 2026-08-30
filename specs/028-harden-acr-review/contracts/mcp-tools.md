# Contrato — Tools MCP de revisión (028)

Delta sobre `specs/027-adversarial-consensus-review/contracts/mcp-tools.md`. Lo que no
aparece aquí no cambia. Todos los nombres de campo son `snake_case`, tanto en la
entrada como en la salida: la respuesta se serializa desde un DTO del adaptador con
etiquetas JSON explícitas, nunca desde un struct de la capa de aplicación sin
etiquetar.

**Invariante transversal (FR-015, FR-016)**: toda tool que modifique una revisión
rechaza la operación si el estado es `approved`, `escalated` o `incomplete`, con el
error `la revisión está en estado terminal <estado> y no admite cambios`. El ledger no
se toca.

---

## `review_start`

**Entrada** — campos nuevos:

| Campo | Tipo | Notas |
|---|---|---|
| `fix_authorized` | boolean, opcional | `false` inicia una revisión de solo lectura. Si se omite se toma de la política del proyecto (`review_fix_authorized`, por defecto `true`) |

`max_fix_rounds` y `auto_fix_severities` siguen siendo opcionales, pero cuando se
omiten **ya no caen a un valor hardcodeado**: se resuelven desde la política del
proyecto y solo después a los defectos del dominio (FR-017).

**Salida** — campos nuevos: `fix_authorized`, `max_fix_rounds`,
`auto_fix_severities`. La revisión devuelve la política **efectiva** con la que quedó
congelada, para que el agente no tenga que adivinarla.

---

## `review_submit`

**Validaciones nuevas**

- Si la revisión declaró identidad esperada para ese revisor, `provider` y `model`
  deben coincidir. Error: `el resultado declara <provider>/<model> pero el revisor
  <A|B> se asignó a <esperado>` (FR-006).
- Todo hallazgo debe traer `local_id`, `location`, `severity`, `category`, `claim`,
  `evidence_class`, `confidence` y al menos un elemento no vacío en `evidence`. Error:
  `el hallazgo <local_id> omite el campo obligatorio <campo>` (FR-007).
- `target_digest` se valida contra el **target vigente** de la revisión: el original
  mientras no haya correcciones, el corregido de la última ronda válida después
  (FR-011).

**Salida**: sin cambios (`stored`, `consensus_ready`, `finding_ids`).

---

## `review_consensus`

**Cambio de semántica**. La entrada sigue siendo `matches` y `unmatched`, pero ahora
describe la clasificación **completa** de la ronda, no una parte.

**Validaciones nuevas**

| Regla | Error |
|---|---|
| Todo hallazgo de la ronda activa aparece exactamente una vez (FR-001) | `quedan <n> hallazgos sin clasificar: <local_ids>` |
| Ningún ID se repite ni aparece en `matches` y `unmatched` a la vez (FR-002) | `el hallazgo <id> se clasifica más de una vez` |
| Ningún ID pertenece a otra ronda o a otra revisión (FR-002) | `el hallazgo <id> no pertenece a la ronda activa` |
| `severity` enviada distinta de la derivada (FR-003) | `la severidad declarada <X> no coincide con la derivada de las fuentes <Y>` |

**Campo `severity`**: pasa a ser opcional e informativo. La severidad persistida es
siempre el máximo de las severidades de los hallazgos fuente. No existe forma de
degradarla.

**Idempotencia (FR-005)**: reenviar la clasificación exacta de una ronda ya registrada
devuelve lo persistido sin escribir, con `idempotent: true` en la salida. Enviar una
clasificación distinta para esa misma ronda se rechaza:
`la ronda <n> ya tiene un consenso registrado y no admite reemplazo`.

**Salida** — campos nuevos: `idempotent` (boolean). Los `consensus_local_id` son
estables: se asignan tras ordenar por el menor ID de hallazgo fuente, de modo que el
mismo conjunto produce siempre los mismos identificadores.

---

## `review_fix_record`

**Validaciones nuevas**

- La revisión debe tener `fix_authorized = true`. Error: `esta revisión es de solo
  lectura y no admite correcciones` (FR-018).
- `base_target_digest` debe ser el target vigente: el original en la ronda 1, el
  `fixed_target_digest` de la ronda anterior después. Error: `la corrección parte de
  <digest> pero el target vigente es <digest>` (FR-009).
- El registro es una transición indivisible: delta, `round`, `status` y
  `current_target_digest` se escriben en una sola transacción. Dos correcciones
  concurrentes para la misma ronda dejan **una** ganadora; la perdedora recibe
  `la ronda <n> ya fue registrada por otra corrección` y no sobrescribe nada (FR-010).

**Salida** — campos nuevos: `current_target_digest`, `round`.

---

## `review_rejudge`

**Cambio de forma de entrada.** El mapa plano `states: {consensus_local_id: state}` no
puede expresar quién emite el juicio, y FR-013 exige dos revisores independientes.

**Entrada nueva**:

```json
{
  "review_id": "acr_…",
  "reviewer":  "A",
  "judgments": {
    "C-001": { "state": "RESOLVED",   "evidence": ["go test ./domain -run TestCobertura"] },
    "C-002": { "state": "UNRESOLVED", "evidence": ["la ruta X sigue sin validarse"] }
  }
}
```

| Campo | Tipo | Notas |
|---|---|---|
| `reviewer` | `"A"` \| `"B"` | Obligatorio. Un revisor emite su propio re-juicio |
| `judgments[].state` | `RESOLVED` \| `UNRESOLVED` \| `REGRESSED` | |
| `judgments[].evidence` | array de string | Obligatorio y no vacío; se redacta antes de persistir (FR-027) |

**Validaciones nuevas**

- El hallazgo debe ser `CONFIRMED` (ya vigente) **y** estar incluido en
  `addressed_consensus_ids` de la corrección vigente. Error: `el hallazgo <id> no
  forma parte de la corrección de la ronda <n>` (FR-013).
- Debe existir una corrección registrada (ya vigente).

**Agregación (FR-014)**: `REGRESSED` de cualquier revisor manda; `RESOLVED` requiere a
los dos; cualquier otro caso queda `UNRESOLVED`. Con un solo re-juicio recibido, el
estado agregado es `UNRESOLVED` — nunca `RESOLVED`.

**Salida**:

```json
{
  "rejudged": [
    { "consensus_local_id": "C-001",
      "reviewer_states": { "A": "RESOLVED", "B": "RESOLVED" },
      "aggregate_state": "RESOLVED" }
  ]
}
```

---

## `review_status`

**Salida ampliada** (FR-022, FR-023). Hoy devuelve cuatro campos; el contrato 027
promete "resumen de hallazgos por estado" y la auditoría de SC-006 exige reconstruir
el linaje completo con **una sola consulta**.

```json
{
  "review_id": "acr_…",
  "status": "consensus_ready",
  "round": 0,
  "verdict": null,
  "fix_authorized": true,
  "target": {
    "type": "diff",
    "revision": "working-tree",
    "original_digest": "e7114e8c…",
    "current_digest": "e7114e8c…"
  },
  "policy": { "max_fix_rounds": 2, "auto_fix_severities": ["CRITICAL", "HIGH"] },
  "reviewers": [
    { "reviewer": "A", "expected": "…/…", "status": "success", "findings": 3 },
    { "reviewer": "B", "expected": "…/…", "status": "success", "findings": 2 }
  ],
  "counts": {
    "by_status":     { "CONFIRMED": 2, "SUSPECT": 1, "CONTRADICTION": 0, "INFO": 1 },
    "by_severity":   { "CRITICAL": 0, "HIGH": 2, "MEDIUM": 1, "LOW": 1, "INFO": 0 },
    "by_rejudgment": { "RESOLVED": 0, "UNRESOLVED": 2, "REGRESSED": 0, "PENDING": 2 }
  },
  "findings": [
    { "consensus_local_id": "C-001", "status": "CONFIRMED", "severity": "HIGH",
      "round": 0, "source_finding_ids": [12, 27],
      "addressed_by_round": 1,
      "rejudgments": [ { "reviewer": "A", "state": "RESOLVED" } ],
      "aggregate_state": "UNRESOLVED" }
  ],
  "fix_rounds": [
    { "round": 1, "base_target_digest": "e7114e8c…", "fixed_target_digest": "a91f…",
      "addressed_consensus_ids": ["C-001"], "modified_paths": ["domain/verdict.go"] }
  ]
}
```

Sigue siendo estrictamente de solo lectura: no ejecuta ninguna transición. Debe
responder en menos de 2 s con 1.000 hallazgos (SC-008).

---

## `review_finalize`

**Salida corregida** (FR-024). Los ocho campos del contrato publicado, en `snake_case`:

```json
{
  "review_id": "acr_…",
  "verdict": "ESCALATED",
  "promotable_findings": [],
  "metrics": {
    "duration": 412,
    "findings_total": 4,
    "findings_confirmed": 2,
    "findings_suspect": 1,
    "contradictions": 0,
    "fix_rounds": 1,
    "memory_promoted": 0,
    "memory_deduplicated": 0
  }
}
```

`duration` va en segundos enteros, medido de `created_at` a `updated_at`.

**Comportamiento nuevo (FR-019)**: una revisión con `fix_authorized = false` y un
hallazgo `CONFIRMED` severo sin resolver finaliza `ESCALATED` en **una sola llamada**,
en vez de devolver `review is not ready to finalize` y quedarse bloqueada. Con
`fix_authorized = true` y rondas disponibles el comportamiento no cambia: sigue
devolviendo que la revisión no está lista, porque queda corrección por hacer.

---

## `review_promote_memory`

**Validación nueva (FR-021)**: la revisión debe tener veredicto `APPROVED`. Error:
`la revisión no está aprobada (<verdict>): su aprendizaje todavía no es conocimiento
del proyecto`. Las comprobaciones vigentes —hallazgo `CONFIRMED` y `RESOLVED`— se
mantienen.

**Salida** — campos nuevos: `deduplicated` (cuántas promociones reforzaron una memoria
existente por `topic_key` en vez de crear una nueva), que alimenta la métrica
`memory_deduplicated` de `review_finalize`.

---

## Exclusiones deliberadas (heredadas de 027, sin cambios)

Ninguna tool acepta ni persiste prompts en bruto, cadenas de razonamiento, transcripts
de revisor, secretos ni credenciales. Los campos de texto libre nuevos —`evidence` de
re-juicio incluido— pasan por el mismo helper de redacción que ya protege `claim` y
`verification` (`redactarTexto`). Ninguna tool acepta un `verdict` como entrada.
