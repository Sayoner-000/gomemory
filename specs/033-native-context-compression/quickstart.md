# Quickstart: validar el motor nativo de compresión

Guía de validación de extremo a extremo contra el **binario real**
(regla de trabajo 2: los tests verdes no bastan). Contratos:
[cli-and-mcp.md](contracts/cli-and-mcp.md) y
[hook-tool-output.md](contracts/hook-tool-output.md). Modelo:
[data-model.md](data-model.md).

## Q0. Preparación aislada

Se aísla el store global con `GOMEMORY_DATA_HOME`. **Nunca** se usa «la base
más reciente» de `~/.local/share/gomemory` (memoria del proyecto:
aislar el store global en pruebas).

```bash
cp ./mem /tmp/mem-2.25.0 && export PATH=/tmp:$PATH   # referencia para Q1, ANTES de compilar
go build -o ./mem . && ./mem --version          # esperado: la versión nueva
export GOMEMORY_DATA_HOME=$(mktemp -d)
P=$(mktemp -d) && cd "$P" && git init -q   # el store global se crea solo al primer uso (mem init ya no hace falta)
cp -r /ruta/al/repo/tests/testdata/compression_corpus ./corpus
```

## Q1. Compatibilidad: `structural` idéntico a la v2.25.0 (SC-008)

```bash
for f in corpus/*; do
  mem-2.25.0 pack compress "$f" > /tmp/old.txt
  ./mem pack compress --level structural "$f" > /tmp/new.txt
  diff <(sed '$d' /tmp/old.txt) <(sed '/^$/,$d' /tmp/new.txt) || echo "DIFIERE: $f"
done
./mem context > /tmp/ctx-new.md   # ajuste ausente → structural
```
**Esperado**: ningún `DIFIERE`. El contenido es igual; solo cambia la línea
de tokens, que ahora va separada. `mem context` sin ajuste es igual al de la
v2.25.0.

## Q2. Ahorro por tipo (SC-001, SC-002)

```bash
./mem pack compress --compare corpus/golist.json
./mem pack compress --compare corpus/gitlog.txt
./mem pack compress --compare corpus/gotest-fail.log
go test ./tests/integration/ -run TestCompressionCorpus -count=1 -v
```
**Esperado**:
- `golist.json`: `máxima` con un ahorro de al menos el 70 % (línea base
  medida: 3,9 % con la estructural).
- Corpus completo: `máxima` al menos un 40 % por debajo de `estructural`
  (lo informa la prueba).

## Q3. Reversibilidad byte a byte (SC-005)

```bash
./mem pack compress --level max --json corpus/golist.json > /tmp/out.json
for r in $(jq -r '.refs[]' /tmp/out.json); do ./mem pack retrieve "$r" > /tmp/orig-$r || echo "FALTA $r"; done
./mem pack retrieve deadbeef0000; echo "exit=$?"
```
**Esperado**: ninguna `FALTA`. Cada original contiene literalmente el
fragmento omitido. La ref inventada termina con `exit=2` y el mensaje del
contrato.

## Q4. Literalidad (SC-006)

```bash
go test ./adapters/secondary/compression/... -run 'TestLiteralGuard|TestProtectedTokens' -count=1 -v
```
**Esperado**: en verde. El conjunto de prueba contiene rutas, URLs, versiones,
mensajes de error e identificadores, y todos aparecen literales en la salida.

## Q5. Deltas de sesión y prefijo estable (SC-003, SC-004)

```bash
./mem settings --compression-level=max   # bandera nueva (contrato cli-and-mcp.md) · o por la TUI: Configuración → Compresión
./mem session start
./mem context > /tmp/t1.md
./mem context > /tmp/t2.md
./mem save -t "nota de prueba" -y learning "delta"
./mem context > /tmp/t3.md
cmp <(sed '/mem:volatile/,$d' /tmp/t1.md) <(sed '/mem:volatile/,$d' /tmp/t3.md) && echo "prefijo estable"
grep -c "ya entregad" /tmp/t2.md
./mem hook post-compact >/dev/null; ./mem context > /tmp/t4.md; grep -c "ya entregad" /tmp/t4.md
go test ./tests/integration/ -run TestSessionDelta20Turns -count=1 -v
```
**Esperado**: `prefijo estable`; `t2` con los bloques marcados como ya
entregados; `t4` = 0, porque la compactación reinicia el registro; la prueba de
20 turnos informa de una reducción del contenido reenviado de al menos el 80 %.
(`mem config` no existe; verificado contra el binario 2.25.0. El nivel se
cambia con `mem settings`, que se amplía en esta feature, o con la TUI.)

## Q6. Privacidad (SC-009)

```bash
printf 'token <private>ghp_FAKE0000</private>\n%.0s' {1..500} > /tmp/priv.txt
./mem pack compress --level max --json /tmp/priv.txt | jq '.refs|length'
DB="$GOMEMORY_DATA_HOME"/projects/*/mem.db
sqlite3 $DB "SELECT COUNT(*) FROM compression_originals WHERE instr(CAST(content_gz AS TEXT),'ghp_')>0;"
```
**Esperado**: `0` refs y `0` filas. El bloque privado solo se comprime sin
pérdida.

## Q7. Salidas de herramientas en runtimes reales (FR-022)

Activar con `./mem settings --tool-output-compression=true` y reinstalar (`./mem install`).

| Runtime | Prueba | Esperado |
|---|---|---|
| Claude Code 2.1.282 | En una sesión: `go list -json ./...` con Bash | El modelo ve la versión comprimida con `⟦mem⟧ … ref=`, y `pack_retrieve` devuelve el original |
| Claude Code | `Read` de un archivo grande y después `Edit` | `Read` sin comprimir; `Edit` funciona |
| Codex 0.157.0 | Una herramienta MCP de otro servidor con salida grande | Comprimida. Un comando de shell sale sin comprimir |
| OpenCode 1.18.32 | `bash` con `go list -json ./...` | Comprimida (verifica que mutar `output.output` surte efecto) |
| OpenCode 2.x | La misma prueba | Si no surte efecto, `mem doctor` → «no soportado» y el plugin no invoca el hook |

Resultado de cada fila → memoria `runtime-tool-output-rewrite` (actualizarla por su `topic_key`).

## Q8. Degradación (FR-006)

El fallo del almacén se inyecta en una prueba con un `OriginalStoreRepository`
falso que devuelve error o supera `StoreTimeout`. Un `chmod` sobre la base no
es fiable, porque SQLite puede seguir escribiendo.

```bash
go test ./adapters/secondary/compression/native/ -run 'TestEngine.*(StoreUnavailable|StoreTimeout|Private)' -count=1 -v
go test ./adapters/primary/cli/ -run TestDoctorCompression -count=1 -v
./mem doctor --json | jq '.compression'
```
**Esperado**: las dos pruebas en verde. Con el almacén caído, el contenido se
entrega con la compresión `structural` y `fallback store_unavailable`, sin
refs; con `--strict` y el almacén no escribible, `doctor` termina con un
código distinto de 0 (lo cubre la prueba). En el binario real,
`.compression` muestra el nivel, el uso de originales y el estado del hook por
runtime.

## Q9. Ajuste adaptativo (SC-010)

```bash
go test ./application/usecases/ -run TestAdaptiveLowersAggressiveness -count=1 -v
./mem pack savings
```
**Esperado**: tras simular más del 20 % de recuperaciones sobre 20 omisiones o
más, `savings` muestra el tipo con agresividad 2 y su motivo.

## Q10. Rendimiento (SC-007)

```bash
GOMEMORY_PERF_ASSERT=1 go test ./adapters/secondary/compression/native/ -run '^TestCompressionCorpusP95$' -count=1 -v
go test ./adapters/secondary/compression/native/ -bench . -benchtime 50x -run '^$'
```
**Esperado**: la primera orden exige un p95 por debajo de 50 ms por cada 10 k
tokens en todas las muestras del corpus; ejecútala en un runner aislado. La
segunda informa `ns/op` y tokens para cada muestra.

## Resultados de validación — 2026-09-25

Validación ejecutada contra el binario compilado desde este árbol, con
`GOMEMORY_DATA_HOME=/tmp/gomemory-q033/data` y un repositorio Git temporal:

| Paso | Resultado real |
|---|---|
| Q0 | Binario compilado desde `./infrastructure`; versión declarada `2.25.0`. Store y proyecto aislados en `/tmp`. |
| Q1 | `TestCompressionStructuralRegression` pasa para todo el corpus, incluida la traza `go-panic.log`. |
| Q2 | `golist.json`: 21 511 → 385 tokens aproximados (98,2 %). Corpus: 167 686 → 45 883 frente a `structural` (72,6 % adicional). |
| Q3 | `pack retrieve` recuperó 86 046 bytes para `dc7f63f2a12d`. La referencia `deadbeef0000` devolvió código 2 y el mensaje contractual. |
| Q4 | `TestLiteralGuard` y `TestProtectedTokens` pasan. |
| Q5 | `TestSessionDelta20Turns`: 163 999 → 16 793 caracteres reenviados en los turnos 2–20 (89,8 % menos). El hook post-compact reinicia el registro. |
| Q6 | `TestCompressionPrivacy_TodasLasVias` pasa; la prueba real produjo 0 referencias para el bloque privado. |
| Q7 | `TestHookToolOutput` pasa con los fixtures reales. Claude Code 2.1.282 ya había sido verificado extremo a extremo; en esta máquina está instalada la 2.1.283. Codex 0.157.0 mantiene soporte parcial para MCP. OpenCode 1.18.32 y 2.x siguen sin una validación interactiva concluyente, por lo que no se declara soporte visible para el modelo. |
| Q8 | `TestEngineStoreUnavailableAndTimeout`, `TestEnginePrivate` y `TestDoctorCompression` pasan. `doctor` informó nivel `max`, almacén escribible, 2 referencias y 51 661 bytes usados. |
| Q9 | `TestAdaptiveLowersAggressiveness` pasa. `pack savings` mostró estadísticas para `json`, `log`, `none` y `structural`. |
| Q10 | `TestCompressionCorpusP95` pasa en las 13 muestras. El peor p95 normalizado observado fue 45,12 ms por 10 k tokens (`gitlog.txt`). El benchmark de 50 iteraciones quedó registrado con `ns/op` y tokens finales por muestra. |

Q7 conserva la limitación documentada en vez de inferir compatibilidad a
partir de una mutación interna del plugin. Validarla en OpenCode requiere una
sesión real cuyo proveedor ejecute una herramienta y permita observar el texto
que recibe el modelo.
