# Evidencia T119 — escenario 9 del quickstart: los tipos feature, evolution y technical (SC-002)

Fecha: 2026-10-28. Ejecutado con el binario compilado desde la rama por `tests/integration/hcd_kinds_test.go` (`TestHCDKindsEndToEnd`), con `HOME` y proyecto temporales. El tipo `fix` lo cubre `TestHCDBinaryMinimalFlow` (escenario 1).

Cada tipo: `start` → `define` con el campo particular ausente → `validate` debe exigirlo y la unidad sigue en `draft` → `define` completo → `validate` sin pendientes → `approve` → `slice` → `tasks` → `check` → `run` → `report` → `verify` → `close` (`completed`).

Campo particular exigido: feature → `Actor`; evolution → `Compatibilidad`; technical → `Condición de mejora`.

Nota: el quickstart nombra el código `DEF-MISSING`; el motor emite `DEF-PENDING` con el campo en `ref`. La transcripción muestra el comportamiento real.

Límite: es el motor conducido desde la terminal (vía de respaldo), no un agente con `/hcd`.

## Transcripción (`go test ./tests/integration -run TestHCDKindsEndToEnd -v`)

```text
start    → estado=draft rev=1 pendientes=[plantilla sin editar: 9-11 DEF-PENDING]
define   → estado=draft rev=2 pendientes=[DEF-PENDING@Actor DEF-PENDING@Actor]
validate → estado=draft rev=2 pendientes=[DEF-PENDING@Actor DEF-PENDING@Actor]
define   → estado=draft rev=3 pendientes=[]
validate → estado=draft rev=3 pendientes=[]
approve  → estado=ready rev=4 pendientes=[]
slice    → estado=ready rev=5 pendientes=[]
tasks    → estado=ready rev=6 pendientes=[]
check    → estado=ready rev=6 pendientes=[]
run      → estado=in_progress rev=7 pendientes=[]
report   → estado=in_progress rev=8 pendientes=[]
verify   → estado=verification rev=9 pendientes=[]
close    → estado=completed rev=10 pendientes=[]
start    → estado=draft rev=1 pendientes=[plantilla sin editar: 9-11 DEF-PENDING]
define   → estado=draft rev=2 pendientes=[DEF-PENDING@Compatibilidad]
validate → estado=draft rev=2 pendientes=[DEF-PENDING@Compatibilidad]
define   → estado=draft rev=3 pendientes=[]
validate → estado=draft rev=3 pendientes=[]
approve  → estado=ready rev=4 pendientes=[]
slice    → estado=ready rev=5 pendientes=[]
tasks    → estado=ready rev=6 pendientes=[]
check    → estado=ready rev=6 pendientes=[]
run      → estado=in_progress rev=7 pendientes=[]
report   → estado=in_progress rev=8 pendientes=[]
verify   → estado=verification rev=9 pendientes=[]
close    → estado=completed rev=10 pendientes=[]
start    → estado=draft rev=1 pendientes=[plantilla sin editar: 9-11 DEF-PENDING]
define   → estado=draft rev=2 pendientes=[DEF-PENDING@Condición de mejora]
validate → estado=draft rev=2 pendientes=[DEF-PENDING@Condición de mejora]
define   → estado=draft rev=3 pendientes=[]
validate → estado=draft rev=3 pendientes=[]
approve  → estado=ready rev=4 pendientes=[]
slice    → estado=ready rev=5 pendientes=[]
tasks    → estado=ready rev=6 pendientes=[]
check    → estado=ready rev=6 pendientes=[]
run      → estado=in_progress rev=7 pendientes=[]
report   → estado=in_progress rev=8 pendientes=[]
verify   → estado=verification rev=9 pendientes=[]
close    → estado=completed rev=10 pendientes=[]
--- PASS: TestHCDKindsEndToEnd (2.32s)
    --- PASS: TestHCDKindsEndToEnd/feature (0.77s)
    --- PASS: TestHCDKindsEndToEnd/evolution (0.40s)
    --- PASS: TestHCDKindsEndToEnd/technical (0.43s)
PASS
ok  	mem/tests/integration	2.979s
```
