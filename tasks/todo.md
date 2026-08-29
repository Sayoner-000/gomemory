# Paridad de canales entre agentes (Codex · OpenCode · Claude Code)

**Fecha**: 2026-08-29 · **Estado**: [1], [2] y [3] completos

## Objetivo

Cerrar las brechas de canal por las que Codex y OpenCode perdían el método de
descomposición y la constitución, y dejar que la matriz de canales refleje
capacidades reales en vez de afirmaciones caducadas.

## Resultado

### [2] Habilidades a los tres agentes — COMPLETO

- [x] **[2.1]** Generalizado el instalador a `InstallAgentSkill(name, body, agents...)`
      en `adapters/primary/setup/agent_skills_setup.go` (renombrado desde
      `adversarial_review_setup.go`, que ya no describía su contenido).
- [x] **[2.2]** `atomic-decomposition` publicada en Codex y OpenCode.
- [x] **[2.3]** `constitution` publicada en los tres (incluido Claude: su
      instalador previo es de ámbito de PROYECTO, así que la ruta global de
      Claude no tenía dueño).
- [x] **[2.4]** Matriz actualizada: `.codex/skills` y `.opencode/skills`
      declaradas como `KindNativeWrapper` gestionado.

### [3] Motivos declarados en la matriz — COMPLETO

- [x] Reemplazado el motivo impreciso de `KindPlanGuard` («el ciclo del agente no
      ofrece un punto de decisión») por el verificado: **ninguno de los dos expone
      una llamada a herramienta que marque «estoy presentando el plan»**, que es
      exactamente lo que hace interceptable a Claude Code vía `ExitPlanMode`.

### [1] Inyección por turno en Codex — COMPLETO

- [x] **[1.1]** Verificado en sesión interactiva real (Codex 0.151.0):
      `UserPromptSubmit` dispara **y su stdout llega al modelo como contexto**
      (el agente respondió con el marcador secreto de la sonda). `codex exec` no
      ejecuta hooks —ninguno—, así que la comprobación exigió sesión real.
- [x] **[1.2]** `renderPromptContext` en `hook_dialect.go`: `user-prompt-submit`
      era el único hook que no pasaba por el traductor de dialectos. Emitía JSON
      de Claude siempre, y `--emit` se ignoraba.
- [x] **[1.3]** Matriz: `codex · KindPlanEntry · usuario` pasa de «no aplicable»
      a `.codex/config.toml` gestionado.
- [x] **[1.4]** `{Event: "UserPromptSubmit", Sub: "user-prompt-submit", Emit: "text"}`
      en `codexGomemoryHooks`, con campo `Emit` nuevo en `CodexHook`.
- [x] **[1.5]** Verificado con binario real sobre un `config.toml` con hooks
      ajenos: escribe `mem hook user-prompt-submit --emit=text`, preserva lo
      ajeno, idempotente, TOML válido.

## Verificación (no solo tests)

- Suite completa `go test ./...` en verde.
- Binario compilado contra HOME aislado: **9 habilidades = 3 agentes × 3
  habilidades**, contenido byte a byte idéntico a la fuente embebida.
- Idempotencia: segunda ejecución reporta **0** escrituras nuevas.
- No ensucia: con solo `~/.claude` presente, **no** se crean `~/.codex` ni
  `~/.opencode`.
- Rutas verificadas contra los binarios reales, no contra documentación: el de
  Codex contiene `.codex/skills` y `SKILL.md`; el de OpenCode contiene
  literalmente `.opencode/skills/my-skill/SKILL.md`.

## Lecciones

1. **La matriz de canales caducó en silencio, dos veces.** Declaraba que Codex no
   tenía formato de habilidades (falso: 96 ocurrencias de `SKILL.md` en su
   binario) y daba un motivo impreciso para el guardián de plan. Una celda
   marcada «no aplicable» no la revisa nadie: es donde una afirmación caduca
   sobrevive sin que ningún test la contradiga.
2. **El control negativo evitó una conclusión falsa.** Al no disparar el hook en
   `codex exec`, la lectura fácil era «Codex no soporta UserPromptSubmit». El
   control con `SessionStart` —que sí funciona en interactivo— demostró que el
   experimento era inválido, no la hipótesis.
3. **Un test existente atrapó la regresión que introduje.** `TestC7_Destinos
   GlobalesConcuerdanConLaMatriz` exigió declarar por qué Codex no lleva
   envoltorio de comandos. La respuesta correcta no era relajar el test, sino
   separar las dos superficies nativas —comandos y habilidades— en dos celdas.

4. **Casi rompo Claude Code sin que nada fallara a la vista.** Al enrutar
   `user-prompt-submit` por el traductor usé `detectDialect`, que devuelve
   *neutral* cuando no se fuerza nada — y Claude Code lo invoca **sin** bandera.
   El hook seguía imprimiendo algo, así que no parecía roto: simplemente Claude
   dejaba de recibir su inyección por turno. Lo atraparon los tests de
   integración existentes. El defecto ahora es Claude explícitamente, y hay un
   test que lo fija.

### [4] Defecto de `--agents` — COMPLETO

El valor por defecto era `opencode,claude`, así que `mem setup-mcp --scope global`
sin argumentos **nunca registraba Codex**, pese a que `globalScopeAgents` sí lo
declaraba: quien no supiera pasar `--agents codex` se quedaba sin su ciclo de
memoria entero, y nada lo advertía porque el comando terminaba en éxito.

- [x] Extraído a la constante `defaultAgentList = "opencode,claude,codex"`.
- [x] `TestAgentesPorDefecto_IncluyenCodex` fija la invariante que lo sostiene:
      todo agente del defecto debe existir en `globalScopeAgents`, para que ese
      valor no pueda volver a prometer lo que el flujo no cumple.

## Aplicado en la máquina real

- Binario compilado instalado en `~/.local/bin/mem`.
  Respaldo del anterior: `~/.local/bin/mem.backup-20260829091022`.
  **Revertir con `rm -f` antes del `cp`** (ver cierre de macOS más abajo).
- `mem setup-mcp --scope global` ejecutado. Respaldo del config de Codex:
  `~/.codex/config.toml.pre-setup-20260829091120`.
- Estado verificado: 3 agentes × 3 habilidades, y el ciclo de Codex con sus
  cuatro enganches, incluido `mem hook user-prompt-submit --emit=text`.

Falta solo reiniciar Codex: pedirá **autorizar el hook nuevo** y guardará su
`trusted_hash` (gomemory nunca lo calcula a mano).

## Escritura no atribuida — RESUELTA

El `config.toml` ya contenía el hook nuevo antes de aplicar el setup (mtime
09:00:24). Una primera medición dio «la suite está limpia» y **era falsa**; el
origen resultó ser precisamente la contaminación del HOME por la suite. Ver la
sección final de este documento.

---

## Cierre del hallazgo de macOS (regla 7)

El bug de firma por inodo no se quedó en anécdota de esta sesión: se comprobó si
afectaba a los caminos de instalación reales del producto.

- **`cmd_update.go`** — a salvo. Hace `os.Rename` del binario actual antes de
  escribir el nuevo, así que el reemplazo cae siempre en un inodo nuevo.
- **`scripts/install.sh`** — tenía el defecto en el fallback. Medido en macOS:
  `install` crea inodo nuevo (seguro), `cp` directo lo reutiliza. El fallback
  solo salta cuando `install` falla, o sea justo cuando el usuario ya está en
  terreno frágil. **Corregido** con un `rm -f` previo al `cp`.

El síntoma sin este arreglo era un `mem` que muere con SIGKILL en cada
invocación, sin mensaje y con `codesign -v` dando la firma por válida.

---

## Contaminación del HOME por la suite — CORREGIDO

`go test ./...` modificaba el `~/.codex/config.toml` real de quien ejecutaba.
Ésa fue la escritura de las 09:00:24 que quedó sin atribuir más arriba.

**Causa**: los tests lanzan el binario como subproceso (`exec.Command(bin,
"install", target)`) fijando `cmd.Dir` pero no `cmd.Env`, así que el hijo hereda
el HOME del proceso de test. `t.Setenv("HOME", …)` protege al código
in-process, no a un subproceso sin `Env` explícito.

**No era un defecto del producto**: `setupCodex` cablea Codex en ámbito de
usuario a propósito —Codex no tiene equivalente por proyecto—, y lo dice por
pantalla. Lo que fallaba era ejercer esa ruta contra el HOME de verdad.

**Corregido** en los `TestMain` de `tests/contract` y `tests/integration`:
aíslan HOME y USERPROFILE además del `GOMEMORY_DATA_HOME` que ya aislaban.
Se eligió TestMain sobre parchear los 40+ puntos de lanzamiento porque basta que
uno nuevo lo olvide para que la contaminación vuelva sin aviso.
`anclarCachesDeGo()` fija GOCACHE/GOMODCACHE antes de mover HOME, o cada
ejecución recompilaría el mundo.

**Verificado dos veces**: con HOME de sacrificio (3 hooks antes, 3 después) y
contra el HOME real por mtime (sin cambios).

### Propuesta descartada tras verificar

Se iba a extender `TestC5` para cubrir `CmdInstall`. Habría codificado una
invariante falsa: que `mem install` escriba `~/.codex/config.toml` es deliberado.

### Trampa de medición que casi cierra el caso en falso

La primera comprobación («la suite está limpia») dio verde y era **falsa**: se
midió con el hook ya presente, y `ensureCodexGomemoryHooks` es idempotente, así
que no había cambio que detectar. Para medir contaminación hay que partir de un
estado que **obligue** a escribir.
