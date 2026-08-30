# Fase 1 — Modelo de Datos: Fortalecimiento de la revisión ACR

Este documento describe el ledger de revisión **después** de la funcionalidad 028.
Todo cambio es aditivo: las revisiones ya persistidas se siguen leyendo sin migración
destructiva. Las columnas nuevas se añaden con `addColumnIfMissing` y la tabla nueva
con `CREATE TABLE IF NOT EXISTS`, siguiendo el patrón vigente en
`adapters/secondary/persistence/db.go`.

Convención: **[nuevo]** marca lo que introduce esta funcionalidad; el resto ya existe.

---

## 1. Revisión (`reviews` → `domain.Review`)

Ciclo adversarial completo. Conserva identidad, política, revisores esperados, ronda,
estado y veredicto.

| Campo | Tipo | Notas |
|---|---|---|
| `review_id` | TEXT | `acr_<uuid>`; único por proyecto |
| `target_type`, `target_revision`, `target_digest`, `target_scope` | TEXT | Target **original** congelado al iniciar |
| `current_target_digest` **[nuevo]** | TEXT | Target **vigente**: el original hasta la primera corrección, luego el corregido por la última ronda válida (FR-008) |
| `max_fix_rounds`, `auto_fix_severities` | INTEGER / TEXT | Política efectiva de la revisión |
| `fix_authorized` **[nuevo]** | INTEGER (0/1) | `0` = revisión de solo lectura (FR-018). Por defecto `1`, para no alterar revisiones existentes |
| `reviewer_a_provider`, `reviewer_a_model` **[nuevo]** | TEXT | Identidad **esperada** del revisor A (FR-006) |
| `reviewer_b_provider`, `reviewer_b_model` **[nuevo]** | TEXT | Identidad esperada del revisor B |
| `independence_level`, `independence_reason` | TEXT | `full` \| `degraded` |
| `round`, `status`, `verdict` | INTEGER / TEXT | Estado del protocolo |
| `created_at`, `updated_at` | TEXT | UTC-5; su diferencia es la métrica `duration` |

**Reglas de validación**
- `current_target_digest` nunca queda vacío: al crear la revisión se inicializa con
  `target_digest`.
- `fix_authorized = 0` implica que ninguna corrección puede registrarse, sea cual sea
  `max_fix_rounds`.
- Las identidades esperadas pueden estar vacías (revisión sin declararlas); si están
  presentes, todo resultado debe coincidir.

### Transiciones de estado

La máquina ya está en `domain.ReviewStatus.CanTransitionTo`. Lo que 028 añade es
**obligar a usarla**: se expone `Review.TransitionTo(next) error` y ningún caso de uso
asigna `Status` directamente (FR-015).

```
frozen ──▶ awaiting_reviewers ──▶ consensus_ready ──┬──▶ fixing ──▶ rejudging ──┐
                    │                               │                    │      │
                    ▼                               ├──▶ approved        ├──────┘
              incomplete ◀──────────────────────────┼──▶ escalated       │
                                                    └──▶ incomplete ◀────┘
```

`approved`, `escalated` e `incomplete` son **terminales e inmutables** (FR-016):
cualquier `submit`, `consensus`, `fix_record`, `rejudge` o `promote` sobre ellos se
rechaza sin tocar el ledger.

---

## 2. Resultado de revisor (`reviewer_results` → `domain.ReviewerResult`)

Sin cambios de esquema. Cambia su validación: si la revisión declara identidad
esperada para ese revisor, `provider` y `model` deben coincidir (FR-006), y un
resultado que llega después de una corrección se valida contra
`current_target_digest`, no contra el original (FR-011).

`UNIQUE(review_id, reviewer, round)` ya impide dos resultados del mismo revisor en la
misma ronda. Un resultado `failure` es final para su ronda.

---

## 3. Hallazgo fuente (`findings` → `domain.Finding`)

Sin cambios de esquema. Se endurece la validación de entrada (FR-007): `local_id`,
`location`, `severity`, `category`, `claim`, `evidence_class`, `evidence` (al menos un
elemento no vacío) y `confidence` son obligatorios. Hoy la tabla permite cadena vacía
por defecto en varios de ellos y `Finding.Confirmable()` solo se consulta al
confirmar; la validación pasa al momento de recibir el resultado, que es el borde del
sistema.

---

## 4. Clasificación de consenso (`consensus_findings` → `domain.ConsensusFinding`)

| Campo | Tipo | Notas |
|---|---|---|
| `consensus_local_id` | TEXT | `C-001`, `C-002`… Asignado tras ordenar por el menor ID de hallazgo fuente, no por orden de llegada (FR-005) |
| `status` | TEXT | `CONFIRMED` \| `SUSPECT` \| `CONTRADICTION` \| `INFO` |
| `severity` | TEXT | **Derivada** de las fuentes: el máximo de sus severidades (FR-003). Ya no se acepta del llamador |
| `claim` | TEXT | Redactado con `redactarTexto` |
| `source_finding_ids` | TEXT (JSON) | Uno o dos IDs. Cada ID de la ronda aparece en **exactamente una** clasificación (FR-001, FR-002) |
| `rejudgment_state` | TEXT | **Pasa a ser derivado** de la tabla `rejudgments`; no se acepta del llamador |
| `round_fingerprint` **[nuevo]** | TEXT | Huella determinista de la clasificación completa de la ronda; habilita la idempotencia de FR-005 |

**Reglas de validación**
- `CONFIRMED` y `CONTRADICTION` exigen exactamente dos fuentes, de revisores distintos.
- `SUSPECT` e `INFO` exigen exactamente una fuente.
- `CONFIRMED` exige que ambas fuentes sean `Confirmable()`.
- Reenviar una ronda con la misma huella es una operación de lectura; con huella
  distinta se rechaza.

### Orden de severidad

```
CRITICAL > HIGH > MEDIUM > LOW > INFO
```

`CRITICAL` y `HIGH` son las severas (`Severity.Severe()`, ya en `domain/finding.go`) y
son las que bloquean la aprobación.

---

## 5. Delta de corrección (`fix_rounds` → `domain.FixDelta`)

Sin cambios de esquema; cambia cómo se escribe y cómo se valida la cadena.

| Campo | Notas |
|---|---|
| `round` | Derivado, nunca de entrada. `UNIQUE(review_id, round)` |
| `base_target_digest` | La ronda 1 debe partir del `target_digest` original; la ronda N del `fixed_target_digest` de la ronda N-1 (FR-009) |
| `fixed_target_digest` | Distinto del base: una corrección que no cambia nada se rechaza |
| `addressed_consensus_ids` | Al menos uno, todos `CONFIRMED` y autorizados por la política |
| `modified_paths`, `verification`, `diff_digest` | Evidencia; `verification` se redacta |

**Escritura transaccional (FR-010)**: lectura de rondas existentes, derivación del
número de ronda, inserción del delta y actualización de `reviews.round`,
`reviews.status` y `reviews.current_target_digest` ocurren dentro de una única
transacción abierta con `BEGIN IMMEDIATE`. El `UNIQUE(review_id, round)` es la red de
seguridad final.

---

## 6. Re-juicio **[nueva tabla]** (`rejudgments` → `domain.ReJudgment`)

Resultado independiente de **un** revisor sobre **un** hallazgo corregido en **una**
ronda. Es la entidad que hoy no existe y sin la cual FR-013 y FR-014 no se pueden
expresar.

```sql
CREATE TABLE IF NOT EXISTS rejudgments (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id             INTEGER NOT NULL,
    round                 INTEGER NOT NULL,
    consensus_finding_id  INTEGER NOT NULL,
    reviewer              TEXT    NOT NULL,           -- 'A' | 'B'
    state                 TEXT    NOT NULL,           -- RESOLVED | UNRESOLVED | REGRESSED
    evidence              TEXT    NOT NULL DEFAULT '[]',
    created_at            TEXT    NOT NULL DEFAULT (<Now>),
    FOREIGN KEY (review_id)            REFERENCES reviews(id),
    FOREIGN KEY (consensus_finding_id) REFERENCES consensus_findings(id),
    UNIQUE(review_id, round, consensus_finding_id, reviewer)
);
CREATE INDEX IF NOT EXISTS idx_rejudgments_finding
    ON rejudgments(consensus_finding_id);
```

**Reglas de validación**
- Solo se admite re-juicio sobre un hallazgo `CONFIRMED`.
- Solo se admite si la corrección vigente lo incluye en `addressed_consensus_ids`
  (FR-013). Un hallazgo que nadie corrigió no puede declararse resuelto.
- `evidence` se redacta como el resto del ledger (FR-027).

### Agregación (FR-014) — función pura en `domain/rejudgment.go`

| Estados de A y B | Estado agregado |
|---|---|
| Cualquiera `REGRESSED` | `REGRESSED` |
| Ambos `RESOLVED` | `RESOLVED` |
| Cualquier otro caso (incluido falta uno) | `UNRESOLVED` |

El resultado se escribe en `consensus_findings.rejudgment_state` en la misma
transacción que el re-juicio, de modo que la columna nunca contradice a la tabla.

---

## 7. Política de revisión (`Settings` → `domain.ReviewPolicy`)

| Campo | Origen | Defecto |
|---|---|---|
| `review_max_fix_rounds` | `Settings` (ya existe) | `domain.DefaultMaxFixRounds` = 2 |
| `review_auto_fix_severities` | `Settings` (ya existe) | `["CRITICAL","HIGH"]` |
| `review_fix_authorized` **[nuevo]** | `Settings` | `true` |

Precedencia (FR-017 y la suposición declarada en la spec): valores explícitos de la
revisión → política del proyecto → defectos del dominio. Hoy la política del proyecto
existe pero nadie la lee; 028 la conecta en `StartReview` y la persiste en la revisión,
de modo que un cambio posterior de `Settings` no altera revisiones ya iniciadas.

---

## 8. Métricas de revisión (`usecases.ReviewMetrics`)

No se persiste: se deriva del ledger al finalizar. Los ocho campos del contrato:

| Campo del contrato | Derivación |
|---|---|
| `duration` | `updated_at - created_at` en segundos |
| `findings_total` | Filas de `consensus_findings` de todas las rondas |
| `findings_confirmed` | …con `status = CONFIRMED` |
| `findings_suspect` | …con `status = SUSPECT` |
| `contradictions` | …con `status = CONTRADICTION` |
| `fix_rounds` | Filas de `fix_rounds` |
| `memory_promoted` **[nuevo]** | Memorias insertadas por `review_promote_memory` para esta revisión |
| `memory_deduplicated` **[nuevo]** | Promociones que reforzaron una memoria existente por `topic_key` en vez de crear una nueva |

---

## Diagrama de relaciones

```
reviews 1───n reviewer_results 1───n findings
   │                                    │
   │                                    │ (source_finding_ids, JSON)
   │                                    ▼
   ├──────────────────────────────► consensus_findings 1───n rejudgments  [nuevo]
   │                                    ▲
   └───────────► fix_rounds ────────────┘
                (addressed_consensus_ids, JSON)
```

## Resumen de la migración

| Objeto | Operación | Idempotente |
|---|---|---|
| `reviews.current_target_digest` | `addColumnIfMissing` TEXT | Sí |
| `reviews.fix_authorized` | `addColumnIfMissing` INTEGER | Sí |
| `reviews.reviewer_a_provider` / `_model` | `addColumnIfMissing` TEXT | Sí |
| `reviews.reviewer_b_provider` / `_model` | `addColumnIfMissing` TEXT | Sí |
| `consensus_findings.round_fingerprint` | `addColumnIfMissing` TEXT | Sí |
| `rejudgments` | `CREATE TABLE IF NOT EXISTS` | Sí |
| `idx_rejudgments_finding` | `CREATE INDEX IF NOT EXISTS` | Sí |

Ninguna columna se borra, ninguna se vuelve `NOT NULL` sobre datos existentes y
ningún `DROP` forma parte de esta funcionalidad. Una base anterior a 028 abre, migra y
sigue funcionando: `current_target_digest` vacío se interpreta como "igual al target
original" y `fix_authorized` nulo como `1`.
