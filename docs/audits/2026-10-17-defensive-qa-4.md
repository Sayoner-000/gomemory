# Auditoría defensiva multilenguaje — ronda 4

**Fecha:** 2026-10-17
**Revisión auditada:** `a75ca31fac331756569d07ed4121f67ff1c5a6b7`
**Árbol de trabajo:** sucio — 42 rutas modificadas y 18 sin seguimiento.
**Alcance:** superficies de producción y cambios locales de Socratica: CLI/TUI,
harnesses de generación, persistencia SQLite y saneamiento, locks de despacho,
instaladores TypeScript, Bash y PowerShell, actualizador, CI y cadena de suministro.
**Modalidad:** solo lectura; este informe no incluye correcciones ni parches.

## 1. Resumen de salud del código

- **Lenguajes y tecnologías detectadas:** Go 1.27.2 (producción y pruebas),
  TypeScript/Node (instalador), Bash, PowerShell, Python (runners de auditoría),
  SQLite, GitHub Actions y YAML.
- **Nivel de riesgo general:** **Bajo**.
- **Resultado:** **0 hallazgos** (0 críticos, 0 altos, 0 medios, 0 bajos).

La superficie nueva de Socratica mantiene límites defensivos relevantes: los
harnesses se ejecutan con argumentos separados, `context` con timeout y espera
acotada; OpenCode usa un directorio temporal con permisos restringidos y lo borra
al terminar; el flujo de despacho usa un lock por flujo y una reclamación/confirmación
persistida. Los datos de Socratica se protegen al persistirse y el saneamiento cubre
las columnas de texto y JSON de flujo e historial.

La mitigación de CWE-150 revisada en rondas anteriores sigue activa en las rutas
humanas: el contenido persistido o generado se depura antes de pintarse en terminal
o TUI. Las salidas de pipe y `--print` conservan intencionadamente bytes de entrega
en vez de tratarlas como una pantalla interactiva.

Esta es una auditoría dirigida, no una garantía de ausencia de defectos. No se
ejecutaron `govulncheck`, `staticcheck`, `shellcheck`, `semgrep` ni un escáner de
secretos porque no están disponibles en el host. El Python local es 3.9.6, inferior
al mínimo 3.12 declarado por CI; por ello no se usó para invalidar los runners Python
que emplean sintaxis posterior.

### Cobertura y validación

| Comprobación | Resultado |
|---|---|
| Inspección de grafo: bordes CLI/TUI, generadores, persistencia, locks, instaladores y CI | Pasa — rutas y llamadores auditados |
| `git diff --check` | Pasa |
| `go vet ./...` | Pasa |
| `go test ./...` | Pasa |
| `go test -race ./adapters/primary/cli ./adapters/primary/tui ./adapters/secondary/generator ./adapters/secondary/persistence ./application/usecases` | Pasa |
| `npm test -- --runInBand` en `installer/` | Pasa — 24 pruebas |
| `npm audit --omit=dev --audit-level=high` en `installer/` | Pasa — 0 vulnerabilidades |
| `bash -n` sobre scripts versionados | Pasa |
| `python3 -m compileall -q scripts` | No aplicable al intérprete local: Python 3.9.6 no admite la sintaxis PEP 701 de `scripts/acr-smoke.py`; CI declara Python 3.13 |
| `govulncheck`, `staticcheck`, `shellcheck`, `semgrep`, `gitleaks` | No disponibles en el host |

## 2. Hallazgos y fugas detectadas

| ID | Severidad | Estado | Problema |
|---|---|---|---|
| — | — | — | No se confirmaron defectos en el alcance auditado. |

### Verificaciones de cierre de riesgos previos

- **Inyección de secuencias de terminal (CWE-150):** las rutas Socratica usan
  `domain.StripTerminalControls` antes de renderizar contenido no confiable en
  terminal/TUI (`domain/terminal_text.go:14`,
  `adapters/primary/cli/cmd_socratica.go:51`,
  `adapters/primary/console/safe_ui.go:10`,
  `adapters/primary/tui/socratica.go:679`). Las pruebas de CLI y TUI bajo
  `-race` finalizaron correctamente.
- **Ejecución y recursos de harnesses:** la ejecución de Claude/Codex emplea
  `exec.CommandContext`, timeout y `WaitDelay`
  (`adapters/secondary/generator/exec.go:49`); OpenCode crea y elimina un
  workspace 0700 y sólo escribe el perfil fijo 0600
  (`adapters/secondary/generator/opencode.go:90`).
- **Datos privados en Socratica:** los valores de flujo se protegen al escribir
  (`adapters/secondary/persistence/socratica.go:145`) y el mantenimiento cubre
  tanto campos escalares como JSON del flujo y del historial
  (`adapters/secondary/persistence/socratica_sanitize.go:17`).
- **Integridad de instalación:** los instaladores verifican un checksum de asset
  único antes de extraer/publicar y las descargas TypeScript tienen timeout y
  límites de tamaño (`scripts/install.sh:152`, `scripts/install.ps1:112`,
  `installer/src/bootstrap.ts:20`). La suite del instalador valida además los
  límites, checksums, reemplazo atómico y cancelación.
- **CI y suministro:** los workflows fijan las acciones por SHA y restringen
  permisos en validación (`.github/workflows/test.yml:12`); la release limita
  `contents: write` al trabajo que publica (`.github/workflows/release.yml:8`).

## 3. Recomendaciones de resiliencia

No se solicita una corrección. Para mantener el nivel observado, conviene que CI
incorpore, cuando estén disponibles de forma fijada, `govulncheck`, análisis estático
Go, `shellcheck` y escaneo de secretos; y que conserve las pruebas de regresión de
terminal, límites de descarga, persistencia privada y despacho concurrente.
