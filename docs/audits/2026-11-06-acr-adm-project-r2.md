# Informe ACR + ADM del Proyecto (Ronda 2)

**Fecha:** 2026-11-06 (America/Bogota)  
**Revisión base:** `8383b0c4575ed5121e4cd8f5a69bef029ded7a3c`  
**Target auditado:** Árbol de trabajo (`8383b0c4...` + worktree), 1856 archivos versionados o no ignorados.  
**Target Digest:** `8147984f52ea3102e65a51ba0f686dec54899bc7d9c5e9d8b4ff491e742bdd1b`  
**Identificador de revisión ACR:** `acr_c624a6df-1a8b-49e4-9a58-46e1b963424c`  
**Modalidad:** Auditoría defensiva multilenguaje (ADM) e inspección adversarial (ACR) en modo solo lectura (`fix_authorized: false`). No se modificó código de producción ni se cerraron ramas.

---

## 1. Resumen Ejecutivo y Salud del Código

**Riesgo general: MEDIO-ALTO.**  
Se detectaron y corroboraron independientemente **2 defectos funcionales (0 críticos, 1 alto, 0 medios, 1 bajo)**:
1. **ALTO (ADM-12 / C-001):** Divergencia de sesión entre comandos CLI e interfaces MCP/hooks cuando existe terminal interactivo (`hostbind.TerminalID`). Rompe la supresión de contexto (`get_plan_context`) y el restablecimiento de entregas tras compactación en entornos con TTY controlador.
2. **BAJO (ADM-13 / C-002):** `TestLatestReleaseTag` y pruebas asociadas de actualización dependen de bind TCP de loopback mediante `httptest.NewServer`, fallando en entornos sandboxed estrictos sin permisos de red local.

Los cierres anteriores de la auditoría de la mañana (H-01 a H-03 en HCD-043/044/045 y ADM-08 a ADM-11 en `process_unix.go`, `process_windows.go`, `portability.go` y `hostbind_test.go`) permanecen intactos, robustos y sin regresión.  
La nueva implementación de **HCD-047** (subagente splash sin memoria en los 4 runtimes) fue auditada y sus pruebas unitarias, de contrato e infraestructura pasan al 100%.

### Veredicto del Ledger ACR: `ESCALATED`
El protocolo formal de consenso ACR registró dos revisores independientes (`Reviewer A` y `Reviewer B`), ambos corroborando los mismos hallazgos (2 CONFIRMED, 0 SUSPECT, 0 contradicciones). Dado que el hallazgo C-001 posee severidad **ALTA** y la revisión se ejecutó en modo solo lectura (`fix_authorized: false`), la máquina de estados aplica el principio *fail-closed* y deriva el veredicto terminal **`ESCALATED`**, requiriendo intervención humana explícita para autorizar o aplicar las correcciones.

---

## 2. Matriz de Validación y Cobertura Mecánica

| Comprobación | Ámbito / Comando | Resultado | Notas |
|---|---|---|---|
| Compilación Go | `go build ./...` | **PASA** | 0 errores |
| Linter Go | `go vet ./...` | **PASA** | 0 advertencias |
| Formato y Espaciado | `gofmt -l .` y `git diff --check` | **PASA** | Árbol limpio de estilo |
| Compilación cruzada Windows | `GOOS=windows go build ./...` | **PASA** | Compatibilidad binaria total |
| Pruebas Windows Workspace | `GOOS=windows go test -c ./adapters/secondary/hcdfiles/...` | **PASA** | Compilación de test limpia |
| Plugin OpenCode v1+v2 | `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | **PASA** | 68/68 pruebas OK (incluye splash child session) |
| Instalador TypeScript | `npm --prefix installer run typecheck` | **PASA** | 0 errores de tipo |
| Suite del Instalador | `npm --prefix installer test` | **PASA** | 24/24 pruebas OK |
| Scripts de Shell | `bash -n scripts/*.sh` | **PASA** | Sintaxis POSIX verificada |
| Suite de Dominio y Casos de Uso | `go test ./domain/... ./application/...` | **PASA** | 100% OK |
| Suite de Adaptadores CLI / Setup | `go test ./adapters/primary/cli/... ./adapters/primary/setup/...` | **PASA** | Requiere loopback para httptest (C-002) |
| Suite de Persistencia | `go test ./adapters/secondary/persistence/...` | **PASA** | 32.8s, tests de concurrencia y aislamiento OK |
| Suite de Contratos | `go test ./tests/contract/...` | **PASA** | 101.0s, todas las especificaciones conformes |
| Suite de Integración | `go test ./tests/integration/...` | **FALLA** | Fallan `TestPlanContextSuppression_Max` y `TestSessionDelta20Turns` debido a C-001 |

---

## 3. Hallazgos Detectados en la Auditoría

### C-001 / ADM-12 — Divergencia de sesión por `TerminalID` entre CLI y MCP/Hooks
- **Categoría:** Concurrencia, gestión de sesiones e integridad de contexto.
- **Severidad:** **ALTA** (Impacto directo en facturación y economía de tokens).
- **Ubicación:** `infrastructure/container.go:420-432`, `adapters/secondary/hostbind/resolver.go:43-56`, `adapters/secondary/persistence/session.go:115-150`.
- **Ruta de alcance:** `mem session start` (CLI) → `ActiveSession` (MCP `get_context` o Hook `session-start` / `post-compact`).
- **Análisis de causa raíz:**
  1. En `infrastructure/container.go:426-430`, para los canales `"cli"` y `"tui"`, `conversationFor` asigna `local = hostbind.TerminalID()`.
  2. Si el proceso se ejecuta en un terminal interactivo o con un TTY asociado (ej. `ttys017`), `TerminalID()` devuelve `"terminal-ttys017"`.
  3. Al ejecutar `mem session start` sin flag `--conversation`, `SessionRepo.Start(project)` crea la sesión con `conversation_id = 'terminal-ttys017'`.
  4. Cuando un proceso desasociado de terminal (ej. el servidor MCP lanzado por el IDE mediante pipe, o un hook invocado sin TTY y sin `session_id` explícito) llama a `conversationSessionID`, `conv.Current()` devuelve `""` porque no hay terminal ni hostbind publicado.
  5. `ActiveConversationSession(db, project, "")` delega en `ActiveSession(db, project)`, el cual ejecuta estrictamente:
     ```sql
     SELECT ... WHERE project = ? AND ended_at IS NULL AND conversation_id IS NULL
     ```
  6. Como la sesión creada por el usuario tiene `conversation_id = 'terminal-ttys017'`, `ActiveSession` devuelve `nil`.
  7. En consecuencia, el servidor MCP y los hooks creen que **no hay sesión activa**. `DeliveryLog.Record` no anota las entregas y `DeliveryLog.Last` devuelve vacío, provocando que `get_plan_context` reenvíe siempre el historial completo (duplicando hasta 6.000 tokens por turno) y que `post-compact` no restablezca el estado de entrega para el canal del terminal.
- **Evidencia reproducible:**
  Ejecutar con TTY activo:
  `go test ./tests/integration -run 'TestPlanContextSuppression_Max|TestSessionDelta20Turns' -v`
  Ambas pruebas fallan por no suprimir el historial entregado en MCP/hooks y no sincronizar la compactación.
- **Propuesta de cierre:**
  1. En `cmd_session.go`: Si `convID == ""` (no se especificó `--conversation`), el comando manual de terminal debe iniciar la sesión compartida del proyecto con `conversation_id IS NULL`, permitiendo que herramientas desacopladas (MCP, hooks de fondo) compartan la sesión como documenta la arquitectura:
     *"el MCP y los hooks comparten la sesión del proyecto sin conversación, como antes"*.
  2. Alternativamente, en `ActiveSession`, si no existe sesión con `conversation_id IS NULL`, evaluar si la sesión de la terminal actual puede enlazarse como fallback explícito.
  3. En `tests/integration`: asegurar que el arnés de pruebas asigne explícitamente el contexto de sesión o desconecte la identidad de terminal host mediante entornos aislados (`TerminalID` vacío o variable de entorno de prueba).

---

### C-002 / ADM-13 — `httptest.NewServer` requiere loopback TCP en pruebas de actualización
- **Categoría:** Entorno de pruebas y programación defensiva.
- **Severidad:** **BAJA**.
- **Ubicación:** `adapters/primary/cli/cmd_update_test.go:40-78` (`TestLatestReleaseTag`, `TestDownloadFileRejectsOversizedBody`).
- **Análisis de causa raíz:**
  Las pruebas unitarias de cliente HTTP levantan instancias reales de `httptest.NewServer`. En entornos fuertemente aislados (sandboxes de CI o harnesses sin permiso de `bind` en loopback `127.0.0.1`), las llamadas fallan con error de red antes de ejecutar el código bajo prueba.
- **Evidencia reproducible:**
  Ejecución en sandbox estricto de `go test ./adapters/primary/cli -run TestLatestReleaseTag` produce fallo inmediato de conexión local; pasa al autorizar acceso loopback.
- **Propuesta de cierre:**
  Reemplazar `httptest.NewServer` por un `http.RoundTripper` simulado (mock transport) inyectado directamente en el cliente HTTP (`&http.Client{Transport: fakeTransport}`), eliminando la dependencia de sockets del sistema operativo.

---

## 4. Estado de Unidades HCD Recientes

- **HCD-043 (Evidencia HCD):** Protege la salida antes de recortarla en `workspace.go`. Verificado.
- **HCD-044 (Privacidad de metadatos):** `filepath` y `topic_key` blindados contra texto privado en inserción, deduplicación e importación. Verificado.
- **HCD-045 (Timeout de descendientes):** Control de grupos de procesos en Unix y `taskkill /T /F` en Windows con `WaitDelay`. Verificado.
- **HCD-046 (Fichas del historial en memoria):** Backfill y persistencia de decisiones y auditorías operativas. Verificado.
- **HCD-047 (Subagente splash sin memoria):** Soporte en los cuatro runtimes (Claude, OpenCode, agy, Codex) verificado en pruebas unitarias y de integración del plugin.

---

## 5. Veredicto Final y Pasos Recomendados

```
===================================================================
ACR LEDGER VERDICT: ESCALATED (Review ID: acr_c624a6df-1a8b-49e4-9a58-46e1b963424c)
ADM RISK: MEDIO-ALTO (1 defecto Alto, 1 defecto Bajo)
===================================================================
```

**Recomendaciones para el siguiente ciclo:**
1. Tratar **C-001 (ADM-12)** mediante una unidad fix rápida (fast-lane HCD) para desacoplar `mem session start` sin flags de la identidad fija de `TerminalID`, o permitir que MCP enlace con la sesión activa del proyecto.
2. Adoptar transportes en memoria para `cmd_update_test.go` para garantizar 100% de ejecuciones verdes en cualquier sandbox sin red.
3. Proceder al cierre formal de **HCD-047** (`/hcd-close`), dado que la funcionalidad del subagente splash está completamente verificada y no guarda relación con la divergencia de `TerminalID`.
