# Guía de validación: ciclo de vida de gomemory en consola

Escenarios que se pueden ejecutar para demostrar la feature de extremo a extremo. **Siempre en un entorno aislado**: nunca contra el `$HOME` ni el almacén reales, salvo en el escenario 8, que está marcado.

## Preparación común

```bash
export SANDBOX=$(mktemp -d)
mkdir -p $SANDBOX/home/.local/bin $SANDBOX/p1/.git $SANDBOX/p2/.git
go build -o $SANDBOX/home/.local/bin/mem ./infrastructure
cp $SANDBOX/home/.local/bin/mem $SANDBOX/reimport-mem
export HOME=$SANDBOX/home GOMEMORY_DATA_HOME=$SANDBOX/data
export PATH=$HOME/.local/bin:/usr/bin:/bin
# Archivos compartidos con una entrada ajena, para comprobar FR-012:
printf '{"mcpServers":{"otro":{"command":"otro"}}}' > $HOME/.claude.json
mkdir -p $HOME/.codex && printf '[mcp_servers.otro]\ncommand = "otro"\n' > $HOME/.codex/config.toml
```

## 1. Binario único (US1, SC-001)

```bash
cd $SANDBOX/p1 && mem install --yes . && test ! -e mem && echo "OK sin copia"
cp $HOME/.local/bin/mem $SANDBOX/p2/mem    # copia real del binario; una versión antigua también sirve
cd $SANDBOX/p2 && echo '{}' | mem hook session-start --emit=claude | jq -r .systemMessage
```
**Esperado**: `p1/mem` no existe; en `p2`, `systemMessage` indica la ruta y versión de la copia retirada y la versión global que la sustituye, y `p2/mem` ya no existe. Un `p2/mem` que sea un script propio (`printf '#!/bin/sh\necho hola' > mem`) **no** se toca.

## 2. `update` desde una copia local (FR-006)

Con un servidor de releases local (`GOMEMORY_RELEASE_DOWNLOAD_BASE` y la base de la API apuntando a un `httptest` o a un directorio servido): `./mem update --yes` → el global queda en la versión servida y `./mem` desaparece. El caso reproducible con servidor `httptest` está en `TestUpdate_DesdeCopiaLocalActualizaElGlobal` de `tests/contract/lifecycle_update_test.go`.

## 3. Checksum alterado (FR-032, SC-009)

Se sirve un `checksums.txt` con un hash cambiado → `mem update --yes` termina con código 1, con `✗ checksum`, y `mem version` sigue devolviendo la versión anterior. El caso reproducible está en `TestUpdate_ChecksumInvalidoAborta` del mismo archivo.

## 4. Aviso de versión (US4, SC-007, SC-008)

```bash
printf '{"latest":"v99.0.0","checked_at":"%s","etag":""}' "$(date -u +%FT%TZ)" > $GOMEMORY_DATA_HOME/update-check.json
cd $SANDBOX/p1 && echo '{}' | mem hook session-start --emit=claude | jq -r .systemMessage   # aviso
echo '{}' | mem hook session-start --emit=claude | grep -q '"systemMessage"' && echo "con aviso" || echo "sin aviso"   # misma sesión: sin aviso (sin avisos la salida es texto plano, no JSON)
GOMEMORY_NO_UPDATE_CHECK=1 mem hook session-start </dev/null   # sin aviso y sin red
```
**Sin red (SC-008)**: ejecuta el último comando con la red bloqueada y con una caché vencida. No aparece ningún proceso `update-check` (`pgrep -f update-check` vacío) y `update-check.json` no cambia.

## 5. Instalación guiada (US3)

En una terminal real: `cd $SANDBOX/p1 && mem install .` → selección múltiple de agentes con los detectados ya marcados, alcance, resumen y confirmación. Después:
```bash
mem install . </dev/null ; echo "exit=$?"          # no espera entrada
jq '.agents, .agent_scope' .memory/settings.json   # selección guardada
NO_COLOR=1 mem install --yes . | cat               # texto plano, misma información
```

## 6. Desinstalación de sistema sin rastros (US2, SC-003, SC-004, SC-011)

```bash
cd $SANDBOX/p2 && mem install --yes .
mem uninstall --all --dry-run > $SANDBOX/plan-antes.txt
mem uninstall --all --dry-run > $SANDBOX/plan-despues.txt
cmp $SANDBOX/plan-antes.txt $SANDBOX/plan-despues.txt && echo "OK plan estable"
mem uninstall --all </dev/null ; echo "exit=$?"    # sin --yes: 1 y nada borrado
mem uninstall --all --yes --export $SANDBOX/export
grep -rl gomemory $HOME ; ls $GOMEMORY_DATA_HOME 2>&1 ; ls $SANDBOX/p*/{mem,.memory} 2>&1
jq . $HOME/.claude.json ; cat $HOME/.codex/config.toml   # solo queda "otro"
stat -f '%Lp' $SANDBOX/export $SANDBOX/export/*.json      # 700 / 600
```
**Esperado**: el `grep` no encuentra nada, el almacén y las integraciones de `p1`/`p2` ya no existen, solo quedan las entradas `otro` (más la bandera compartida `[features] hooks = true` de Codex, que se conserva) y la exportación tiene permisos 0700/0600. La salida no anuncia que `~/.codex/config.toml` "conserva" la entrada de gomemory: esa nota solo aparece en el alcance de proyecto. El `cmp` comprueba que el plan es estable; la ausencia de escrituras se valida con el inventario de archivos y sus hashes antes y después, como en `TestUninstall_DryRunNoModifica` de `tests/contract/lifecycle_uninstall_test.go`. **Reimportación (SC-011)**: usa `$SANDBOX/reimport-mem import $SANDBOX/export/<key>.json` desde cada proyecto; debe restaurar el número de memorias de `index.json`.

## 7. Escaneo (aclaración 2026-09-26)

Se crea un proyecto sin registrar a 3 niveles y otro a 8 niveles de `$HOME`: `mem uninstall --all --dry-run` incluye el primero y el resumen advierte con `--scan` sobre la profundidad. Con `--no-scan` solo aparecen los registrados.

## 8. Validación en la máquina real (solo lectura)

```bash
mem version
for d in ~/home/rcw/*/; do [ -x "$d/mem" ] && echo "$d $("$d/mem" version)"; done
```
No ejecutar `mem doctor` en esta comprobación: puede intentar ajustar permisos del almacén. Tras actualizar y abrir cada proyecto una vez, el bucle no imprime ninguna copia de gomemory (SC-001). Si alguna sigue presente, registrar ruta y versión sin modificarla. Un `exec format error` indica un binario de otra plataforma (por ejemplo, Linux para un contenedor). Se conserva a propósito (ver Edge Cases de la spec).

## Pruebas automatizadas

`go test ./...` más las pruebas de contrato nuevas en `tests/contract/` (lifecycle de install/uninstall en `HOME` y `GOMEMORY_DATA_HOME` temporales, y los invariantes de [hook-session-start.md](contracts/hook-session-start.md)).
