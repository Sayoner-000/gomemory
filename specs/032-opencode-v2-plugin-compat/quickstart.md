# Quickstart: validar el plugin en OpenCode 1.x y 2.x

Guía de validación contra **OpenCode real** (regla de trabajo 2: "verde en
tests" no es "funciona"). Cada escenario usa un `HOME` aislado para no tocar
la configuración de la persona usuaria.

## Preparación

```bash
W=$(mktemp -d)                                  # banco de pruebas desechable
go build -o "$W/mem" ./infrastructure   # binario del árbol actual
export PATH="$W:$PATH"

# OpenCode 2.x (el paquete v2 se llama @opencode/cli, no opencode-ai)
npm i -g --prefix "$W/oc2" @opencode/cli@2.0.16 --allow-scripts=@opencode/cli
# OpenCode 1.x (piso verificado en research R-3)
npm i -g --prefix "$W/oc1" opencode-ai@1.17.0 --allow-scripts=opencode-ai

mkdir -p "$W/h1" "$W/h2" "$W/proj" && (cd "$W/proj" && git init -q)
for H in h1 h2; do HOME="$W/$H" mem setup-mcp --scope global --agents opencode; done
```

## Q0 · Contratos y pruebas unitarias

```bash
go test ./tests/contract/... ./adapters/primary/setup/... ./adapters/primary/cli/... -run 'OpenCode|Opencode|Doctor' -count=1
node --test infrastructure/plugin/opencode/gomemory.test.mjs
```

**Esperado**: todo en verde, incluidas las pruebas nuevas de la rama v2 (ctx falso).

## Q1 · Carga sin error en 2.x (US1, SC-001)

```bash
cd "$W/proj"; export HOME="$W/h2"
"$W/oc2/bin/opencode" reload && "$W/oc2/bin/opencode" plugin list
grep -h "failed to load plugin" "$HOME/.local/share/opencode/log/"*.log | grep gomemory || echo "sin errores de gomemory"
"$W/oc2/bin/opencode" service stop
```

**Esperado**: `gomemory  local  …/plugins/gomemory.ts` en la lista y ninguna
línea `failed to load plugin` que mencione gomemory.

## Q2 · Sin regresión en 1.x (US2, SC-002)

```bash
cd "$W/proj"; export HOME="$W/h1"
"$W/oc1/bin/opencode" debug config >/dev/null
mem session list | head -3
```

Repetir con la instalación local 1.18.32 (`opencode`) usando el mismo `HOME` aislado.

**Esperado**: el plugin se carga por `server()` sin errores en los logs de
OpenCode 1.x. Una sesión real (Q3) produce los mismos rastros que antes del cambio.

## Q3 · Paridad de capacidades con sesión real (US3, SC-003)

Requiere credenciales de un modelo en el `HOME` aislado (`opencode auth login`).
Para cada versión (`oc1` y `oc2`):

1. `opencode run "crea hola.txt con el texto hola y ejecuta ls"`
2. `mem search "Checkpoint automático"` → aparece `hola.txt` y el comando `ls` (**turn-end**).
3. `mem doctor` → `plan_entry` y `turn_reminder` de opencode muestran uso reciente (**inyección** y **rastros**).
4. Pedir una tarea con subagente (`task`) → `mem search` encuentra su salida (**subagent-stop**).
5. Forzar una compactación (`/compact` en la TUI) → el siguiente turno recibe la recuperación una sola vez (**C1/C2**). Si C6 no se pudo leer, `mem doctor` muestra el `channel-error` correspondiente en lugar de un canal sano.

**Esperado**: 5/5 capacidades en 1.x y al menos 4/5 en 2.x. La que falte aparece declarada.

## Q4 · Permisos MCP en 2.x (R-8)

En la sesión de Q3 con `oc2`, el agente llama a `gomemory_search_memories`
**sin** pedir aprobación, y `gomemory_forget_memory` **sí** la pide.

## Q5 · Diagnóstico (US4)

```bash
export HOME="$W/h2"
printf 'export const X = async () => ({})\n' > "$HOME/.config/opencode/plugins/cbm-augment.ts"
touch "$HOME/.config/opencode/plugins/gomemory.test.mjs"
mem doctor | grep -A6 -i opencode
```

**Esperado**: la versión v2 detectada, `plugin gomemory: dual (v1+v2) ✅`, el
aviso sobre `cbm-augment.ts` como plugin ajeno y el aviso del artefacto de
pruebas. Tras reinstalar (`mem setup-mcp --scope global --agents opencode`),
`gomemory.test.mjs` desaparece y `cbm-augment.ts` sigue intacto.

## Q6 · Idempotencia (SC-005)

```bash
sha1sum "$HOME/.config/opencode/opencode.json" "$HOME/.config/opencode/plugins/gomemory.ts"
mem setup-mcp --scope global --agents opencode
sha1sum "$HOME/.config/opencode/opencode.json" "$HOME/.config/opencode/plugins/gomemory.ts"
```

**Esperado**: los mismos hashes.

## Limpieza

```bash
HOME="$W/h2" "$W/oc2/bin/opencode" service stop; rm -rf "$W"
```
