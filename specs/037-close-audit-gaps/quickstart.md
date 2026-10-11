# Guía de validación — feature 037

Esta guía se ejecuta después de implementar la feature. Los comandos existentes de pruebas y build ya están disponibles; `mem sanitize` y los tres runners `audit_037_*` son interfaces previstas que deberán crearse en implementación. Su ausencia es un fallo de preparación, nunca una validación satisfactoria. No se ejecutó saneamiento real durante `$speckit-plan`.

## Prerrequisitos

- Checkout de la revisión a validar, Go >=1.27.0 con toolchain compatible, Node >=22.20.0 y Python >=3.12.
- Windows real para PowerShell y un host OpenCode v2 independiente de la instalación v1 local. Registrar versión concreta; no usar un alias que resuelva v1.
- Dependencias del repositorio disponibles. El instalador usa su package-lock existente; `npm ci` no agrega otro lockfile.
- Para host/modelo, usar credenciales de prueba configuradas por el operador en un entorno aislado; no copiarlas automáticamente del perfil habitual. Si no hay host/modelo requerido, registrar pendiente y salir no cero.

## Preparar un banco aislado

Desde la raíz del checkout:

```bash
AUDIT037_REPO="$PWD"
AUDIT037_WORK="$(mktemp -d)"
mkdir -p "$AUDIT037_WORK/bin" "$AUDIT037_WORK/home" "$AUDIT037_WORK/data" "$AUDIT037_WORK/project" "$AUDIT037_WORK/evidence"
go build -o "$AUDIT037_WORK/bin/mem" ./infrastructure
git -C "$AUDIT037_WORK/project" init -q
go version
node --version
python3 --version
git rev-parse HEAD
```

No redefinir HOME en el shell del operador. Los runners lanzan procesos hijos con HOME, XDG_CONFIG_HOME, XDG_DATA_HOME, XDG_STATE_HOME, XDG_CACHE_HOME y GOMEMORY_DATA_HOME temporales. Ninguna prueba debe escribir el almacén o configuración habitual.

## Q1 — Pruebas dirigidas y globales

```bash
go test ./domain ./application/usecases ./adapters/secondary/persistence ./adapters/primary/cli -count=1
go test -race ./application/usecases ./adapters/secondary/persistence ./adapters/primary/cli -count=1
go test ./tests/contract ./tests/integration -count=1
node --test infrastructure/plugin/opencode/gomemory.test.mjs
npm --prefix installer ci
npm --prefix installer run typecheck
npm --prefix installer test
go test ./... -count=1
go vet ./...
GOMEMORY_COVERAGE_OUTPUT="$AUDIT037_WORK/evidence/coverage" bash scripts/coverage.sh
git diff --check
```

Esperado: todas las regresiones previstas ejecutadas; ninguna selección vacía aceptada; sin carreras; suites plugin e instalador completas; cobertura combinada >=80 % y sin bajar la línea base comparable. Registrar resultados actuales, no reutilizar las cifras del informe del 2026-10-11 como prueba posterior. Si falla cobertura, no marcar cerrada la entrega.

## Q2 — Privacidad desde interfaces reales

```bash
python3 scripts/audit_037_smoke.py \
  --mem-bin "$AUDIT037_WORK/bin/mem" \
  --work-dir "$AUDIT037_WORK/project" \
  --data-home "$AUDIT037_WORK/data" \
  --out-dir "$AUDIT037_WORK/evidence/privacy" --case privacy
```

Runner previsto: usa solo marcadores sintéticos y prueba memorias, sesiones CLI/MCP, compact-summary, channel-error, creación/edición/importación de relaciones, exportación y contexto. Incluye anidamiento, apertura incompleta, mayúsculas, Unicode, texto público y segunda aplicación. Inspecciona SQLite real y capturas stdout/stderr; cero contenido protegido en nuevas filas o respuestas. Recuperación de original privado debe fallar sin revelar contenido; original público debe recuperarse exacto.

Detalles y resultados: [privacy-maintenance](contracts/privacy-maintenance.md), US1 y SC-001.

## Q3 — Saneamiento histórico y recuperación

```bash
python3 scripts/audit_037_smoke.py \
  --mem-bin "$AUDIT037_WORK/bin/mem" \
  --work-dir "$AUDIT037_WORK/project" \
  --data-home "$AUDIT037_WORK/data" \
  --out-dir "$AUDIT037_WORK/evidence/history" --case history
```

El runner prepara registros históricos sintéticos mediante fixture de DB, copias JSON y originales; no intenta insertar datos históricos por una entrada nueva que ya los redacta. Preview debe dejar filas iguales y no revelar texto. Apply debe preservar ids/fechas/público y limpiar los artefactos administrados. Segundo apply: cero cambios lógicos.

Casos adicionales: interrupción antes y después del commit, escritor simultáneo, lector que bloquea checkpoint, copia inaccesible, falta de espacio simulada y cancelación. Comprobar pending_cleanup y salida 1 hasta reintento satisfactorio. Verificar que ninguna prueba borra WAL/SHM manualmente ni genera backup crudo nuevo. En el banco sin clientes abiertos, buscar marcadores en DB, WAL, copias y derivados para comprobar ausencia de residuos controlados.

Para inspección manual únicamente dentro del banco ya inicializado:

```bash
(cd "$AUDIT037_WORK/project" && GOMEMORY_DATA_HOME="$AUDIT037_WORK/data" "$AUDIT037_WORK/bin/mem" sanitize --json)
(cd "$AUDIT037_WORK/project" && GOMEMORY_DATA_HOME="$AUDIT037_WORK/data" "$AUDIT037_WORK/bin/mem" sanitize --apply --json)
```

No trasladar estos comandos a un almacén real sin revisar alcance y detener/reiniciar clientes antiguos. La autorización de saneamiento proviene de --apply cuando el operador lo ejecuta; este plan no lo ejecuta.

## Q4 — Contexto y wrap

```bash
python3 scripts/audit_037_smoke.py \
  --mem-bin "$AUDIT037_WORK/bin/mem" \
  --work-dir "$AUDIT037_WORK/project" \
  --data-home "$AUDIT037_WORK/data" \
  --out-dir "$AUDIT037_WORK/evidence/context-wrap" --case context-wrap
```

Tres construcciones secuenciales y ocho solicitudes MCP en la misma conexión/instancia deben tener métricas de emisión equivalentes, fijando datos/reloj. Comparar emisiones individuales, no el total de usage agregado. Las regresiones Go con race comprueban la variante concurrente interna.

Wrap: stdin cerrado, comando inexistente, archivo sin permisos ejecutables donde aplique, hijo con código 0 y hijo con código 7. Arranque fallido -> 1 y sin diálogo; hijo iniciado -> salida y código preservados. Ver [context-wrap](contracts/context-wrap.md).

## Q5 — Instalación y reemplazo Unix

```bash
python3 scripts/audit_037_smoke.py \
  --mem-bin "$AUDIT037_WORK/bin/mem" \
  --work-dir "$AUDIT037_WORK/project" \
  --data-home "$AUDIT037_WORK/data" \
  --out-dir "$AUDIT037_WORK/evidence/install-unix" --case installation
```

Runner: fixtures de release/tag, asset y comprobante; ninguna red pública. Ejercita versión explícita/latest y toda la matriz de [installation](contracts/installation.md). Conserva bytes del ejecutable previo ante fallos. Update Unix usa barreras de preparación e invocaciones del comando para comprobar anterior/nuevo completos sin ausencia; incluye fuente ausente, permisos, cancelación y fallo de publicación.

## Q6 — Windows

En checkout Windows, con directorio temporal propio:

```powershell
$Audit037Work = Join-Path ([System.IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $Audit037Work | Out-Null
go build -o (Join-Path $Audit037Work 'mem.exe') ./infrastructure
powershell -NoProfile -File scripts/audit_037_windows_smoke.ps1 -MemBin (Join-Path $Audit037Work 'mem.exe') -WorkDir $Audit037Work -Case all
```

Runner usa LOCALAPPDATA temporal e interceptores de red de prueba sin saltarse hash. Validar correcto/incorrecto/ausente/duplicado, red, permisos, archivo inválido y ejecutable previo. Salida cero exige todos los casos. Guardar evidencia de versión PowerShell, Windows y revisión. No interpretar compilación cruzada ni inspección estática como Q6 aprobado.

## Q7 — Consumidor y lifecycle OpenCode v2 real

Seleccionar un binario v2 ya preparado en banco separado; no actualizar el binario global. Si debe aprovisionarse, fijar una versión compatible y verificar su contrato antes de ejecutar. El historial validó 2.0.16; no usar latest como evidencia reproducible.

```bash
OPENCODE_V2_BIN="/ruta/al/banco/opencode-v2"
"$OPENCODE_V2_BIN" --version
node scripts/audit_037_opencode_smoke.mjs \
  --mem-bin "$AUDIT037_WORK/bin/mem" \
  --opencode-bin "$OPENCODE_V2_BIN" \
  --out-dir "$AUDIT037_WORK/evidence/opencode-v2"
```

La ruta anterior es un parámetro que el operador reemplaza por su host v2. Runner aborta con 2/pending_verification si falta host/modelo necesario. Usa servidor privado/standalone y todas las rutas aisladas; no detiene el servicio habitual. Observar resultado recibido después del hook, no solo el objeto temporal modificado.

Esperado: resultado reducido en tres formas soportadas; partes mixtas preservadas; originales públicos recuperables; casos no-op intactos; sesiones ajenas ignoradas. Fallar cada registro de setup y verificar cero recursos propios. Durante evento con IO, solicitar cierre y comprobar <=5 s, sin hijos ni operaciones pendientes, sin publicación tardía. Repetir cleanup y fallos de dispose/suscripción. Ver [opencode-lifecycle](contracts/opencode-lifecycle.md).

## Q8 — Registro y cierre

Guardar por H-01–H-09: revisión, requisitos y escenarios, entorno, comandos, resultado esperado/observado y rutas de evidencia depurada. Estado closed exige validación correspondiente; host/Windows ausentes -> corrected_pending_verification. El runner debe devolver no cero para casos faltantes y no convertirlos en skips silenciosos.

Comparar impacto y llamadores del diff de código con codebase-memory-mcp antes de declarar implementación terminada. Confirmar contratos estructurados y rama v1 sin regresión. El resultado de esta guía es un registro verificable, no una nueva garantía de ausencia universal de defectos.

## Contrato de runners previstos

- `audit_037_smoke.py`: casos privacy, history, context-wrap, installation o all; stdlib y binario real; salida 0 solo con todos los casos seleccionados ejecutados y aprobados, 1 por defecto reproducido/fallo y 2 por preparación inválida.
- `audit_037_windows_smoke.ps1`: parámetros MemBin, WorkDir y Case; crea destinos aislados y usa misma política de códigos y conteo.
- `audit_037_opencode_smoke.mjs`: parámetros del contrato OpenCode; espera y cierra servidor propio, sin asumir que promesa rechazada significa hijo cerrado.
- Todos crean evidencia sin payloads reales, registran número de casos y fallan si es cero. No se importan como plugins de producción ni exponen bypasses de seguridad.
