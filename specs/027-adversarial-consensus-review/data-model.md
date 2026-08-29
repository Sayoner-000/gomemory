# Modelo de datos: Revisión Adversarial por Consenso

Todas las tablas nuevas siguen la convención ya vigente en `adapters/secondary/persistence/db.go`: `project TEXT NOT NULL` en cada fila, claves autoincrementales, `CREATE TABLE IF NOT EXISTS` aditivo dentro de `migrate()`, timestamps `TEXT NOT NULL DEFAULT (Now)`. Ninguna tabla existente se modifica de forma incompatible; solo se añade una columna nullable a `memories`.

## Entidades de dominio (puras, en `domain/`)

### Target

Representa el artefacto congelado sometido a revisión.

| Campo | Tipo | Notas |
|---|---|---|
| `Type` | enum | `commit`, `diff`, `file_set`, `spec`, `plan`, `config`, `architecture`, `migration`, `api_contract`, `implementation` |
| `Revision` | string | identificador legible (SHA de git, revisión de documento) |
| `Digest` | string | SHA-256 u origen equivalente; identidad inmutable real |
| `Scope` | []string | rutas o límites del target |
| `CreatedAt` | timestamp | momento del congelamiento |

**Regla de validación**: `Digest` no puede estar vacío; una vez creado, es inmutable — cualquier operación posterior que reciba un `Digest` distinto para la misma revisión se rechaza (FR-004).

### Review

| Campo | Tipo | Notas |
|---|---|---|
| `ID` | string (`acr_<ULID>`) | identificador público de la revisión |
| `Target` | Target | ver arriba |
| `MaxFixRounds` | int | default 2, configurable por proyecto |
| `AutoFixSeverities` | []Severity | default `[CRITICAL, HIGH]` |
| `IndependenceLevel` | enum | `full`, `degraded` |
| `IndependenceReason` | string | vacío si `full`; ej. `same-model` si `degraded` |
| `Round` | int | ronda de corrección activa (0 = inicial) |
| `Status` | enum | `frozen`, `awaiting_reviewers`, `consensus_ready`, `fixing`, `rejudging`, `approved`, `escalated`, `incomplete` |
| `Verdict` | enum, nullable | solo se llena en estado terminal |

**Transiciones de estado** (aplicadas por `application/usecases`, nunca saltables desde fuera):

```text
frozen → awaiting_reviewers → consensus_ready ──(sin CRITICAL/HIGH confirmados)──▶ approved
                                     │
                                     ▼ (hay CRITICAL/HIGH confirmados y round < max)
                                   fixing → rejudging ──(resuelto)──▶ approved
                                     │                 └─(sin resolver y round == max)─▶ escalated
                                     └─(round == max al llegar aquí)──────────────────▶ escalated
awaiting_reviewers / rejudging ──(falla un revisor)──▶ incomplete
```

### ReviewerResult

| Campo | Tipo | Notas |
|---|---|---|
| `Reviewer` | enum | `A`, `B` |
| `Round` | int | 0 = inicial, N = ronda de re-revisión |
| `Provider` / `Model` | string, opcional | declarados por el llamador |
| `Status` | enum | `success`, `failure` |

**Regla de validación**: a lo sumo un `ReviewerResult` por `(review, reviewer, round)` — reenvíos son idempotentes (upsert), nunca duplican (FR-039).

### Finding

| Campo | Tipo | Notas |
|---|---|---|
| `LocalID` | string | ej. `A-001`, único dentro de su `ReviewerResult` |
| `Location` | string | ruta/rango del target |
| `Severity` | enum | `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `INFO` |
| `Category` | string | libre (ej. `concurrency`) |
| `Claim` | string | afirmación del defecto |
| `EvidenceClass` | enum | `deterministic`, `reproduced`, `contract`, `static-analysis`, `test-failure`, `runtime-observation`, `probabilistic` |
| `Evidence` | []string | enunciados concretos de soporte |
| `Confidence` | enum/float | declarado por el revisor |

**Regla de validación**: un `Finding` sin al menos un elemento en `Evidence` no puede ser fuente de un `ConsensusFinding` con estado `CONFIRMED` (FR-013).

### ConsensusFinding

| Campo | Tipo | Notas |
|---|---|---|
| `ConsensusLocalID` | string | ej. `C-001` |
| `Status` | enum | `CONFIRMED`, `SUSPECT`, `CONTRADICTION`, `INFO` |
| `Severity` | enum | heredada de las fuentes |
| `SourceFindingIDs` | []int | 2 IDs si `CONFIRMED`/`CONTRADICTION`, 1 si `SUSPECT` |
| `RejudgmentState` | enum, nullable | `RESOLVED`, `UNRESOLVED`, `REGRESSED` — se llena tras cada ronda de re-revisión |

**Reglas de validación** (impuestas por `build_consensus`, INV-004/INV-005):
- `CONFIRMED` exige exactamente 2 `SourceFindingIDs`, provenientes de `ReviewerResult` distintos (`A` y `B`) de la misma ronda del mismo `Review`.
- `SUSPECT` exige exactamente 1 `SourceFindingID` y nunca puede pasar a ser `CONFIRMED` retroactivamente sin un segundo hallazgo independiente real.

### FixDelta (ronda de corrección)

| Campo | Tipo | Notas |
|---|---|---|
| `Round` | int | 1..`MaxFixRounds` |
| `BaseTargetDigest` / `FixedTargetDigest` | string | antes/después de la corrección |
| `AddressedConsensusIDs` | []string | solo `ConsensusFinding` con `Status=CONFIRMED` y severidad dentro de `AutoFixSeverities`, salvo autorización explícita |
| `ModifiedPaths` | []string | alcance real de la corrección |
| `Verification` | []string | comandos/resultados de verificación reportados |
| `DiffDigest` | string | huella del cambio exacto |

**Regla de validación**: `record_fix` rechaza la operación si algún `ConsensusFinding` referenciado no es `CONFIRMED`, si su severidad no está en `AutoFixSeverities` sin bandera de autorización explícita, o si `Round` excede `MaxFixRounds` (INV-006, INV-009).

### Verdict

Enum terminal: `APPROVED`, `ESCALATED`, `INCOMPLETE`. Calculado por `domain.DeriveVerdict` (ver `research.md`, Decisión 5) — nunca asignado directamente por un adaptador.

## Esquema SQLite (adición a `migrate()` en `db.go`)

```sql
CREATE TABLE IF NOT EXISTS reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project TEXT NOT NULL,
    review_id TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_revision TEXT NOT NULL,
    target_digest TEXT NOT NULL,
    target_scope TEXT NOT NULL DEFAULT '[]',
    max_fix_rounds INTEGER NOT NULL DEFAULT 2,
    auto_fix_severities TEXT NOT NULL DEFAULT '["CRITICAL","HIGH"]',
    independence_level TEXT NOT NULL DEFAULT 'degraded',
    independence_reason TEXT NOT NULL DEFAULT '',
    round INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'frozen',
    verdict TEXT,
    created_at TEXT NOT NULL DEFAULT (Now),
    updated_at TEXT NOT NULL DEFAULT (Now),
    UNIQUE(project, review_id)
);
CREATE INDEX IF NOT EXISTS idx_reviews_project ON reviews(project);
CREATE INDEX IF NOT EXISTS idx_reviews_status ON reviews(project, status);

CREATE TABLE IF NOT EXISTS reviewer_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id INTEGER NOT NULL,
    reviewer TEXT NOT NULL,
    round INTEGER NOT NULL DEFAULT 0,
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    submitted_at TEXT NOT NULL DEFAULT (Now),
    FOREIGN KEY (review_id) REFERENCES reviews(id),
    UNIQUE(review_id, reviewer, round)
);

CREATE TABLE IF NOT EXISTS findings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reviewer_result_id INTEGER NOT NULL,
    local_id TEXT NOT NULL,
    location TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT '',
    claim TEXT NOT NULL,
    evidence_class TEXT NOT NULL,
    evidence TEXT NOT NULL DEFAULT '[]',
    confidence TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (reviewer_result_id) REFERENCES reviewer_results(id),
    UNIQUE(reviewer_result_id, local_id)
);

CREATE TABLE IF NOT EXISTS consensus_findings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id INTEGER NOT NULL,
    round INTEGER NOT NULL DEFAULT 0,
    consensus_local_id TEXT NOT NULL,
    status TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT '',
    claim TEXT NOT NULL DEFAULT '',
    source_finding_ids TEXT NOT NULL DEFAULT '[]',
    rejudgment_state TEXT,
    created_at TEXT NOT NULL DEFAULT (Now),
    FOREIGN KEY (review_id) REFERENCES reviews(id),
    UNIQUE(review_id, consensus_local_id)
);
CREATE INDEX IF NOT EXISTS idx_consensus_review ON consensus_findings(review_id);

CREATE TABLE IF NOT EXISTS fix_rounds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    review_id INTEGER NOT NULL,
    round INTEGER NOT NULL,
    base_target_digest TEXT NOT NULL,
    fixed_target_digest TEXT NOT NULL,
    addressed_consensus_ids TEXT NOT NULL DEFAULT '[]',
    modified_paths TEXT NOT NULL DEFAULT '[]',
    verification TEXT NOT NULL DEFAULT '[]',
    diff_digest TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (Now),
    FOREIGN KEY (review_id) REFERENCES reviews(id),
    UNIQUE(review_id, round)
);
```

**Columna aditiva** (vía `addColumnIfMissing`, sin migración destructiva):

```sql
ALTER TABLE memories ADD COLUMN source_review_id INTEGER;
```

Usada por `promote_review_memory` para enlazar la memoria promovida con su `review_id` de origen (trazabilidad de linaje, FR-038), sin crear una tabla de memoria paralela (ver `research.md`, Decisión 4).

## Linaje de revisión (trazabilidad, FR-038)

```text
reviews (1) ──< reviewer_results (N, por ronda) ──< findings (N)
   │
   ├──< consensus_findings (N, por ronda) ──> source_finding_ids [findings.id, findings.id]
   ├──< fix_rounds (N) ──> addressed_consensus_ids [consensus_findings.consensus_local_id]
   └──> memories.source_review_id (0..N, promoción de conocimiento)
```

Cualquier fila puede recorrerse hacia `reviews.review_id` para reconstruir qué se revisó, qué se detectó, qué corrección se intentó y cuál fue el resultado, sin acceso al almacenamiento interno (Historia de usuario 4).
