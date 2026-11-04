# Auditoría ACR + ADM — Worktree Post-HCD-022

**Fecha:** 2026-11-04  
**Revisión base:** `0ce92606550b06ce27e2fa72cf93b04c000ff96b` (`HCD-021`)  
**Árbol de trabajo auditado:** `0ce9260+worktree`  
**Digest del target:** `4a3c720aeefab0a1248bd51cb099292410e19cd571f069f3b36b417db4f8f3e4`  
**Identificador de revisión ACR:** `acr_b9fbfcbc-ee03-4afe-a5d8-3a5e764bf489`  
**Veredicto Final ACR:** **APPROVED** (`findings_total: 0`, `fix_rounds: 0`)  

---

## 1. Objetivo y alcance

Se auditó de forma exhaustiva el contenido del worktree actual correspondiente a la unidad de evolución **HCD-022 («Símbolos de estado uniformes con lipgloss en las validaciones de terminal»)** sobre la revisión base `0ce9260` (HCD-021):

1. **Fuente única de símbolos de estado (`adapters/primary/console`):**
   - Incorporación de `StateSelected` (`▸`) y `StateFlagged` (`⚑`) al enum `StepState`.
   - Exposición de `(l Layout) Mark(s StepState) string`, `Mark(s StepState) string` y `Glyph(s StepState) string`.
   - Integración con lipgloss v2 respetando la paleta de color en terminal estilizada y degradación sin secuencias ANSI en entornos sin TTY, con `NO_COLOR=1` o `TERM=dumb`.
   - Ampliación de `statusGlyph` en `adapters/primary/console/layout.go` para reconocer los ocho estados del juego (`○ ● ✓ ⚠ ✗ − ▸ ⚑`).
   - Retiro de la sustitución destructiva de emojis en `glyphs.Replace`.

2. **Sustitución de emojis en validaciones de la CLI (`adapters/primary/cli` y `setup`):**
   - Reemplazo de emojis de estado (`✅`, `❌`, `⚠️`, `ℹ️`, `➖`) y decorativos (`🔌`, `📦`, `🧹`, `📁`, etc.) en comandos del ciclo de vida (`setup-mcp`, `install`, `uninstall`, `update`, `install_cleanup`, `doctor`, `docs`, `constitution`, `seeds`).
   - Símbolo de advertencia uniforme: `⚠` (U+26A0 sin la variante emoji U+FE0F), con color Warning de la paleta.
   - Símbolo informativo uniforme: `○` (glifo neutro), con color Muted de la paleta.
   - Migración de llamadas directas a `fmt.Print*` a `console.Printf`, `humanf` y `humanln` con envoltorio `Layout` y paginación (`BeginPage`/`EndPage` en `setup-mcp`).

3. **Consistencia de estados y errores en la TUI (`adapters/primary/tui`):**
   - Sustitución del símbolo `✕` por el glifo canónico `✗` (`console.Glyph(console.StateFail)`).
   - Sustitución de `♥` por `⚑` (`console.Glyph(console.StateFlagged)`) para el tipo `Preference`.
   - Reemplazo de emojis decorativos y de aviso en vistas de mantenimiento, duplicados, importación y uso.

4. **Guarda automatizada contra emojis (`adapters/primary/console/emoji_guard_test.go`):**
   - Test de guarda `TestSinEmojis` que audita estáticamente todo el código fuente de producción en `adapters/primary/{cli,setup,console,tui}`, garantizando que no existan emojis del plano supletorio (U+1F300–U+1FAFF), selectores de variante (U+FE0F) ni caracteres prohibidos (`✅`, `❌`, `➖`, `ℹ`, `✕`, `♥`).

---

## 2. Metodología y límites

- **Auditoría Defensiva (ADM):** Inspección exhaustiva de programación defensiva:
  - Manejo seguro de cadenas y glifos UTF-8 multidimensionales / multibyte.
  - Prevención de desbordamientos en `statusGlyph` y `stateGlyph`.
  - Supresión estricta de secuencias de escape ANSI en tuberías, scripts y redirecciones (`NO_COLOR`, `TERM=dumb`, ausencia de TTY).
  - Preservación de contratos de prompt y datos de LLM (`application/usecases/build_context.go` y `domain/plan_shape.go` intactos).
- **Consenso Adversarial (ACR):** Ejecución del protocolo formal persistido en la base de datos de gomemory (`review_start` → `review_submit` [A y B] → `review_consensus` → `review_finalize`).
  - **Revisor A (Funcional / Regresiones):** Evaluación de contratos de salida de la CLI y TUI, compatibilidad con la paleta de colores, estructura visual de `Layout`, suite de pruebas unitarias y de integración.
  - **Revisor B (Defensivo / ADM):** Control de terminal, comportamiento ante entornos degradados o restringidos, preservación de pipelines Unix, robustez en parseo de estados y seguridad de dependencias.
  - **Nivel de independencia:** **Parcial / Degradado** (`degraded: same-model`). Ejecutado secuencialmente en el entorno del modelo (`gemini-3.8-flash`) manteniendo aislamiento estricto de contexto entre las fases previas a la síntesis de consenso.

---

## 3. Validación Mecánica del Host

Todas las comprobaciones mecánicas pasaron al 100%:

| Comprobación | Ámbito | Resultado |
|---|---|---|
| `go build ./...` | Todo el workspace | **Pasa** (código 0, sin advertencias) |
| `go vet ./...` | Todo el workspace | **Pasa** (código 0, sin advertencias) |
| `go test ./adapters/primary/console ./adapters/primary/cli ./adapters/primary/setup ./adapters/primary/tui` | Paquetes modificados en HCD-022 | **Pasa** (100% de tests OK) |
| `go test ./adapters/primary/console -run TestEmojiGuard_Detecta -count=1` | Guarda de emojis | **Pasa** (valida rechazo y permisos) |
| `go test ./adapters/primary/console -run 'TestSinEmojis/(cli|setup|console|tui)' -count=1` | Auditoría de emojis en producción | **Pasa** (0 emojis en código de producción) |
| `go test ./adapters/primary/cli -run TestCicloDeVida_SinFmtPrint -count=1` | Comprobación de `fmt.Print*` en ciclo de vida | **Pasa** (0 llamadas no envueltas) |
| `go test ./adapters/primary/cli -run TestSetupMCPGlobal_SimbolosSinEmoji -count=1` | Salida de `setup-mcp` | **Pasa** (glifos uniformes sin ANSI) |
| `go test ./adapters/primary/tui -run TestTUI_ErrorUsaMarcaDeConsole -count=1` | Glifos TUI | **Pasa** (usa `✗` y `⚑`, 0 `✕` y 0 `♥`) |
| `go test ./adapters/primary/cli -run 'Test(LatestReleaseTag|DownloadFileRejectsOversizedBody)'` | Tests con loopback | **Pasa** (0 errores) |
| `go test ./tests/integration -run 'TestUpdateIntegration|TestUpdateCheckDoesNotMutate'` | Integración update | **Pasa** (0 errores) |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | Plugin dual OpenCode v1+v2 | **Pasa** (66/66 pruebas OK) |
| `bash scripts/test-install-uninstall.sh` | Instalador y desinstalador | **Pasa** (limpieza y permisos OK) |
| `bash scripts/test-purge-physical.sh` | Purga física y sanitización SQLite | **Pasa** (0 fugas de texto en disco) |
| `bash -n scripts/install.sh` | Sintaxis POSIX sh del instalador | **Pasa** (sintaxis válida) |

---

## 4. Evaluación de los Revisores

### Revisor A: Correctitud Funcional y Regresiones
1. **Unificación de glifos (`Mark` y `Glyph`):** La API de `adapters/primary/console` proporciona una única fuente de verdad para los 8 estados del sistema. Las implementaciones en `adapters/primary/cli`, `setup` y `tui` convergen determinísticamente en esta fuente.
2. **Estructura visual de comandos de ciclo de vida:** `CmdMCPSetup`, `CmdInstall`, `CmdUninstall`, `CmdUpdate` y `CmdInstallCleanup` utilizan `humanf`/`humanln` o `console.Printf`, logrando paridad de diseño y jerarquía con `mem doctor`.
3. **Respeto a contratos de prompt:** Se confirmó que `application/usecases/build_context.go` conserva sus encabezados y emojis semánticos intactos (`## 🔥`, `## 🧭`, `## 🔗`, `## ⚠`), garantizando que los modelos de lenguaje reciban el formato de contexto esperado sin desviaciones.
4. **Comportamiento TUI:** Los iconos de tipo en `typeIcon` mapean fielmente `Preference` a `⚑` y `Bugfix` al `✗` estándar de `console`, eliminando la discrepancia histórica con `✕`.
5. **Veredicto Revisor A:** `status: success`, 0 defectos.

### Revisor B: Lente Defensiva (ADM)
1. **Degradación segura y control de escape ANSI:** `Layout.Mark` evalúa exhaustivamente `styledTerminal(l.env)` antes de invocar los estilos de `lipgloss`. En ausencia de TTY, o cuando `NO_COLOR` está presente o `TERM=dumb`, devuelve exclusivamente el glifo UTF-8 plano sin secuencias de escape ANSI, evitando la corrupción de registros o pipelines de integración continua.
2. **Sanitización del glifo de advertencia:** El glifo de advertencia se fija en `⚠` (U+26A0) excluyendo el selector de variación emoji (`\uFE0F`). Esto previene fallos de cálculo de ancho de línea en emuladores de terminal modernos que renderizan variantes de 2 celdas.
3. **Robustez ante estados desconocidos:** La función `stateGlyph` implementa un caso por defecto que retorna de forma segura `"○"` ante cualquier valor no contemplado de `StepState`, previniendo pánicos o desbordamientos.
4. **Prevención de regresiones mediante guardas mecánicas:** `TestSinEmojis` y `TestCicloDeVida_SinFmtPrint` actúan como candados de CI para evitar que futuros cambios reintroduzcan pictogramas o salidas no controladas directas a stdout.
5. **Veredicto Revisor B:** `status: success`, 0 defectos.

---

## 5. Ledger de Consenso y Estado de Criterios

| Criterio | Descripción | Estado | Resolución y Verificación |
|---|---|---|---|
| **C-01** | Símbolos de estado uniformes en `setup-mcp` | **SATISFECHO** | Usa `✓`, `✗`, `⚠` (sin U+FE0F) y `○` con colores de la paleta. Verificado en `TestSetupMCPGlobal_SimbolosSinEmoji`. |
| **C-02** | Degradación sin ANSI en `NO_COLOR` o no TTY | **SATISFECHO** | `Layout.Mark` emite glifos planos sin secuencias `\x1b[`. Verificado en `TestMark`. |
| **C-03** | Guarda estricta contra emojis en `adapters/primary` | **SATISFECHO** | `emojiGuard` audita `cli`, `setup`, `console`, `tui`. Verificado en `TestSinEmojis`. |
| **C-04** | TUI usa los mismos símbolos (`✗` y `⚑`) | **SATISFECHO** | Se retiraron `✕` y `♥`. Verificado en `TestTUI_ErrorUsaMarcaDeConsole`. |
| **C-05** | Ciclo de vida integrado con `Layout` | **SATISFECHO** | 0 llamadas directas a `fmt.Print*` en ciclo de vida. Verificado en `TestCicloDeVida_SinFmtPrint`. |

---

## 6. Veredicto Final y Conclusión

El proceso de Consenso Adversarial formal registrado en gomemory emitió el veredicto:
```
VERDICT: APPROVED
Identificador de revisión: acr_b9fbfcbc-ee03-4afe-a5d8-3a5e764bf489
Hallazgos críticos o pendientes: 0
Rondas de corrección requeridas: 0
Masa de validación mecánica: 100% PASS
```

El worktree cumple integralmente con todos los criterios de aceptación y especificaciones de **HCD-022**. Los cambios son seguros, defensivos, respetan los contratos externos y están listos para ser incorporados al repositorio.
