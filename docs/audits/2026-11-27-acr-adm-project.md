# Informe ACR + ADM — superficies de producción

Fecha: 2026-11-27. Revisión auditada: `c8295be10cb23f9e9dcba4cc9d4ba78fa7957ac5` (árbol limpio, tree `d44bcaa2ddd6748fac31c6480f1556056ece316f`).
ACR: `acr_93a32f92-751b-40bc-b106-69db03cd8d2e`, revisión de seguimiento de `acr_6d40dfd8-adce-48d9-bff2-9c6073de210f` (delta de 125 archivos, 3 hallazgos heredados).
Solo informe: no se modificó código de producción, no se corrigieron defectos, no se autorizó commit ni publicación.

## 1. Resumen de salud del código

- **Tecnologías:** Go 1.27.4 en producción y pruebas; SQLite (modernc); TypeScript/Node en el plugin de OpenCode y el instalador; shell en `scripts/`.
- **Nivel de riesgo general: MEDIO.** No hay defectos críticos ni altos. La base es sólida: suite amplia en verde, redacción de privacidad en múltiples fronteras, escritura atómica con rename y cierres de auditorías anteriores vigentes. Quedan dos defectos de alcance acotado: una revalidación HCD que puede informar éxito parcial y un contrato de proveedor desactualizado frente a la implementación.
- **Resultado:** 2 hallazgos (0 críticos, 0 altos, 1 medio, 1 bajo). ACR: **APPROVED** con 0 CONFIRMED, 2 SUSPECT y 0 contradicciones; el veredicto del ledger no implica ausencia de defectos ni autorización de entrega.

### Cobertura y validación

| Comprobación | Resultado |
|---|---|
| `git rev-parse HEAD` / tree / estado | `c8295be` / `d44bcaa` / limpio, congelado antes de la revisión |
| Dos revisores ACR sobre el mismo target | A: `success`, 0 hallazgos; B: `success`, 2 hallazgos |
| `go vet ./...` | Pasa |
| `go test ./...` | Pasa (CLI 17.8 s, contratos 78.9 s, integración 40.8 s) |
| `go test -race ./domain ./application/usecases ./adapters/secondary/persistence ./adapters/secondary/hcdfiles` | Pasa |
| `go mod verify` | Pasa: todos los módulos verificados |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | Pasa 68/68 |
| `npm --prefix installer run typecheck` y `npm --prefix installer test` | Pasa: typecheck limpio, 24/24 |
| `bash -n scripts/install.sh scripts/test-install-uninstall.sh` | Pasa (solo sintaxis) |
| `git diff --check` | Pasa (árbol limpio) |
| `govulncheck`, `gitleaks`, `staticcheck`, `semgrep`, `shellcheck` | No disponibles en PATH; no instalados |
| Windows real, escaneo exhaustivo de secretos, auditoría línea por línea | No ejecutados |

### Límites

El ACR se ejecutó como revisión de seguimiento sobre el delta desde `7cd5e35`; los tres hallazgos heredados se dieron por asentados y no se re-adjudicaron. Independencia **`degraded`**: dos contextos aislados pero el mismo modelo (`opencode/hy4-flash-free`). La inspección es dirigida por riesgo, no exhaustiva: no cubre todo el árbol ni todas las plataformas. Las suites verdes no demuestran ausencia de los defectos descritos.

## 2. Hallazgos y fugas detectadas

| ID | Severidad | Estado ADM / ACR | Problema |
|---|---|---|---|
| H-01 | Media | CONFIRMADO / SUSPECT (C-001) | `verify --stale` revalida solo el subgrupo con comando y deja las tareas manuales caducadas sin aviso |
| H-02 | Baja | CONFIRMADO / SUSPECT (C-002) | Contrato de `CodeGraphProvider` describe un sondeo corto; la implementación inicializa e indexa con plazo de 10 min |

La diferencia de estado es deliberada: en ADM el hallazgo está demostrado sin ambigüedad en el código; en ACR sigue siendo SUSPECT porque lo reportó un solo revisor y el consenso exige dos fuentes independientes.

### H-01 — Revalidación parcial silenciosa en `verify --stale`

- **Categoría:** error lógico / éxito falso.
- **Severidad:** Media. **Estado:** CONFIRMADO (ADM), SUSPECT (ACR C-001, fuente B-001).
- **Ubicación:** `application/usecases/hcd.go:384-422` (`verifyStaleEach`); entrada en `:320-333` (`Verify`).
- **Ruta de alcance:** `mem hcd verify --stale` (`adapters/primary/cli/hcd/execute.go`) → `HCD.Verify` → `verifyStaleEach`.
- **Análisis:** `domain.HCDState.Stale()` (`domain/hcd_stage.go:258-310`) incluye tanto tareas cuya última evidencia verde fue un comando como tareas cuya última evidencia verde fue manual. `verifyStaleEach` agrupa por comando: las que no tienen `EvidenceCommand` van a `manual` y se omiten (`:399-404`). La única rama de error es `len(order) == 0` (`:410-412`); si existe al menos un comando, se reejecuta ese grupo, se devuelve la última evidencia y la función termina con éxito. Las tareas manuales permanecen caducadas y ningún campo `Warnings` lo refleja: la persona recibe "revalidación correcta" aunque solo se observó una parte.
- **Evidencia:** `verifyStaleEach` en `application/usecases/hcd.go:399-421`; `Stale()` en `domain/hcd_stage.go:299-310`; `Evidence.Passed()` en `domain/hcd.go:132-134` (la evidencia manual cuenta como observación que pasó). Revisor A no encontró el defecto; revisor B lo describió con el mismo mecanismo y el orquestador lo verificó en fuente.
- **Corrección mínima propuesta (no aplicada):** fallar cerrado antes de ejecutar cualquier comando, para no dejar efectos parciales:

  ```go
  if len(manual) > 0 {
      return view, domain.Evidence{}, fmt.Errorf("%w: las tareas caducadas %s solo pasaron con evidencia manual: vuelve a observarlas con --task N --manual",
          domain.ErrValidation, strings.Join(manual, ", "))
  }
  ```

  Colocar este bloque justo después de construir `manual`/`order` y antes del bucle de reejecución. Alternativa si se desea revalidación parcial: devolver la lista de omitidas en `view.Warnings` y hacer que la CLI la imprima; en ese caso el éxito debe declarar explícitamente el subconjunto observado.
- **Validación del cierre:** prueba con una tarea caducada manual y otra con comando: debe devolver `domain.ErrValidation` y no escribir evidencia nueva; las pruebas existentes de `--stale` con comandos siguen pasando. Con la alternativa de avisos, comprobar que la salida nombra las tareas omitidas.

### H-02 — Contrato de `CodeGraphProvider` desactualizado

- **Categoría:** contrato / documentación de arquitectura.
- **Severidad:** Baja. **Estado:** CONFIRMADO (ADM), SUSPECT (ACR C-002, fuente B-002).
- **Ubicación:** `application/ports/code_graph_provider.go:12-20`; `adapters/secondary/codegraph/codegraphcli/provider.go:28,158-184`.
- **Ruta de alcance:** `get_context` / hook de fin de turno → `MaybeRefresh()` → proceso detached `mem code-refresh` → `Provider.Refresh` → `codegraph init --yes` o `codegraph index --quiet`.
- **Análisis:** el contrato del puerto afirma que el refresco desacoplado "sondea al proveedor con timeout corto" y que gomemory nunca dispara indexación del proveedor. La implementación, en cambio, inicializa un proyecto sin `.codegraph/` y reindexa cuando `reindexRecommended`, con `indexTimeout = 10 * time.Minute`, y escribe el snapshot en `.memory`. El hot path no se bloquea (el hijo está desacoplado), por lo que no hay fallo funcional demostrado; hay una divergencia entre contrato y comportamiento que invalidra las garantías que los consumidores creen tener y deja sin documentar un efecto de escritura y un plazo largo. Existe el interruptor `GOMEMORY_NO_CODE_REFRESH`.
- **Evidencia:** comentario del puerto en `application/ports/code_graph_provider.go:17-20`; `indexTimeout` en `provider.go:28`; `init --yes` en `:158-166`; `index --quiet` en `:174-180`; `MaybeRefresh` en `:81-103`. Verificado por el orquestador en código.
- **Corrección mínima propuesta (no aplicada):** si el comportamiento actual es el deseado, actualizar el contrato del puerto para describir el refresco detached con `init`/`index`/`sync`, el plazo de 10 minutos y el opt-out por entorno. Si la política es no indexar automáticamente, mover `init`/`index` a `mem index` o `doctor --fix` y dejar `Refresh` como sondeo corto.
- **Validación del cierre:** `TestSnapshot_EsIndependienteYNoBloquea` y `TestRefresh_InicializaAutonomamenteProyectoNuevo` siguen verdes; añadir una comprobación de contrato que alinee la documentación del puerto con la ruta ejecutada y verificar que `MaybeRefresh` retorna sin esperar al hijo.

### Cierres históricos verificados

- 2026-11-06: H-01 (recorte antes de redactar), H-02 (plazo HCD con descendientes) y H-03 (filepath/topic_key sin proteger) cerrados en HCD-043/044/045; el código actual conserva `ProtectText` antes del recorte, grupo de procesos con `WaitDelay` y `ValidateMemoryMetadata`.
- 2026-11-21: C-001 a C-004 cerrados en v2.55.1 (validación de privacidad en descubrimiento y persistencia, esquema 14 con reapunte pendiente, `SaveMetadata` sin campos de ciclo de vida, `MoveDoc` con renombre atómico no-replace). El revisor A confirmó que no reaparecen en el delta; los tres inherited siguen asentados y no se re-adjudicaron.

## 3. Recomendaciones de resiliencia

1. Aplicar fail-closed a cualquier revalidación que no pueda observar todas sus tareas: un éxito parcial debe decir qué se observó o no declararse éxito.
2. Alinear el contrato de `CodeGraphProvider` con la implementación y hacer explícito el efecto de escritura del refresco detached; conservar y documentar `GOMEMORY_NO_CODE_REFRESH`.
3. Incorporar `govulncheck` y un escáner de secretos al CI de release: no estaban disponibles localmente y las suites verdes no sustituyen ese análisis.
4. Para la próxima ronda ACR, usar modelos distintos o proveedores distintos para elevar la independencia de `degraded` a `partial`/`full`.

## ACR — resultado del ledger

```
Target:      repository @ c8295be10cb23f9e9dcba4cc9d4ba78fa7957ac5
Digest:      d44bcaa2ddd6748fac31c6480f1556056ece316f
Review:      acr_93a32f92-751b-40bc-b106-69db03cd8d2e
Since:       acr_6d40dfd8-adce-48d9-bff2-9c6073de210f
Independence: degraded (mismo modelo, contextos aislados)
Rounds:      initial 1 · fix 0 · rejudgment 0
Confirmed:   0    Suspect: 2 (C-001 MEDIUM, C-002 LOW)
Contradictions: 0    Info: 0
Corrections: ninguna (solo lectura)    Re-judgment: n/a
VERDICT: APPROVED
```

`APPROVED` significa únicamente que el target satisfizo el protocolo de revisión con dos revisores y consenso; no autoriza commit, push, merge, despliegue ni publicación.
