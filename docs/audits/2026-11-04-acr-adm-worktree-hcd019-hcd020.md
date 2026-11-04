# Auditoría ACR + ADM — Worktree Post-HCD-019 & HCD-020

**Fecha:** 2026-11-04  
**Revisión base:** `351be65aa4657f1c16aba90147faebaf95cd7943` (`v2.44.3`)  
**Árbol de trabajo auditado:** `351be65+worktree`  
**Digest del target:** `1795c9ea1674524fd0e47dea879e39b670d97c39c093929c5c464b6e66624c87`  
**Identificador de revisión ACR:** `acr_ba79bcd4-8aca-44e9-a242-d481361f2794`  
**Veredicto Final ACR:** **APPROVED** (`findings_total: 0`, `fix_rounds: 0`)  

---

## 1. Objetivo y alcance

Se auditó de forma exhaustiva el contenido del worktree actual, el cual comprende dos unidades de evolución y corrección defensiva sobre el release `v2.44.3`:

1. **HCD-019 (Cierre formal de hallazgos ADM-06 y ADM-07):**
   - **ADM-06:** Saneamiento de comillas dobles externas en argumentos del transcript de Antigravity CLI (`CommandLine` y `Path` vía `cleanAgyArg` en `adapters/primary/cli/hook_agy.go`). Permite que `filepath.Ext` identifique correctamente `.go` y ejecute la reindexación incremental en turnos de Antigravity.
   - **ADM-07:** Flexibilización de la comparación de versión en `adapters/primary/setup/agy_setup.go` (`isAgyVersionCompatible`) admitiendo la serie semántica `1.4.x` sin degradar falsamente `mem doctor` a `StateOutdated`.
   - **Cliente HCD:** Inclusión nativa de `"antigravity"` y `"gemini"` en el mapa de clientes válidos del CLI de HCD (`adapters/primary/cli/hcd/execute.go`).
2. **HCD-020 (Conexión de CodeGraph en Agy, diagnóstico de CBM y validación real del compresor):**
   - **G1 (CodeGraph MCP en Agy):** Incorporación de `"agy"` (`~/.gemini/config/mcp_config.json`) en `codeGraphMCPState`, `codeGraphMissing`, `codeGraphRegistered` y diagnóstico en `mem doctor`.
   - **Q1 (Targets explícitos en instalador):** Sustitución de `codegraph install --target auto` por `codeGraphInstallArgs` con targets explícitos (`claude,codex,opencode,antigravity`) para evitar que el instalador de CodeGraph genere archivos ajenos no deseados (`~/.gemini/GEMINI.md` y `~/.gemini/settings.json`).
   - **G2 (Diagnóstico de Codebase-Memory MCP):** Advertencia de `mem doctor` cuando falte `codebase-memory-mcp` en el `mcp_config.json` de Agy y el binario exista en el `PATH`, sugiriendo el comando manual correspondiente.
   - **Preservación e Idempotencia:** `InstallAgyGlobal` conserva entradas ajenas y servidores MCP existentes (`codegraph`, `codebase-memory-mcp`) sin reescribir `mcp_config.json` en ejecuciones subsecuentes.
   - **Documentación:** Actualización de `docs/MANUAL.md`.

---

## 2. Metodología y límites

- **Auditoría Defensiva (ADM):** Análisis estático y dinámico enfocado en programación defensiva: validación de entradas no confiables (transcripts, payloads JSON externos), prevención de fugas de descriptores, inyección de comandos en invocaciones a subprocesos y fail-safe de hooks ante fallos.
- **Consenso Adversarial (ACR):** Ejecución del protocolo estandarizado en la base de datos de gomemory (`review_start` → `review_submit` [A y B] → `review_consensus` → `review_finalize`).
  - **Revisor A (Funcional / Regresiones):** Verificación de contratos observables, compatibilidad de prefijos de versión, idempotencia de configuración e integridad de la suite existente.
  - **Revisor B (Defensivo / ADM):** Saneamiento de cadenas, manejo seguro de rutas (`filepath`), seguridad de argumentos en `exec.CommandContext`, ausencia de inyecciones y no contaminación de archivos del host (`GEMINI.md`).
  - **Nivel de independencia:** **Parcial / Degradado** (`degraded: same-model`). Ambos roles fueron ejecutados secuencialmente en el mismo entorno de modelo (`gemini-3.8-flash`) manteniendo aislamiento estricto de contexto entre las fases de recolección previas al ledger de consenso.

---

## 3. Validación Mecánica del Host

Todas las comprobaciones mecánicas pasaron al 100% sin advertencias ni fallos:

| Comprobación | Ámbito | Resultado |
|---|---|---|
| `go build ./...` | Todo el workspace | **Pasa** (código 0, sin advertencias) |
| `go vet ./...` | Todo el workspace | **Pasa** (código 0, sin advertencias) |
| `go test ./... -count=1` | Suite completa (unitarios, integración y contratos) | **Pasa** (100% de tests OK) |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | Plugin dual OpenCode v1+v2 | **Pasa** (66/66 pruebas OK) |
| `bash scripts/test-install-uninstall.sh` | Instalador y desinstalador | **Pasa** (limpieza y permisos OK) |
| `bash scripts/test-purge-physical.sh` | Purga física y sanitización SQLite | **Pasa** (0 fugas de texto en disco) |
| `bash -n scripts/install.sh` | Sintaxis POSIX sh del instalador | **Pasa** (sintaxis válida) |

---

## 4. Evaluación de los Revisores

### Revisor A: Correctitud Funcional y Regresiones
1. **Limpieza de argumentos (`cleanAgyArg`):** Elimina eficazmente las comillas envolventes dobles preservando rutas complejas y sin alterar argumentos internos. Cubierto por `TestAgyCleanArgQuotes`.
2. **Compatibilidad de versión (`isAgyVersionCompatible`):** Soporta `1.4.12` y cualquier parche menor `1.4.x`, manteniendo la guarda estricta frente a incrementos mayores (`2.0.0`) según la Constitución. Cubierto por `TestInspectAgyEntry_FlexibleVersion`.
3. **Targets explícitos de CodeGraph:** La función `codeGraphInstallArgs` mapea de forma determinista los agentes faltantes a los identificadores reconocidos por el instalador de CodeGraph (`"agy"` → `"antigravity"`). Cubierto por `TestEnsureCodeGraphMCP_TargetsExplicitos`.
4. **Idempotencia de instalación:** `InstallAgyGlobal` respeta las entradas preexistentes en `mcp_config.json` y no produce modificaciones innecesarias en segundas pasadas. Cubierto por `TestInstallAgyGlobal_ConservaMCPAjenos`.
5. **Veredicto Revisor A:** `status: success`, 0 defectos.

### Revisor B: Lente Defensiva (ADM)
1. **Inyección de argumentos y comandos:** `codeGraphInstallArgs` construye un slice de argumentos directos pasado a `exec.CommandContext`, sin pasar por intérpretes de shell (`sh -c` o `bash -c`), anulando riesgos de inyección de comandos.
2. **Lectura segura de archivos de configuración:** `agyMCPServer` y las funciones auxiliares de lectura de JSON gestionan adecuadamente los errores de `os.ReadFile` y `json.Unmarshal`, degradando de forma segura (`ok=false`) sin pánicos ni excepciones no controladas.
3. **No contaminación del entorno:** La eliminación de `--target auto` en favor de targets explícitos previene la creación espuria de `~/.gemini/GEMINI.md` y `~/.gemini/settings.json`, cumpliendo con la preferencia técnica de aislamiento.
4. **Resiliencia ante timeouts:** Las llamadas a instaladores externos (`ensureCodeGraphMCP`) cuentan con presupuestos acotados (`context.WithTimeout(2*time.Minute)`).
5. **Veredicto Revisor B:** `status: success`, 0 defectos.

---

## 5. Ledger de Consenso y Estado de Hallazgos

| ID | Origen | Estado Previo | Estado Actual | Resolución y Verificación |
|---|---|---|---|---|
| **ADM-06** | Auditoría v2.44.3 | Confirmado | **CERRADO** | `cleanAgyArg` en `hook_agy.go` limpia comillas en `CommandLine` y `Path`. Verificado en `TestAgyCleanArgQuotes`. |
| **ADM-07** | Auditoría v2.44.3 | Confirmado | **CERRADO** | `isAgyVersionCompatible` en `agy_setup.go` admite parches `1.4.x`. Verificado en `TestInspectAgyEntry_FlexibleVersion`. |
| **G1 (CG)** | HCD-020 | Confirmado | **CERRADO** | Detección y conexión de CodeGraph MCP en `~/.gemini/config/mcp_config.json`. Verificado en `TestCodeGraphMCP_Agy`. |
| **G2 (CBM)**| HCD-020 | Confirmado | **CERRADO** | Advertencia clara en `mem doctor` si falta `codebase-memory-mcp` en Agy. Verificado en `TestDoctorCodebaseMemoryMCP_Agy`. |
| **Q1 (Auto)**| HCD-020 | Confirmado | **CERRADO** | Uso estricto de targets explícitos sin `--target auto`. Verificado en `TestEnsureCodeGraphMCP_TargetsExplicitos`. |

---

## 6. Veredicto Final y Conclusión

El proceso de Consenso Adversarial formal registrado en gomemory emitió el veredicto:
```
VERDICT: APPROVED
Identificador de revisión: acr_ba79bcd4-8aca-44e9-a242-d481361f2794
Hallazgos críticos o pendientes: 0
Rondas de corrección requeridas: 0
```

El worktree se encuentra en un estado técnicamente sólido, con todas las pruebas automatizadas en verde, documentación al día en `docs/MANUAL.md` y artefactos HCD (`HCD-019` y `HCD-020`) formalmente concluidos. Los cambios están listos para ser incorporados al historial git (`git commit`).
