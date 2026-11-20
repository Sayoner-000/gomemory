# Informe ACR + ADM — Worktree HCD-058 y cambios locales

- **Fecha:** 2026-11-20
- **Revisión auditada:** `1676882d080415336becc7cb330f4884fae38567` (árbol sucio — worktree con 20 archivos modificados y 4 archivos nuevos)
- **Digest congelado del target:** `dc4432d28859d7fe8cd8ba0f57cb472ff765a30e447875ab2222f4cace37c3ac`
- **Review ID (ACR):** `acr_acd48877-11f1-4d60-be9d-b0216b84c7c5`
- **Independencia ACR:** `degraded` (mismo agente emulando Pase A y Pase B en modo solo lectura; no hay subagentes aislados disponibles en este runtime)
- **Alcance:** Diff completo del worktree y archivos nuevos (`HCD-058` — `mem hcd rename`, tool MCP `hcd_rename`, reconciliación en `HCD.Show`, `Workspace.MoveDoc`, `DecisionHistoryRepository.RetargetPath`, `domain/seed.go` Constitución 2.2.0, `infrastructure/plugin/opencode/gomemory.ts`, templates HCD/constitución y `README.md`).

---

## Parte I — Adversarial Consensus Review (ACR)

```text
ADVERSARIAL CONSENSUS REVIEW

Target:        worktree (HEAD 1676882d080415336becc7cb330f4884fae38567 + diff + untracked)
Revision:      1676882d080415336becc7cb330f4884fae38567+worktree (digest dc4432d28859d7fe8cd8ba0f57cb472ff765a30e447875ab2222f4cace37c3ac)
Independence:  degraded (pases A y B ejecutados secuencialmente por el mismo agente sin aislamiento de subagentes)
Review rounds: 1
Fix rounds:    0

Confirmed:
  - C-001 (MEDIUM): HCD.Rename / ReplaceUnitTitle no rechaza ni colapsa saltos de línea en title (ProtectHCDField preserva '\n'), permitiendo inyectar encabezados Markdown en hcd/<ID>-<slug>.md y desincronizar unit.Title frente a ParseUnitDoc.
  - C-002 (LOW): HCD.retarget descarta silenciosamente el error de PathRetargeter.RetargetPath (_ = r.RetargetPath(...)), pudiendo dejar memories.filepath y memory_files.path desincronizados en silencio.

Suspect:
  - C-004 (LOW): Ventana TOCTOU entre os.Lstat(dst) y os.Rename(src, dst) en Workspace.MoveDoc bajo ejecuciones concurrentes.

Contradictions: 0
Info:
  - C-003 (INFO): ReplaceUnitTitle no normaliza CRLF ni BOM UTF-8 antes de buscar '# ', a diferencia de ReplaceSection que sí normaliza '\r\n'.

Corrections:   0 (revisión en modo solo lectura; sin hallazgos CRITICAL/HIGH)
Re-judgment:   N/A
Verification:  go vet ./... (PASS), go test -race en paquetes afectados (PASS)

VERDICT: APPROVED
```

---

## Parte II — Auditoría Defensiva Multilenguaje (ADM)

### 1. Resumen de salud del código

- **Lenguajes / tecnologías detectadas:**
  - **Producción:** Go (`domain`, `application`, `adapters`), TypeScript (`infrastructure/plugin/opencode/gomemory.ts`), Markdown/Plantillas (`infrastructure/templates/...`, `README.md`), SQLite (`adapters/secondary/persistence`).
  - **Pruebas:** Go (`*_test.go` en `usecases`, `cli`, `cli/hcd`, `persistence`).
- **Nivel de riesgo general:** **Bajo** (sin hallazgos Críticos ni Altos; 1 hallazgo Medio de validación de entrada en borde y 2 hallazgos Bajos).
- **Evaluación breve:**
  La implementación de `HCD-058` respeta la arquitectura limpia (`domain` → `application/ports` → `usecases` → `adapters`) y mantiene la separación de bordes en CLI y MCP. Destaca positivamente:
  1. Validación estricta de rutas en `Workspace.resolve` (rechaza `..`, rutas fuera de `hcd/` y cualquier componente symlink).
  2. Rollback best-effort (`undo()`) en `HCD.Rename` si falla `WriteDoc` o `store.Save` tras mover el archivo.
  3. Transacción SQLite con consultas parametrizadas, filtro estricto por `project` y manejo de colisiones de clave en `RetargetPath` (`UPDATE OR IGNORE` + `DELETE`).
  4. Pruebas unitarias y de contrato dedicadas para CLI, MCP, casos de uso y persistencia.
- **Resultado:** 3 hallazgos accionables (0 críticos, 0 altos, 1 medio, 2 bajos) + 1 observación informativa.
- **Límites de la inspección:** No se ejecutaron `govulncheck`, `staticcheck`, `gitleaks` ni `tsc` por no estar instalados en el host; la auditoría se centró en el diff del worktree y los archivos nuevos respecto a `HEAD`.

**Cobertura y validación**

| Comprobación | Resultado |
|---|---|
| `go vet ./...` | Pasa (sin avisos) |
| `go test -race ./domain/... ./application/... ./adapters/primary/cli/... ./adapters/secondary/hcdfiles/... ./adapters/secondary/persistence/...` | Pasa (todos los paquetes en verde con `-race`) |
| `govulncheck ./...` | No disponible — binario no instalado en el host |
| `staticcheck` | No disponible — binario no instalado en el host |
| `gitleaks` | No disponible — binario no instalado en el host |
| `tsc --noEmit` | No disponible — `tsc` no instalado en el host |

---

### 2. Hallazgos y fugas detectadas

| ID | Severidad | Estado | Problema |
|---|---|---|---|
| `H-01` (`C-001`) | Media | **RESUELTO** | Inyección de encabezados Markdown y desincronización de `unit.Title` vía saltos de línea en `HCD.Rename` / `ReplaceUnitTitle` |
| `H-02` (`C-002`) | Baja | **RESUELTO** | Error tragado en `HCD.retarget` (`_ = r.RetargetPath(...)`) sin aviso de desincronización |
| `H-03` (`C-004`) | Baja | SOSPECHOSO (heredado) | Ventana TOCTOU entre `os.Lstat(dst)` y `os.Rename(src, dst)` en `Workspace.MoveDoc` |

---

#### H-01 — Inyección de encabezados Markdown y desincronización de título vía `\n` en `HCD.Rename`

- **Categoría:** Integridad / Validación de entrada en bordes
- **Severidad:** Media
- **Estado:** CONFIRMADO
- **Ubicación:** `application/usecases/hcd.go:446` (`HCD.Rename`), `domain/terminal_text.go:34-36` (`ProtectHCDField`), `domain/hcd_doc.go:424-436` (`ReplaceUnitTitle`).
- **Ruta de alcance:** `mem hcd rename --title "..."` o tool MCP `hcd_rename` (`adapters/primary/cli/cmd_mcp_history_tools.go:45-51`) → `usecases.HCD.Rename`.
- **Análisis de vulnerabilidad:**
  `ProtectHCDField` llama a `StripTerminalControls`, cuya función `isTerminalControl` excluye explícitamente `'\n'` y `'\t'`. Por ello, un `title` con `\n` o `\r` supera la validación de `HCD.Rename` y se interpola directamente en `ReplaceUnitTitle` como `"# " + title`. Si el título incluye `\n## Checklist\n- [x] ...`, inyecta secciones Markdown arbitrarias en `hcd/<ID>-<slug>.md`. Además, al volver a leer el documento con `ParseUnitDoc`, `doc.Title` solo captura la primera línea después de `# `, quedando permanentemente distinto de `unit.Title` en SQLite y provocando que cada `Show` detecte una divergencia falsa y reescriba `hcd_units`.
- **Evidencia:**
  - `domain/terminal_text.go:31`: `return r != '\t' && r != '\n' && (r < 0x20 || (r >= 0x7f && r <= 0x9f))`
  - `domain/hcd_doc.go:431`: `lines[i] = "# " + title` (sin normalizar ni rechazar `\n` o `\r`).
- **Código corregido (seguro y defensivo):**
  Colapsar saltos de línea en `HCD.Rename` (o rechazar títulos multilínea en el borde de dominio/caso de uso):
  ```go
  // En application/usecases/hcd.go (HCD.Rename)
  title := domain.ProtectHCDField(strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(in.Title))
  title = strings.Join(strings.Fields(title), " ")
  if title == "" {
      return HCDView{}, fmt.Errorf("%w: el nuevo título no puede estar vacío", domain.ErrValidation)
  }
  ```
- **Validación del cierre:**
  Test en `application/usecases/hcd058_rename_test.go` pasando `Title: "Título\n## Checklist\n- [x] 99. fake"` y verificando que el archivo resultante no contiene `## Checklist` extra y que `Show` posterior no marca divergencia.

---

#### H-02 — Error tragado en `HCD.retarget` al actualizar rutas en memoria

- **Categoría:** Éxito falso / Integridad
- **Severidad:** Baja
- **Estado:** CONFIRMADO
- **Ubicación:** `application/usecases/hcd.go:552-559` (`HCD.retarget`).
- **Ruta de alcance:** `usecases.HCD.Rename` y `usecases.HCD.Show` (cuando `loadDoc` reconcilia un archivo movido).
- **Análisis de vulnerabilidad:**
  `HCD.retarget` ejecuta `_ = r.RetargetPath(h.project, from, to)`. Aunque es intencional no revertir el `os.Rename` del documento si falla la actualización de memorias históricas, descartar el error en silencio impide que el operador o las pruebas detecten que `memories.filepath` y `memory_files.path` quedaron apuntando a la ruta obsoleta en SQLite.
- **Evidencia:**
  - `application/usecases/hcd.go:557`: `_ = r.RetargetPath(h.project, from, to)`
- **Código corregido (seguro y defensivo):**
  Propagar el error o adjuntar una advertencia en `HCDView` / devolver error controlado tras confirmar el guardado de la unidad para que el usuario pueda ejecutar `mem hcd index --apply` si la BD falló.
- **Validación del cierre:**
  Test con un mock de `PathRetargeter` que devuelva error y verificar que se reporta advertencia o error explícito en lugar de éxito silencioso.

---

#### H-03 — Ventana TOCTOU entre `os.Lstat` y `os.Rename` en `Workspace.MoveDoc`

- **Categoría:** Concurrencia / Integridad
- **Severidad:** Baja
- **Estado:** SOSPECHOSO
- **Ubicación:** `adapters/secondary/hcdfiles/workspace.go:80-85` (`Workspace.MoveDoc`).
- **Ruta de alcance:** `usecases.HCD.Rename` → `Workspace.MoveDoc`.
- **Análisis de vulnerabilidad:**
  `os.Lstat(dst)` comprueba si existe el destino antes de llamar a `os.Rename(src, dst)`. En POSIX `os.Rename` sobrescribe `dst` si aparece entre ambas llamadas bajo concurrencia. En un CLI local de usuario único la probabilidad es muy baja, pero en llamadas MCP paralelas podría sobrescribir un archivo recién creado con el mismo destino.
- **Evidencia:**
  - `adapters/secondary/hcdfiles/workspace.go:80-85`: `os.Lstat(dst)` seguido de `os.Rename(src, dst)`.
- **Código corregido (seguro y defensivo):**
  En plataformas Unix, usar `os.Link(src, dst)` seguido de `os.Remove(src)` (falla atómicamente con `EEXIST` si `dst` existe), manteniendo `os.Rename` solo como fallback donde `os.Link` no aplique.
- **Validación del cierre:**
  Test en `adapters/secondary/hcdfiles` verificando que `MoveDoc` falla si `dst` ya existe sin sobrescribir su contenido.

---

### 3. Recomendaciones de resiliencia

1. **Sanitización de línea única para títulos HCD:** Normalizar/colapsar `\r` y `\n` en `HCD.Start` y `HCD.Rename` (o mediante una función dedicada `domain.ProtectHCDSingleLine`) para garantizar que ningún título rompa la estructura Markdown de `hcd/<ID>-<slug>.md`.
2. **Normalización CRLF/BOM en `ReplaceUnitTitle`:** Igualar el comportamiento de `ReplaceUnitTitle` al de `ReplaceSection` (`strings.ReplaceAll(string(data), "\r\n", "\n")`) para tolerar ediciones manuales en Windows.
3. **Herramientas estáticas en entorno local:** Mantener `govulncheck` y `staticcheck` disponibles en el entorno de desarrollo/CI para cubrir análisis de vulnerabilidades alcanzables y reglas estáticas adicionales en Go.

---

### 4. Revalidación post-corrección (Follow-up ACR)

- **Follow-up Review ID:** `acr_ccd27925-af41-4cfb-8635-60386fa6fa08` (enlazado a `acr_acd48877-11f1-4d60-be9d-b0216b84c7c5`)
- **Digest revalidado:** `bd8a061c962c38c1a0765a94a21024cd9c935b040bcf2fb0acd9be92ee083744`
- **Independencia:** `degraded`
- **Correcciones aplicadas:**
  - `H-01` (`domain/hcd_doc.go`, `application/usecases/hcd.go`): `HCD.Rename` y `ReplaceUnitTitle` colapsan `\r\n`, `\n`, `\r` y espacios múltiples en el título; `ReplaceUnitTitle` también normaliza `\r\n` del documento. Verificado con `TestHCDRenameSanitizesMultilineTitle`.
  - `H-02` (`application/usecases/hcd.go`): `HCD.retarget` devuelve `error` y tanto `Rename` como `loadDoc` lo propagan con contexto explícito. Verificado con `TestHCDRenameReportsRetargetError`.
- **Verificación ejecutada tras los cambios:**
  - `go vet ./...` → Pasa
  - `go test -race -count=1 ./domain/... ./application/usecases -run 'TestHCDRename|TestHCDReconcile'` → Pasa
  - `go test -race ./adapters/primary/cli/hcd/... ./adapters/primary/cli -run 'TestHCDRename'` → Pasa
- **Veredicto final:** **`APPROVED`** (0 hallazgos nuevos, 0 regresiones).
