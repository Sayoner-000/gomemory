# Auditoría Defensiva Multilenguaje (ronda 3) — 2026-10-17

- **Revisión auditada:** `a75ca31fac331756569d07ed4121f67ff1c5a6b7` — **árbol SUCIO** (44 modificados + sin seguimiento). Respecto a la ronda 2, el árbol **sí cambió**: se incorporaron los cierres de sus hallazgos H-01 y H-02.
- **Cambios relevantes desde la ronda 2 (superficie nueva a auditar):** `domain/terminal_text.go` (**nuevo**), `domain/terminal_text_test.go` (**nuevo**), `adapters/secondary/persistence/memory_terminal_test.go` (**nuevo**), `adapters/secondary/persistence/memory.go` (guarda en lectura), `installer/src/bootstrap.ts` (topes de tamaño), `installer/test/bootstrap.test.mjs`, `scripts/install.sh` (topes `--max-filesize`).
- **Alcance:** el código nuevo/modificado de esta ronda + superficies aún no auditadas en profundidad: guardián de texto de terminal y **todas** sus rutas de render, subsistema de compresión/omisiones (`compression_originals` y refs), cierre del protocolo de revisión (`finalize_review`, CAS), y el acotado de `mem hook` por stdin.
- **Modo:** solo lectura; sin correcciones de producción. Este documento es el único archivo creado.

## 1. Resumen de salud del código

**Nivel de riesgo general: Bajo.**

**Evaluación breve:** los cierres de la ronda 2 son sólidos. `domain.StripTerminalControls` está bien construido (elimina C0 salvo `\t`/`\n`, DEL y C1; normaliza CRLF y convierte UTF-8 inválido en U+FFFD, de modo que un byte C1 crudo de 8 bits no se cuela) y se aplica **en lectura** (`finishScannedMemory`, llamado en las 5 consultas de memoria), así que cubre también filas escritas por versiones anteriores y a todos los consumidores de memorias (CLI, TUI, MCP) — con prueba dedicada (`memory_terminal_test.go`). El instalador ya acota el tamaño (`download` con tope por `Content-Length` y por acumulado; topes de entrada y de binario en la extracción; `--max-filesize` en el shell) y su suite pasa 24/24. El subsistema de omisiones identifica por hash de contenido con resolución de colisiones y expulsión LRU, y todo su SQL es parametrizado. El cierre de revisión deriva el veredicto de estado persistido con comparación-y-cambio y un orden de lecturas deliberado (marca antes que datos). El hallazgo de esta ronda es de profundidad de la defensa: **el guardián de terminal quedó aplicado solo a las memorias, no a los demás textos almacenados que también se imprimen**.

**Resultado:** 1 hallazgo (0 críticos, 0 altos, 0 medios, 1 bajo).

### Cobertura y validación (esta ronda)

| Comprobación | Resultado |
|---|---|
| `go build ./...` + `go vet ./...` | Pasa |
| `go test ./...` (completo) | Pasa (0 fallos) |
| `go test -race` en `persistence` y `usecases` | Pasa |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | Pasa — 46/46 |
| `node --test installer/test/*.mjs` | Pasa — 24/24 |
| `npx tsc --noEmit` (instalador) | Pasa |
| `npm audit --omit=dev` (instalador) | Pasa — 0 vulnerabilidades |
| `bash -n` sobre `scripts/*.sh` | Pasa |
| Cierre de la ronda 2 — H-01 (escapes de terminal en memorias) | **Efectivo** — `domain/terminal_text.go:14-28`; `finishScannedMemory` (`memory.go:832-841`) invocado en `memory.go:516,556,612,653,685,854`; prueba `memory_terminal_test.go` (fila insertada cruda → lectura sin secuencias) |
| Cierre de la ronda 2 — H-02 (sin cota de tamaño) | **Efectivo** — `bootstrap.ts:14-16` (topes), `:20-44` (`download` corta por `Content-Length` y por acumulado), `:79-95` (tope del binario en zip/tar); `install.sh:26-27,128,199-201` (`--max-filesize`); suite instalador 24/24 |
| Acotado de `mem hook` por stdin | Pasa — `readBounded(os.Stdin, hookStdinMax)` (`cmd_hook.go:1266`, helper `:1285`) |

## 2. Hallazgo

| ID | Severidad | Estado | Problema |
|---|---|---|---|
| H-01 | Baja | CERRADO | El guardián de secuencias de control cubre solo las memorias: los textos de Socratica, los resúmenes de sesión y los hallazgos de revisión se siguen renderizando con C0/C1 |

### H-01 — Cobertura parcial del guardián de terminal (CWE-150 residual)

- **Categoría:** Defensa en profundidad incompleta / inyección de escape.
- **Severidad:** Baja.
- **Estado:** CONFIRMADO (por lectura; determinista).
- **Ubicación (aplicación del guardián):** `domain/terminal_text.go:14-28` (`StripTerminalControls`) se usa **únicamente** en `adapters/secondary/persistence/memory.go:837-841` (dentro de `finishScannedMemory`). No hay ninguna otra llamada en producción (búsqueda global: solo `memory.go` y la definición).
- **Puntos de render NO cubiertos:**
  - **Socratica (texto del modelo o del agente):** `domain/socratica.go:206,216` fija `ProposedPrompt = ev.Proposed`; se persiste y se lee con `domain.ProtectText` únicamente (`persistence/socratica.go:193` escritura, `:241` lectura de flujo, `:463` lectura de historial); `application/usecases/socratica_flow.go:189` guarda `OriginalPrompt` también solo con `ProtectText`. Se imprimen por `sio.printf` (`cmd_socratica.go:48`, que aplica `ProtectText`) en `cmd_socratica.go:351,397,446-451,544,588,695,741,915`, y en la TUI (`tui/socratica.go:531,808,877`, también solo `ProtectText`).
  - **Resúmenes de sesión:** `cmd_session.go:101` (`mem session end -s`, `ProtectText`) y `cmd_session.go:135` (`mem session list`, `ansi.Truncate` — conserva las secuencias).
  - **Hallazgos de revisión (texto del agente):** `cmd_review_query.go:163` imprime `finding.Claim` con `fmt.Printf` en crudo; la lectura aplica `ProtectText` (`persistence/review.go:1191`) pero no el guardián de terminal.
- **Análisis:** la ronda 2 corrigió la clase solo en la lectura de memorias. Los textos anteriores (propuesta de Socratica, original del prompt, preguntas, resumen de sesión, `claim`/`evidence` de revisión) son **contenido generado por el modelo o aportado por el agente** y se muestran en terminal por CLI y TUI sin neutralizar `ESC`/C0/C1. Un texto con `\x1b]52;c;<base64>\x07` (portapapeles), `\x1b]0;…\x07` (título) o `\x1b[…` (cursor/estado) sigue emitiéndose tal cual en `mem socratica`, `mem socratica history`, la pantalla SOCRATICA del TUI, `mem session end/list` y `mem review show`. `domain.ProtectText` neutraliza secretos y `<private>`, **no** secuencias de control.
- **Evidencia:**
  - Búsqueda global de `StripTerminalControls`: solo `memory.go:837-841` y `domain/terminal_text.go`.
  - `persistence/socratica.go:193,241,463` y `application/usecases/socratica_flow.go:189` usan exclusivamente `domain.ProtectText`.
  - `cmd_socratica.go:48`: `fmt.Fprint(s.out, domain.ProtectText(...))`; llamadas con `f.OriginalPrompt`/`f.ProposedPrompt` en las líneas citadas.
  - `cmd_session.go:101,135` y `cmd_review_query.go:163` (render sin guardián).
- **Código corregido (defensivo, mínimo):** enrutar todo texto almacenado por el mismo guardián que las memorias — alternativa (a): ampliar `finishScannedMemory` con funciones simétricas en los lectores de Socratica, sesiones y ledger de revisión; alternativa (b): aplicarlo en el choke point de render (`sio.printf`/`errorf`, helpers de `console.Panel`/`fit`, funciones de render del TUI y los `Printf` de `review`/`session`). La opción (b) cubre también texto que no pasa por persistencia.
- **Validación propuesta:** crear un flujo de Socratica cuyo `ProposedPrompt` contenga `"\x1b]0;PWNED\x07"` y comprobar que `mem socratica history` y la TUI lo muestran **literal**; análogos para `mem session end -s` y `mem review show`. Test de regresión al estilo de `memory_terminal_test.go`.

## 3. Superficies revisadas sin hallazgo (evidencia)

- **Guardián de terminal en memorias** (`domain/terminal_text.go`): el atajo (`utf8.ValidString && sin controles → s`) es correcto; `strings.Map` convierte bytes inválidos en U+FFFD (cubre un C1 crudo de 8 bits); CRLF→LF; DEL y C1 eliminados. Prueba con casos (`terminal_text_test.go`) y prueba de lectura con fila cruda (`memory_terminal_test.go`).
- **Instalador Node (topes):** `download` rechaza por `Content-Length` (`bootstrap.ts:25-28`) **y** corta por acumulado sin declararlo (`:33-42`); extracción con tope de binario (`:79-95`); extracción restringida al miembro exacto (sin zip-slip).
- **Instalador shell:** `--max-filesize` en la API y en el asset/checksums (`install.sh:26-27,128,199-201`); checksum verificado antes de extraer.
- **Compresión / omisiones:** `compression_originals.go` identifica por `sha256` del contenido con `freeRef` y escalado de longitud ante colisión (`:108-121`), TTL y expulsión LRU (`:169-197`); `RetrieveOriginal` normaliza `ref=` y todo el SQL es parametrizado.
- **Cierre de revisión:** `finalize_review.go` lee la marca de re-juicios **antes** que los datos del veredicto (`:66-73`), deriva el veredicto de estado persistido (`:91`) y escribe con comparación-y-cambio (`:117-127`, `ExpectedStatus/Round/Digest/RejudgmentMark/ReviewerResultsMark`), impidiendo reabrir o cerrar con estado obsoleto.
- **Hooks:** entrada por stdin acotada (`readBounded`) y errores a `stderr`; no se emite contenido del payload sin pasar por los helpers.

## 4. Observaciones sin hallazgo

- **Controles de formato Unicode no cubiertos:** `StripTerminalControls` no retira los controles de formato/bidireccionalidad (p. ej. U+202A–U+202E, U+2066–U+2069), que permiten **suplantación visual** del texto mostrado (clase «Trojan Source»). Está fuera del objetivo declarado del guardián (CWE-150, secuencias de terminal), por lo que se anota como observación, no como defecto.
- **`mem docs` imprime contenido crudo** (`cmd_docs.go:177`): si el contenido viene de la memoria, ya está cubierto por el guardián en la lectura; si viene del texto incrustado, es contenido del propio binario. Sin hallazgo.
- **H-03 de la ronda 2** (fallo intermitente de `go test -race ./adapters/primary/cli`) no se volvió a ejercer en esta ronda; sigue sin reproducción determinista.

## 5. Límites de esta ronda

- Herramientas no disponibles en el host: `govulncheck`, `staticcheck`, `gitleaks`, `semgrep`, `shellcheck`, `trufflehog`.
- Sin ejecución en Windows/PowerShell real; el instalador Windows se auditó por lectura y por su suite.
- Host con Python 3.9.6 (por debajo del mínimo declarado 3.12): `py_compile` no es representativo para scripts con sintaxis ≥3.12.
- Revisión dirigida por superficies; sin análisis de flujo exhaustivo (`staticcheck`).

## 6. Relación con rondas previas

- **Ronda 1** (`docs/audits/2026-10-17-defensive-qa.md`): H-01..H-05 (socratica/instalador/persistencia); H-04 descartado por ACR.
- **Ronda 2** (`docs/audits/2026-10-17-defensive-qa-2.md`): H-01 (escapes de terminal en memorias) y H-02 (topes del instalador) — **cerrados y verificados aquí**; H-03 (intermitencia `-race`) sigue abierto/informativo.
- **Ronda 3** (este documento): un hallazgo bajo que extiende la cobertura del guardián de la ronda 2 a los textos que quedaron fuera (Socratica, sesiones, revisión).

## Anexo — Cierre aplicado (2026-10-17)

**Criterio:** el guardián se aplica **al pintar, no al dato**. En Socratica los bytes son contractuales: el dominio valida la opción elegida por coincidencia exacta (`domain/socratica.go:188`) y el original y el prompt aprobado se entregan byte a byte (FR-016). Depurar el flujo al leerlo, como en las memorias, habría roto ambas garantías. Se limpia solo lo que llega a una terminal.

| Superficie | Cierre |
|---|---|
| CLI `mem socratica` | `socraticaIO.printf/errorf` pasan sus argumentos `string`/`error` por `StripTerminalControls` cuando el destino es una terminal (`outScreen`/`errScreen`); una tubería recibe los bytes exactos. Los paneles ya estilizados se marcan con el tipo `rendered` y su contenido se limpia antes de enmarcarlo (falla cerrado: un panel sin marcar pierde el estilo, no la protección). El launcher de `--print` escribe a través de `terminalSafe` solo si stdout es una terminal. La copia OSC 52 se limpia |
| UI de consola (toda la app) | Decorador `console.safeUI` en `NewPlain`/`NewRich`: título, `Label` y `Hint` sin controles en `Select`/`MultiSelect`/`Confirm`/`TypedConfirm`. La selección devuelve el `Value` intacto y la respuesta se reconstruye desde el flujo real |
| TUI SOCRATICA | `firstLine` (lista), texto de preguntas y opciones, `socraticaCard` (original/propuesta/síntesis), vista estrecha, `m.soc.err`, campo de edición y portapapeles |
| Mensajes de fallo | `ExplainFailure` (generador y fallback del caso de uso): el mensaje es para mostrar y puede traer stderr del harness |
| `mem session end/list` | Resumen mostrado y resumen truncado (`ansi.Truncate` conservaba los escapes) |
| `mem review show/status/history` | `show` escribe todo su informe por `terminalSafe` (cubre `Claim`, `IndependenceReason`, `Revision`, `Scope`, `ModifiedPaths` y cualquier campo futuro); `status` e `history` limpian `Target.Revision` |

**Validación** (cada prueba se ejecutó primero sin el cierre y falló):
- `TestSocratica_PantallaSinSecuenciasYEntregaExacta`: en terminal, sin ESC/BEL y con el texto visible conservado. La opción elegida (`"Equipo \x1b[31m"`) y el prompt aprobado se guardan exactos; por tubería, el prompt aprobado sale byte a byte.
- `TestSocraticaTUI_PintaSinSecuencias`: lista, preguntas y detalle sin OSC/BEL/`\x1b[2J`, y pulsar `1` guarda la opción con sus bytes.
- `TestSafeUI_PintaSinSecuenciasYDevuelveElValor` y `TestTerminalSafe_EscrituraPorTrozos`.
- Binario real con `GOMEMORY_DATA_HOME` aislado: `mem session end -s` y `mem session list` muestran `]0;PWNED` como texto, sin OSC/BEL.
- `go vet ./...` y `go test ./...` completos; `-race` en `cli`, `tui`, `console`, `generator` y `usecases`.

**Límite:** `mem review show` se validó por la prueba unitaria del writer y porque la función ya no contiene ninguna escritura directa. No hay prueba de extremo a extremo, porque crear una revisión con hallazgos exige el protocolo MCP completo de dos revisores.
