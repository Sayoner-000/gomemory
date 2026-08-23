# Contrato — Artefactos de `mem install` / `mem update`

**Componentes**: `cmd_install.go` (modificado), `cmd_install_cleanup.go` (nuevo)

---

## Contrato de artefactos: qué toca la instalación

| Ruta en el proyecto destino | Antes | Después |
|---|---|---|
| `mem` (binario) | crea | **crea** (sin cambio) |
| `.gitignore` | edita | **edita** (sin cambio) |
| `.claude/` (plugin, hooks, permisos) | crea | **crea** (sin cambio) |
| `.claude/skills/atomic-decomposition/` | crea | **crea** (sin cambio) |
| `.claude/skills/constitution/` | — | **crea** (nuevo, FR-027) |
| `.opencode/commands/atomic-decomposition.md` | crea | **crea** (sin cambio) |
| `.opencode/commands/constitution.md` | — | **crea** (nuevo, FR-027) |
| `.cursor/mcp.json` | crea | **crea** (sin cambio) |
| `.specify/extensions/gomemory-context/` | crea si hay spec-kit | **crea si hay spec-kit** (sin cambio) |
| `.memory/` | crea | **crea** (sin cambio) |
| `AGENTS.md` | crea/edita | **elimina con respaldo** |
| `CLAUDE.md` | crea/edita | **elimina con respaldo** |
| `CLAUDE.txt` | edita si existe | **elimina con respaldo** |
| `.cursorrules`, `.windsurfrules` | edita si existen | **no toca** |
| `speckit-constitution-gen.md` | crea | **elimina sin respaldo** |
| `.windsurf/mcp_config.json` | crea | **desregistra** |
| `.cline/mcp_settings.json` | crea | **desregistra** |

## Orden de operaciones en `CmdInstall`

```
1. Copiar binario
2. Inicializar/verificar memoria
2b. SeedDefaults                      <- NUEVO
3. Actualizar .gitignore
3b. cleanupLegacyArtifacts            <- NUEVO
4. (eliminado: AGENTS.md/CLAUDE.md)
4b. (eliminado: copia de la constitución)
4c. InstallSpeckitExtension           (sin cambio)
4d. InstallAtomicPlanWrappers         (sin cambio)
4e. InstallConstitutionWrappers       <- NUEVO
5. Configurar agentes: opencode, claude-code, cursor, codex
   (windsurf y cline eliminados de esta lista)
6. Aplicar auto-approve
7. Mensaje final                      <- REESCRITO
```

**Por qué la siembra va antes que la limpieza**: si algo falla al sembrar, los
archivos legados siguen ahí y la persona no se queda sin las reglas en ninguna de
las dos formas.

## Contrato del mensaje final

Debe nombrar dónde quedaron las reglas y la constitución, y **no** debe mencionar
archivos que ya no se generan:

```
🎉 gomemory instalado. Ahora puedes:

   cd <target>
   ./mem            # Abrir TUI
   ./mem --help     # Ver todos los comandos

   Las reglas de trabajo y la constitución quedaron guardadas en la memoria del
   proyecto: el agente recibe las reglas automáticamente en get_context() y aplica
   la constitución con /constitution. Ya no se generan archivos de instrucciones.
```

Se elimina la línea «el agente AI usará la memoria automáticamente al leer AGENTS.md»
y el aviso de v1.9 sobre `setup-mcp --scope global`.

## Contrato de `cleanupLegacyArtifacts(target string)`

| Garantía | Detalle |
|---|---|
| Idempotente | Segunda pasada: sin salida, sin cambios |
| No fatal | Ningún fallo interrumpe la instalación |
| Conservador con lo ajeno | Un JSON con otros servidores conserva esos servidores; un JSON ilegible no se toca |
| Trazable | Cada eliminación se informa; cada respaldo informa su ruta |
| Seguro ante fallo de respaldo | Si el respaldo no se puede escribir, el original NO se borra |

## Contrato del envoltorio `/constitution`

| Garantía | Detalle |
|---|---|
| Sin copia del texto | El cuerpo instruye resolver desde la memoria; jamás incrusta la constitución |
| Regenerado siempre | Se reescribe si difiere del embebido, igual que `InstallAtomicPlanWrappers` |
| Opcional | Un fallo avisa y no bloquea la instalación |
| Destinos | `.claude/skills/constitution/SKILL.md`, `.opencode/commands/constitution.md` |
