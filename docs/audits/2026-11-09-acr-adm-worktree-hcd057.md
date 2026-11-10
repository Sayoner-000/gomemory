# Informe ACR + ADM — Worktree HCD-057

**Fecha:** 2026-11-09  
**Revisión base (`HEAD`):** `8213859d228be5b6744e410b1b5816d5933414e3`  
**Árbol de trabajo auditado:** `8213859d228be5b6744e410b1b5816d5933414e3+HCD-057` (`dirty: true`, 43 archivos: 20 modificados + 23 nuevos)  
**Target Digest:** `ad9e9510849b4a29a28d7ff4d7c3ff9d74b8ef794138f134dd36fc471b1e22e5`  
**Identificador de revisión ACR:** `acr_cfdfdc49-3177-4be1-9bb8-d666cdfeb2f8`  
**Veredicto Final ACR:** **`APPROVED`** (`0 CRITICAL`, `0 HIGH`, `2 MEDIUM CONFIRMED`, `4 LOW SUSPECT`, `0 CONTRADICTIONS`)  
**Alcance:** Unidad **HCD-057** («Integrar el examen socrático en HCD y controlar la decisión de revisión splash») a través de `domain/`, `application/ports/`, `application/usecases/`, `adapters/secondary/persistence/`, `adapters/primary/cli/`, `adapters/primary/setup/`, `infrastructure/`, `docs/` y `tests/`.

---

## 1. Resumen de salud del código

- **Lenguajes / tecnologías detectadas:**
  - **Producción:** Go (`domain`, `application/ports`, `application/usecases`, `adapters/secondary/persistence`, `adapters/primary/cli`, `adapters/primary/cli/hcd`, `adapters/primary/setup`, `infrastructure`), SQL / SQLite (esquema v13 en `hcd_schema.go`), Markdown / Skills (`infrastructure/templates/hcd/*/SKILL.md`, `docs/MANUAL.md`, `docs/DATABASE.md`), JavaScript / MJS (`infrastructure/plugin/opencode`), TypeScript (`installer/`).
  - **Pruebas:** Go (`*_test.go` en `domain`, `application/usecases`, `adapters/secondary/persistence`, `adapters/primary/cli`, `adapters/primary/setup`, `infrastructure`, `tests/contract`, `tests/integration`), Node test runner (`*.test.mjs`).
- **Nivel de riesgo general:** **Medio** (0 hallazgos críticos o altos; 2 hallazgos medios confirmados con cierre directo).
- **Evaluación breve:**
  La arquitectura e implementación de **HCD-057** presentan alta solidez estructural y apego a las reglas de arquitectura hexagonal (`TestLayerRules_CodigoDeProduccion` pasa sin nuevas excepciones). La persistencia en `adapters/secondary/persistence/hcd_definition_review.go` emplea consultas SQL 100 % parametrizadas, aislamiento estricto por `(project, unit_id)`, control de concurrencia optimista (CAS sobre `expectedLast` dentro de transacciones inmediatas `_txlock=immediate`), idempotencia exacta por `(project, operation_id)` y redacción preventiva de etiquetas `<private>` y credenciales mediante `event.Protected()` antes de escribir en disco. Asimismo, los cierres previos de la sesión (`C-001` sobre precedencia del último evento de ciclo de vida en `SplashSatisfied`/`DefinitionReviewSummary` y `C-002` en ceremonias de integración) se verificaron vigentes y cubiertos por pruebas.
- **Resultado:** **5 hallazgos ADM / 6 entradas en el ledger ACR** (`0 Críticos`, `0 Altos`, `2 Medios CONFIRMADOS`, `3 Bajos`), sin tocar archivos de producción durante la auditoría (modo solo lectura).
- **Declaración de límites:** La auditoría abarcó la totalidad de los 43 archivos del worktree de HCD-057, sus llamadas e interacciones en el grafo de código y la suite mecánica del repositorio. No sustituye pruebas en entornos Windows reales para el job `windows` de GitHub Actions.

### Cobertura y validación mecánica

| Comprobación | Resultado |
|---|---|
| `gofmt -l .` | **Falla (H-01 / C-001)** — 5 archivos de HCD-057 sin formatear (`adapters/primary/cli/hcd/execute.go`, `adapters/primary/setup/hcd_skills_setup_test.go`, `domain/hcd_definition_review.go`, `domain/hcd_stage.go`, `domain/hcd_stage_socratic_test.go`). |
| `go vet ./...` | **Pasa** (`0` advertencias). |
| `git diff --check` | **Pasa** (`0` errores de whitespace o conflictos). |
| `go test ./tests/contract -run TestLayerRules -count=1` | **Pasa** (`0` violaciones de capas L1–L3). |
| `go test -race ./domain ./application/usecases ./adapters/secondary/persistence ./adapters/primary/cli/hcd ./adapters/primary/setup ./infrastructure ./tests/integration -run 'TestHCD\|TestLayerRules' -count=1` | **Pasa** (`0` data races, `7/7` paquetes verdes). |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | **Pasa** (`68/68` tests verdes). |
| `npm --prefix installer run typecheck && npm --prefix installer test` | **Pasa** (`0` errores TS, `24/24` tests verdes). |
| `bash -n scripts/install.sh scripts/coverage.sh` | **Pasa** (sintaxis POSIX/Bash válida). |

---

## 2. Consenso Adversarial (ACR)

```
ADVERSARIAL CONSENSUS REVIEW

Review ID:     acr_cfdfdc49-3177-4be1-9bb8-d666cdfeb2f8
Target:        worktree (43 archivos modificados/nuevos de HCD-057)
Revision:      8213859d228be5b6744e410b1b5816d5933414e3+HCD-057
Digest:        ad9e9510849b4a29a28d7ff4d7c3ff9d74b8ef794138f134dd36fc471b1e22e5
Independence:  degraded (same-model: google-antigravity/gemini-3.1-pro, pasadas independientes A y B)
Review rounds: 1
Fix rounds:    0 (modo auditoría read-only)

Confirmed (2):
  - C-001 [MEDIUM] (A-001 + B-001): 5 archivos de HCD-057 sin gofmt rompen el paso gofmt de CI (.github/workflows/test.yml).
  - C-002 [MEDIUM] (A-002 + B-002): ValidateTransition en domain/hcd_definition_review.go evalúa complete/fail/unavailable y dismiss sobre todo splashEvents de la revisión sin acotar al último accept vigente.

Suspect (4):
  - C-003 [LOW] (A-003): Alcance de dismiss en ValidateTransition frente a múltiples intentos complete en la misma revisión (subsumido en H-02 / C-002).
  - C-004 [LOW] (A-004): helpReference() en adapters/primary/cli/cli.go:243-253 omite la línea de 'review' dentro del bloque detallado de 'mem hcd'.
  - C-005 [LOW] (B-003): Lectura de --input en adapters/primary/cli/hcd/execute.go:137-152 sin io.LimitReader ni verificación de trailing tokens tras json.NewDecoder.Decode.
  - C-006 [LOW] (B-004): Tabla hcd_definition_reviews no incluida en readSanitizeCandidates (adapters/secondary/persistence/privacy_maintenance.go) para barridos históricos de mem sanitize.

Contradictions: 0
VERDICT: APPROVED
```

---

## 3. Hallazgos y fugas detectadas (ADM + ACR)

| ID | ACR Ledger | Severidad | Estado | Problema |
|---|---|---|---|---|
| **H-01** | `C-001` (`A-001` + `B-001`) | **Media** | **CONFIRMADO** | 5 archivos de HCD-057 no cumplen `gofmt`, provocando fallo inmediato en el job `core` de CI (`.github/workflows/test.yml`). |
| **H-02** | `C-002` / `C-003` (`A-002`, `A-003`, `B-002`) | **Media** | **CONFIRMADO** | `ValidateTransition` (`domain/hcd_definition_review.go:165-214`) evalúa `complete`/`fail`/`unavailable` y `dismiss` contra todo el historial `splashEvents` de la revisión en lugar de acotarse al último `accept` vigente, impidiendo completar o descartar tras una secuencia `skip → accept → start`. |
| **H-03** | `C-005` (`B-003`) | **Baja** | **CONFIRMADO** | `executeWithStdin` (`adapters/primary/cli/hcd/execute.go:137-152`) lee `--input` sin límite de bytes (`io.LimitReader`) y no comprueba tokens sobrantes tras el primer objeto JSON. |
| **H-04** | `C-006` (`B-004`) | **Baja** | **CONFIRMADO** | `readSanitizeCandidates` (`adapters/secondary/persistence/privacy_maintenance.go:131-137`) no incluye `hcd_definition_reviews.payload` en los barridos retrospectivos de `mem sanitize`. |
| **H-05** | `C-004` (`A-004`) | **Baja** | **CONFIRMADO** | `helpReference()` (`adapters/primary/cli/cli.go:243-253`) no lista el subcomando `review` en el bloque detallado de `mem hcd`. |

---

#### H-01 — Cinco archivos de HCD-057 sin formatear con `gofmt` rompen la tubería de CI (`C-001`)

- **Categoría:** Integridad / CI
- **Severidad:** Media
- **Estado:** CONFIRMADO
- **Ubicación:**
  - `adapters/primary/cli/hcd/execute.go:184-191`
  - `adapters/primary/setup/hcd_skills_setup_test.go:128-134`
  - `domain/hcd_definition_review.go:134-136`
  - `domain/hcd_stage.go:23-29`
  - `domain/hcd_stage_socratic_test.go:21-25`
- **Ruta de alcance:** Ejecución del workflow `.github/workflows/test.yml` (job `core`, paso `gofmt`, líneas 25-33) en cualquier `push` o `pull_request`.
- **Análisis de vulnerabilidad:** El primer paso de verificación en `.github/workflows/test.yml` ejecuta `unformatted="$(gofmt -l .)"` y aborta con `exit 1` si la salida no es vacía. Actualmente, cinco archivos modificados o creados en HCD-057 tienen desalineación de campos o líneas largas sin formatear.
- **Evidencia:**
  ```text
  $ gofmt -l .
  adapters/primary/cli/hcd/execute.go
  adapters/primary/setup/hcd_skills_setup_test.go
  domain/hcd_definition_review.go
  domain/hcd_stage.go
  domain/hcd_stage_socratic_test.go
  ```
- **Código corregido (seguro y defensivo):**
  Ejecutar `gofmt -w` sobre los cinco archivos afectados:
  ```bash
  gofmt -w adapters/primary/cli/hcd/execute.go \
    adapters/primary/setup/hcd_skills_setup_test.go \
    domain/hcd_definition_review.go \
    domain/hcd_stage.go \
    domain/hcd_stage_socratic_test.go
  ```
- **Validación del cierre:**
  `test -z "$(gofmt -l .)"` debe terminar con código de salida `0`.

---

#### H-02 — Desalineación de alcance en `ValidateTransition` tras `skip → accept → start` o múltiples intentos `complete` (`C-002` / `C-003`)

- **Categoría:** Error lógico / Consistencia de máquina de estados
- **Severidad:** Media
- **Estado:** CONFIRMADO
- **Ubicación:** `domain/hcd_definition_review.go:165-214` (`HCDDefinitionReviewEvent.ValidateTransition`)
- **Ruta de alcance:** `mem hcd review --kind splash --action complete|fail|unavailable|dismiss` → `executeWithStdin` (`adapters/primary/cli/hcd/execute.go:173`) → `(*HCD).RecordDefinitionReview` (`application/usecases/hcd_definition_review.go:58`) → `event.ValidateTransition(events)`.
- **Análisis de vulnerabilidad:**
  1. En `ValidateTransition`, la rama `case "start"` (líneas 155-163) busca la última decisión (`accept` o `skip`) y permite iniciar un intento si la última decisión es `"accept"`, incluso si antes hubo un `"skip"` en la misma revisión.
  2. Sin embargo, la rama `case "complete", "fail", "unavailable"` (líneas 165-190) recorre **todo** `splashEvents` desde el principio y rechaza con `"cannot be recorded after skip"` en cuanto encuentra cualquier `h.Action == "skip"` histórico (línea 172), aunque haya sido reemplazado por un `accept` y `start` posteriores. Además, si hubo un `accept → start(att-1) → skip → accept`, permitiría completar `att-1` perteneciente al `accept` anterior.
  3. De forma análoga, `case "dismiss"` (líneas 191-214) busca `h.Action == "complete"` y `h.Action == "dismiss"` en todo `splashEvents` en lugar de acotarse al último evento de ciclo de vida `complete` posterior al último `accept` (como sí hacen `SplashSatisfied` en `domain/hcd_stage.go:81-130` y `DefinitionReviewSummary` en `domain/hcd_stage.go:173-227`).
- **Evidencia:**
  En `domain/hcd_definition_review.go:171-183`:
  ```go
  for _, h := range splashEvents {
      if h.Action == "skip" {
          return invalid(e.Action + " cannot be recorded after skip")
      }
      if h.Action == "start" && h.Payload.AttemptID == e.Payload.AttemptID {
          hasStart = true
      }
      ...
  }
  ```
  Dada la secuencia válida `[skip("op-1"), accept("op-2"), start("op-3", AttemptID: "att-1")]`, las tres primeras transiciones son aceptadas por `ValidateTransition`, pero al enviar `complete("op-4", AttemptID: "att-1")` la línea 172 falla en el primer elemento (`skip("op-1")`), dejando el intento `att-1` bloqueado sin poder cerrarse con `complete`, `fail` ni `unavailable`.
- **Código corregido (seguro y defensivo):**
  Acotar `ValidateTransition` a los eventos posteriores a la última decisión (`accept`/`skip`), manteniendo simetría exacta con `SplashSatisfied`:
  ```go
  	lastDecision := ""
  	lastDecisionIdx := -1
  	for i, h := range splashEvents {
  		if h.Action == "accept" || h.Action == "skip" {
  			lastDecision = h.Action
  			lastDecisionIdx = i
  		}
  	}
  	switch e.Action {
  	case "accept", "skip":
  		return nil
  	case "start":
  		if lastDecision != "accept" {
  			return invalid("start requires active accept")
  		}
  		return nil
  	case "complete", "fail", "unavailable":
  		if lastDecision == "skip" {
  			return invalid(e.Action + " cannot be recorded after skip")
  		}
  		if lastDecision != "accept" {
  			return invalid(e.Action + " requires prior start")
  		}
  		postAccept := splashEvents[lastDecisionIdx+1:]
  		var hasStart, completedOrFailed bool
  		for _, h := range postAccept {
  			if h.Action == "start" && h.Payload.AttemptID == e.Payload.AttemptID {
  				hasStart = true
  			}
  			if (h.Action == "complete" || h.Action == "fail" || h.Action == "unavailable") && h.Payload.AttemptID == e.Payload.AttemptID {
  				if h.OperationID != e.OperationID {
  					completedOrFailed = true
  				}
  			}
  		}
  		if !hasStart {
  			return invalid(e.Action + " requires matching start for attempt " + e.Payload.AttemptID)
  		}
  		if completedOrFailed {
  			return invalid("attempt " + e.Payload.AttemptID + " is already finished")
  		}
  		return nil
  	case "dismiss":
  		if lastDecision != "accept" {
  			return invalid("dismiss requires an active completed splash review")
  		}
  		postAccept := splashEvents[lastDecisionIdx+1:]
  		lastLifecycleIdx := -1
  		for i := len(postAccept) - 1; i >= 0; i-- {
  			if postAccept[i].Action != "dismiss" {
  				lastLifecycleIdx = i
  				break
  			}
  		}
  		if lastLifecycleIdx == -1 || postAccept[lastLifecycleIdx].Action != "complete" {
  			return invalid("dismiss requires an existing Blocker finding with id " + e.Payload.FindingID)
  		}
  		var foundBlocker bool
  		for _, f := range postAccept[lastLifecycleIdx].Payload.Findings {
  			if f.ID == e.Payload.FindingID && f.Severity == "Blocker" {
  				foundBlocker = true
  				break
  			}
  		}
  		if !foundBlocker {
  			return invalid("dismiss requires an existing Blocker finding with id " + e.Payload.FindingID)
  		}
  		for _, h := range postAccept[lastLifecycleIdx+1:] {
  			if h.Action == "dismiss" && h.Payload.FindingID == e.Payload.FindingID && h.OperationID != e.OperationID {
  				return invalid("finding " + e.Payload.FindingID + " is already dismissed")
  			}
  		}
  		return nil
  ```
- **Validación del cierre:**
  Añadir en `TestHCD057SplashTransitions` (`domain/hcd_splash_transitions_test.go`) el caso `skip → accept → start(att-1) → complete(att-1) → dismiss(B1)` verificando que `ValidateTransition` lo acepta y que `complete` sigue rechazándose cuando `skip` es posterior a `start`.

---

#### H-03 — Lectura de `--input` sin cota de memoria ni verificación de datos sobrantes (`C-005`)

- **Categoría:** Programación defensiva / Validación de bordes
- **Severidad:** Baja
- **Estado:** CONFIRMADO
- **Ubicación:** `adapters/primary/cli/hcd/execute.go:137-152` (`executeWithStdin`)
- **Ruta de alcance:** CLI `mem hcd review --input <archivo|->`.
- **Análisis de vulnerabilidad:**
  1. `io.ReadAll(stdin)` y `os.ReadFile(inputRef)` leen la entrada completa en memoria sin cota máxima (p. ej. 1 MiB), exponiendo al proceso CLI a un consumo innecesario si se pasa por error un flujo o archivo grande.
  2. `dec.Decode(&payload)` decodifica el primer valor JSON pero no comprueba que el flujo termine en `io.EOF`, aceptando entradas concatenadas o con basura al final (`{"summary":"ok"}{"extra":1}`).
- **Evidencia:**
  En `adapters/primary/cli/hcd/execute.go:139-151`:
  ```go
  if inputRef == "-" {
      raw, err = io.ReadAll(stdin)
  } else {
      raw, err = os.ReadFile(inputRef)
  }
  ...
  dec := json.NewDecoder(bytes.NewReader(raw))
  dec.DisallowUnknownFields()
  if err := dec.Decode(&payload); err != nil {
      return emitError(out, errOut, p.json, "validation", fmt.Sprintf("entrada de examen inválida: %v", err))
  }
  ```
- **Código corregido (seguro y defensivo):**
  ```go
  const maxReviewInputBytes = 1 << 20 // 1 MiB

  func decodeStrictReviewPayload(r io.Reader) (domain.HCDDefinitionReviewPayload, error) {
  	var payload domain.HCDDefinitionReviewPayload
  	raw, err := io.ReadAll(io.LimitReader(r, maxReviewInputBytes+1))
  	if err != nil {
  		return payload, err
  	}
  	if len(raw) > maxReviewInputBytes {
  		return payload, fmt.Errorf("el archivo de entrada supera el límite de %d bytes", maxReviewInputBytes)
  	}
  	dec := json.NewDecoder(bytes.NewReader(raw))
  	dec.DisallowUnknownFields()
  	if err := dec.Decode(&payload); err != nil {
  		return payload, err
  	}
  	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
  		return payload, fmt.Errorf("contenido adicional inesperado tras el objeto JSON")
  	}
  	return payload, nil
  }
  ```
- **Validación del cierre:**
  Prueba unitaria en `adapters/primary/cli/hcd/execute_review_test.go` pasando `{"summary":"ok"} trailing` y verificando código de salida `5` (`validation`).

---

#### H-04 — Cobertura de `hcd_definition_reviews` en el saneamiento histórico (`mem sanitize`) (`C-006`)

- **Categoría:** Prevención de fugas / Mantenimiento de privacidad
- **Severidad:** Baja
- **Estado:** CONFIRMADO
- **Ubicación:** `adapters/secondary/persistence/privacy_maintenance.go:131-137` (`readSanitizeCandidates`)
- **Ruta de alcance:** CLI `mem sanitize [--apply]` → `(*SanitizeProject).Preview` / `Apply` → `(*PrivacyMaintenanceStore).Candidates`.
- **Análisis de vulnerabilidad:** Aunque `AppendReview` y `RecordDefinitionReview` invocan `event.Protected()` antes de insertar en `hcd_definition_reviews`, si una fila se insertara con una versión anterior de las reglas de redacción o mediante importación/manipulación directa de la base SQLite, `mem sanitize` no escanea actualmente la columna `payload` de `hcd_definition_reviews` (ni `quote`/`excerpt` de `hcd_approvals`/`hcd_evidence`).
- **Evidencia:**
  `readSanitizeCandidates` (`adapters/secondary/persistence/privacy_maintenance.go:45-166`) revisa `instinct_stats`, `sessions`, `memory_relations`, `channel_activity`, `compression_originals`, `socratica_*`, `reviews`/`findings` y `memories`, sin consultar `hcd_definition_reviews`.
- **Código corregido (seguro y defensivo):**
  Añadir una categoría `SanitizeHCDDefinitionReview` en `domain/privacy_maintenance.go` y su lector/aplicador en `adapters/secondary/persistence/privacy_maintenance.go` que deserialice `domain.HCDDefinitionReviewPayload`, aplique `Protected()` y compare el JSON canónico (siguiendo el mismo patrón de `readSocraticaCandidates`).
- **Validación del cierre:**
  Prueba en `adapters/secondary/persistence/` que inserte un payload crudo con `<private>SYNTH-TOKEN-0001</private>` en `hcd_definition_reviews`, ejecute `SanitizeProject.Apply` y compruebe que el token desaparece de la tabla y del WAL tras `Compact`.

---

#### H-05 — `helpReference()` no incluye la línea de `review` en el bloque detallado de `mem hcd` (`C-004`)

- **Categoría:** Mantenibilidad / Documentación CLI
- **Severidad:** Baja
- **Estado:** CONFIRMADO
- **Ubicación:** `adapters/primary/cli/cli.go:243-253` (`helpReference`)
- **Ruta de alcance:** CLI `mem help` o `mem --help`.
- **Análisis de vulnerabilidad:** El índice resumido `helpIndex` (`cli.go:92`) y la ayuda directa de `mem hcd` (`adapters/primary/cli/hcd/cmd_hcd.go:16`) sí documentan `review`, pero la referencia completa devuelta por `helpReference()` (`cli.go:243-253`) enumera `start`, `show · next`, `clarify`, `approve`, `verify`, `analyze` y `close · cancel · promote`, omitiendo `review`.
- **Evidencia:**
  En `adapters/primary/cli/cli.go:246-250`:
  ```text
    start --kind fix|feature|evolution|technical --lane fast|full --title "…" --why "…" [--intent "…"]
    show · next [unidad]                            Paso y siguiente skill (no escriben)
    clarify --ask "…" · clarify --id Qn --answer "…"
    approve --quote "…"                             Aprueba la definición (solo carril full)
  ```
- **Código corregido (seguro y defensivo):**
  Añadir la línea de `review` entre `clarify` y `approve` en `adapters/primary/cli/cli.go`:
  ```go
      clarify --ask "…" · clarify --id Qn --answer "…"
      review --kind socratic|splash --action <acción> --revision <rev> --operation-id <id> …
      approve --quote "…"                             Aprueba la definición (solo carril full)
  ```
- **Validación del cierre:**
  Ampliar `TestHCD057SocraticCLI` en `adapters/primary/cli/cmd_hcd_socratic_test.go` para comprobar que `helpReference()` contiene `"review --kind socratic|splash"`.

---

## 4. Recomendaciones de resiliencia

1. **Pre-commit / Verificación de formato en tareas HCD:** Incorporar `gofmt -l .` dentro de la comprobación de la tarea final de regresión de cada unidad HCD (o antes de pasar a `StageClose`), evitando que desalineaciones cosméticas rompan el primer paso de `.github/workflows/test.yml`.
2. **Fuente única de derivación del estado de intentos `splash`:** Extraer una función auxiliar privada en `domain` (por ejemplo, `activeSplashWindow(events []HCDDefinitionReviewEvent, rev HCDReviewRevision)`) compartida por `ValidateTransition`, `SplashSatisfied` y `DefinitionReviewSummary`. De este modo, cualquier regla de precedencia entre `accept`, `skip`, `start`, `complete`, `fail`, `unavailable` y `dismiss` se evalúa en un único punto sin riesgo de divergencia entre la escritura y la lectura.
3. **Decodificación JSON acotada en bordes CLI:** Estandarizar en `adapters/primary/cli/clikit` un helper `DecodeStrictJSON(r io.Reader, maxBytes int64, dst any)` que combine `io.LimitReader`, `DisallowUnknownFields()` y la comprobación de `io.EOF` tras el primer objeto para todos los subcomandos que consumen `--input`.
