# Implementation Plan: Ciclo de vida de gomemory en consola — instalar, actualizar y desinstalar sin rastros

**Branch**: `034-console-lifecycle` | **Date**: 2026-09-26 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/034-console-lifecycle/spec.md`

## Summary

gomemory pasa a tener **un solo binario**. Si hay uno global en el PATH:
- `install` deja de copiarlo a cada proyecto;
- las copias antiguas se retiran, con un aviso, al abrir el proyecto;
- `./mem update` actualiza el global.

Además, la feature añade:
- Un **aviso de versión** al estilo Codex: consulta en segundo plano como mucho una vez cada 24 h, con una caché global, y aviso en `systemMessage`. `mem update` verifica el checksum.
- Una **consola guiada** al estilo skills.sh para `install`, `update` y `uninstall`: agentes detectados ya marcados, opción recomendada, resumen y confirmación, y flags equivalentes. Sin TTY nunca pregunta.
- Una **desinstalación sin rastros**, con alcance de proyecto o de sistema. Incluye inventario, exportación privada previa, registro de rutas más un escaneo acotado, y retirada selectiva de las entradas en archivos compartidos.
- Tres defectos que se corrigen por el camino:
  - `uninstall` no borraba la memoria del almacén global;
  - `export` escribía la memoria con 0644;
  - no existía `.env.example`.

La base técnica es solo código que ya existe: `binRefFor`, `ActivationInspector`, las funciones de `cmd_uninstall.go`, el patrón `MaybeRefresh`/`detach`, la escritura atómica de `writeSnapshot`, `latestReleaseTag`, `replaceSelf`, `ExportProject` y bubbletea/bubbles/lipgloss v2. Detalle y motivos en [research.md](research.md).

## Technical Context

**Language/Version**: Go 1.27 (`go.mod`, toolchain go1.27.1)

**Primary Dependencies**: biblioteca estándar (`crypto/sha256`, `net/http`, `os/exec`, `path/filepath`), `charm.land/bubbletea/v2`, `bubbles/v2` y `lipgloss/v2` (ya directas), y `github.com/charmbracelet/x/term` (ya en el grafo como indirecta; pasa a directa). **Cero módulos nuevos.**

**Storage**: archivos JSON o de texto pequeños, escritos de forma atómica con permisos 0600/0700: `<DataHome>/update-check.json`, `<DataHome>/projects/<key>/root` y `.memory/.update-notice`, más campos nuevos en `.memory/settings.json`. Sin cambios en el esquema SQLite.

**Testing**: `go test` más Testify. Pruebas unitarias junto al código; contrato de extremo a extremo en `tests/contract/`, con `HOME` y `GOMEMORY_DATA_HOME` temporales y un servidor de releases `httptest`.

**Target Platform**: macOS, Linux y Windows (amd64/arm64), los 5 destinos de goreleaser.

**Project Type**: CLI y servidor MCP (un solo binario).

**Performance Goals**: `session-start` sin latencia añadida por la consulta de versión (SC-006). Instalación guiada en menos de 1 minuto (SC-010).

**Constraints**: nunca red en el camino crítico de un hook; tiempo límite de 5 s en la consulta y de 2 s al identificar una copia; sin TTY nunca se lee stdin; los archivos compartidos solo se editan de forma parcial.

**Scale/Scope**: en la máquina de referencia, 121 proyectos en el almacén y 6 copias locales antiguas; el escaneo llega hasta 6 niveles bajo `~`.

## Constitution Check

*GATE: se evalúa antes de la fase 0 y se revisa después de la fase 1. Referencia: la constitución vigente servida desde la memoria (`mem docs show constitution`).*

| Regla | Estado | Nota |
|---|---|---|
| §3 Hexagonal: dominio sin I/O | ✅ | `Version`, `UpdateCheck`, las reglas de retirada y el TTL, en `domain/`. Los puertos `ReleasePort`, `UpdateCheckRepository` y `ProjectRegistryRepository`, en `application/ports/` ([data-model](data-model.md)) |
| §3 Toda dependencia externa por puerto | ✅ | La API de releases pasa detrás de `ReleasePort`. Hoy `cmd_update.go` hace HTTP directo desde el adaptador primario: se mueve a `adapters/secondary/release/` |
| §4 Composition root único | ✅ | El wiring de los adaptadores nuevos va en `infrastructure/container.go`. `install`, `update` y `update-check`, que se despachan sin contenedor, reciben las dependencias de una función raíz ligera en ese mismo paquete |
| §1.2 Todo I/O con `context.Context` | ✅ | Los puertos nuevos llevan `ctx` como primer parámetro |
| §8 Tiempo límite en llamadas externas | ✅ | 5 s en la consulta y en la descarga del checksum, 2 s al ejecutar `version` sobre una copia |
| §8 Fire-and-forget sin perder trabajo | ✅ | La consulta es cacheable y se reintenta al vencer el TTL. `last_error` queda visible en `doctor` |
| §9 Variables de entorno en `.env.example` | ✅ (cierra una brecha) | No existía `.env.example`. Se crea con las variables actuales más la nueva (R13) |
| §10 Pruebas deterministas | ✅ | Reloj (`ClockPort`), HTTP (`httptest`), `HOME` y `DATA_HOME` temporales. Nunca la red pública |
| Constitución III: cobertura ≥80 % | ✅ | `bash scripts/coverage.sh` mide la suite completa con `-coverpkg=./...` y une por bloque la cobertura del binario instrumentado que ejercitan las pruebas de contrato. Una ejecución limpia dio 80,8 %. La medición clásica sin instrumentar subprocesos era 56,7 % y omitía esas rutas. |
| §10 No modificar pruebas existentes | ✅ | Se mantiene la salida byte a byte de `session-start` sin avisos ([contrato](contracts/hook-session-start.md)). El cambio de comportamiento de `install` (sin copia si hay global) actualiza la spec y **sus** pruebas en el mismo cambio, como permite §10. Se listan en tasks |
| §15 Cadena de suministro | ✅ | Verificación SHA-256 con `checksums.txt` usando `crypto/sha256`, sin criptografía propia |
| §15 Scanner en CI | ✅ | El job `govulncheck` precede a `goreleaser` en `.github/workflows/release.yml`; un hallazgo detiene la publicación |
| §15 Mínimo privilegio y path traversal | ✅ | Permisos 0600/0700. El escaneo no sigue enlaces. La retirada solo actúa sobre archivos regulares verificados |
| §20 Sin dependencias nuevas | ✅ | Se descarta `huh` (R7) |
| §20 Sin URLs externas hardcodeadas | ✅ | Base de releases configurable (`GOMEMORY_RELEASE_DOWNLOAD_BASE`, ya existente) |
| §18 Documentación en español | ✅ | |
| Constitución: archivos obligatorios | ✅ | Se añadieron `docs/ARQUITECTURA.md` y `docs/DATABASE.md`; la primera enlaza la guía de arquitectura existente para conservar sus enlaces |

**Resultado**: el control de cobertura ≥80 % se satisface con la medición combinada reproducible de la suite y el binario de contrato. El perfil queda disponible en la ruta que imprime `scripts/coverage.sh`.

**Revisión tras la fase 1**: el diseño añadió 3 puertos, 3 adaptadores secundarios y 1 paquete primario (`console`), sin accesos cruzados entre capas ni dependencias nuevas. Al cierre, la cobertura combinada alcanzó 80,8 %.

## Desviaciones y decisiones que precisan la spec

1. **Salida de `session-start`**: sin avisos no cambia (texto plano). Con aviso, en Claude pasa a JSON con `systemMessage` más `additionalContext` (R3). Así no se tocan las pruebas de contrato actuales.
2. **Modo no interactivo de `uninstall`**: sin `--keep-memory` ni `--export`, la memoria se **borra**, pero solo con `--yes` explícito (FR-018). La recomendación de exportar se aplica en modo interactivo.
3. **`.env.example`**: se añade al alcance para cumplir §9 con la variable nueva (FR-031).
4. **`export` a 0600**: defecto latente que esta feature activa al reutilizar la exportación; se corrige aquí (R12).
5. **Alcance global de agentes**: solo Claude Code, Codex y OpenCode tienen integración global; se valida toda la selección antes de escribir y no se crean archivos de agente en el proyecto actual (FR-022a).

## Project Structure

### Documentation (this feature)

```text
specs/034-console-lifecycle/
├── plan.md              # Este archivo
├── research.md          # Fase 0: R1–R14
├── data-model.md        # Fase 1: entidades, archivos y puertos
├── quickstart.md        # Fase 1: validación de extremo a extremo
├── contracts/
│   ├── cli.md           # install, update, update-check, uninstall, doctor, export
│   └── hook-session-start.md
├── checklists/requirements.md
└── tasks.md             # Fase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
domain/
├── version.go                     # NUEVO: Version, ParseVersion, Newer, UpdateCheckTTL
└── lifecycle.go                   # NUEVO: UninstallPlan/Item, reglas de copia retirable, directorios omitidos del escaneo

application/ports/
├── release.go                     # NUEVO: ReleasePort
├── update_check_repository.go     # NUEVO
└── project_registry_repository.go # NUEVO

application/usecases/
└── lifecycle_*.go                 # NUEVO: planificar/ejecutar desinstalación, decidir el aviso (reutiliza ExportProject de portability.go)

adapters/secondary/
├── release/github.go              # NUEVO: API de releases con ETag y timeout, checksums.txt
└── persistence/
    ├── update_check.go            # NUEVO: caché atómica 0600
    ├── registry.go                # NUEVO: projects/<key>/root y escaneo acotado
    └── db.go / globalstore.go     # MOD: OpenByKey para exportar sin ruta conocida

adapters/primary/
├── console/                       # NUEVO: multiselección, selección, confirmación, pasos y resumen; texto plano
└── cli/
    ├── binref.go                  # MOD: resolución única del global (R1)
    ├── local_copy.go              # NUEVO: identificar y retirar la copia local (R2)
    ├── cmd_install.go             # MOD: consola, --yes/--agents/--scope, sin copia, registro, selección
    ├── cmd_update.go              # MOD: destino global, checksum, consola; HTTP → ReleasePort
    ├── cmd_update_check.go        # NUEVO: subcomando fire-and-forget
    ├── cmd_uninstall.go           # MOD: alcances, inventario, exportación, rutas globales, orden, resumen
    ├── cmd_export.go              # MOD: archivo con 0600 (R12)
    ├── cmd_doctor.go              # MOD: sección "Versión y binario"
    ├── cmd_hook.go                # MOD: session-start → retirada, lanzamiento de la consulta, avisos
    └── hook_dialect.go            # MOD: renderSessionStart (R3)

adapters/primary/setup/            # MOD: instalación por agente seleccionado, texto del protocolo `mem …`
infrastructure/                    # MOD: wiring de los adaptadores nuevos
.specify/extensions/gomemory-context/scripts/{bash,powershell}/  # MOD: preferir `mem` del PATH
scripts/install.sh, scripts/install.ps1                           # MOD: ofrecer la instalación guiada con TTY
.env.example                                                     # NUEVO
docs/architecture.md, docs/MANUAL.md, README.md, CHANGELOG.md    # MOD: documentación

tests/contract/
├── lifecycle_install_test.go      # NUEVO: sin copia, retirada, selección, no interactivo
├── lifecycle_uninstall_test.go    # NUEVO: sistema sin rastros, dry-run, exportación, escaneo
└── hook_session_start_notice_test.go  # NUEVO: invariantes del contrato del hook
```

**Structure Decision**: se mantiene la estructura hexagonal existente (`domain/`, `application/`, `adapters/`, `infrastructure/`). El único paquete nuevo de primer nivel dentro de adaptadores es `adapters/primary/console/`, reutilizable por los tres comandos del ciclo de vida.

## Orden de entrega (historias independientes)

1. **P1 — Binario único** (R1, R2, R6 sin checksum; FR-001…006 y 004a). MVP publicable por sí solo.
2. **P2 — Desinstalación sin rastros** (R10, R11, R12; FR-007…019 y 009a). Corrige el defecto de privacidad.
3. **P4 — Aviso de versión y checksum** (R3, R4, R5; FR-028…034). Se adelanta a P3 porque el checksum es seguridad y porque no depende de la consola.
4. **P3 — Consola guiada** (R7, R8, R9, R14; FR-020…027). Pone la capa interactiva sobre flujos que ya funcionan sin ella.

Cada historia termina con pruebas y los escenarios aplicables del quickstart. La validación real de solo lectura y la publicación quedan como controles de cierre de la feature.

## Complexity Tracking

No hay violaciones de la constitución que justificar.
