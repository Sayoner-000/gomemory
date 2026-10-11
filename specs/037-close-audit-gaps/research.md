# Investigación: cierre de auditoría 037

Fecha: 2026-10-11. Base: revisión `30528515597292ecfcd42c7c08283cae0fbbbd12`, informe [defensive-qa](../../docs/audits/2026-10-11-defensive-qa.md) y [spec](spec.md). Investigación de fuente mediante `codebase-memory-mcp` en `Users-josegomezj-home-rcw-gomemory`. Memoria de planificación y constitución cargadas antes del diseño.

## R1 — Política de texto y superficies

- **Decisión**: recorrido de etiquetas ASCII con profundidad en `domain.RedactPrivate`; reutilizarlo en `IsPrivateContent`. Componer redacción privada y de credenciales en escrituras y lecturas de campos afectados; las salidas históricas quedan protegidas antes de saneamiento.
- **Motivo**: `EndSession`, `UpdateSessionSummary`, escrituras de relaciones y `RecordChannelError` guardan crudo; lectores equivalentes y `ExportProject` lo devuelven. `Builder.Build` recorta resúmenes antes de emitirlos: proteger antes del recorte evita cortar una etiqueta y perder su significado.
- **Alternativas**: proteger solo InsertMemory no cubre tablas auxiliares; proteger solo escrituras deja historia expuesta; una regex sin profundidad no satisface anidamiento.
- **Evidencia**: `domain/redact.go`, `domain/compression.go`; `session.go`, `relation.go`, `channel_activity.go`, `cmd_session.go`; `application/usecases/portability.go` y `build_context.go`.

## R2 — Saneamiento y contrato de mantenimiento

- **Decisión**: comando nuevo `mem sanitize`, preview por defecto, `--apply` explícito y `--json`; alcance solo proyecto actual. Ports tipados: UnitOfWork transaccional, repositorio de candidatos y puerto de artefactos/informe. Sin nuevo esquema. Crear manifest `pending_cleanup` antes de transacción; reintentar toda transformación idempotente si hubo crash.
- **Motivo**: DB y archivos no comparten commit; no se debe declarar limpieza terminada solo por actualizar filas. Aplicación reevalúa registros actuales, preserva ids/fechas y modifica únicamente campos permitidos. El callback de UnitOfWork permite control en el caso de uso sin exponer SQL o driver.
- **Alternativas**: migración destructiva al abrir, respaldo crudo automático y reemplazo completo de DB con conexiones activas se descartan. Lock global de todos comandos no resulta necesario para la operación lógica serializada por SQLite.
- **Evidencia**: `openDBFile` configura busy timeout y transacción inmediata; `CreateSnapshot` genera JSON en `DataHome/backups/<ProjectKey>`; `Builder.WriteFile` genera `.memory/context.md`. Tras commit, reescribir copias administradas con temporales depurados, regenerar contexto y eliminar originales privados reconocidos; no reescribir originales públicos bajo una referencia que promete recuperación exacta.

## R3 — Residuos y concurrencia

- **Decisión**: activar secure_delete en conexión de mantenimiento antes de cambios; después del commit y cierre de statements, compactar y realizar checkpoint TRUNCATE verificando busy. Residuos inaccesibles o bloqueo mantienen pending_cleanup y código no cero. No eliminar WAL/SHM por operaciones de archivos.
- **Motivo**: una sentencia de checkpoint sin error puede informar bloqueo. La limpieza física requiere ventana offline con clientes antiguos detenidos/reiniciados; no se puede garantizar protección frente a un binario antiguo que vuelve a escribir datos crudos.
- **Alternativas**: asumir que UPDATE elimina residuos o borrar WAL directamente no prueba cierre seguro. No se promete borrado forense de SSD/copias externas.
- **Fuentes**: [SQLite PRAGMA](https://www.sqlite.org/pragma.html#pragma_wal_checkpoint), [WAL](https://www.sqlite.org/wal.html), [VACUUM](https://www.sqlite.org/lang_vacuum.html). Verificar residuos e índices con marcadores sintéticos en archivos controlados y registrar limitaciones.

## R4 — Builder reutilizado

- **Decisión**: snapshot local de la configuración inmutable y contador a cero por llamada; conservar firma de `Build` y ports. Probar recorder/contador reales bajo concurrencia.
- **Motivo**: `Builder.discardedChars` se modifica en `acota` y `fits`; MCP reutiliza Builder. Reiniciar el receptor compartido todavía tendría carreras; una copia local separa estado efímero.
- **Alternativas**: mutex global serializa todas las construcciones; un reset al inicio no elimina la carrera. No introducir recarga mutable de configuración en esta feature.

## R5 — Reemplazo e integridad

- **Decisión**: temporal al lado del ejecutable Unix, preparación completa, permisos, sync/close y rename final; scripts resuelven tag estable una vez y verifican una única entrada SHA-256 exacta para el asset antes de extraer.
- **Motivo**: `replaceSelf` retira destino antes de copiar; ambos scripts extraen sin checksum. `.goreleaser.yml` publica `checksums.txt` y nombres de assets existentes.
- **Alternativas**: rename a `.old` primero deja ausencia; copiar encima expone parcial. Checksum del mismo origen verifica integridad publicada y no se presenta como firma independiente.
- **Fuente**: [Go os.Rename](https://pkg.go.dev/os#Rename); el contrato nuevo de disponibilidad se limita a Unix. No se extiende al reemplazo Windows ni al fallo eléctrico.

## R6 — Mutación de OpenCode v2

- **Decisión**: adaptar el texto reducido a `event.result` y cubrir content string, content parts y output string; conservar las demás partes. No mutar si faltan texto, ganancia, ownership o resultado completado.
- **Motivo**: el adapter actual pasa un objeto temporal al hook v1 y no copia su cambio al resultado. El contrato oficial presenta reasignación de `event.result` tras ejecución.
- **Alternativas**: medir la salida temporal no demuestra ahorro al consumidor; depender de comportamiento v1 no prueba v2.
- **Fuente**: [OpenCode v2 Plugins](https://dev.opencode.ai/v2/docs/build/plugins/). Validación host queda exigida por FR-026. OpenCode observado localmente: 1.18.35; historial de feature 032 verificó 2.0.16, pero no demuestra este cambio.

## R7 — Cierre <=5 segundos

- **Decisión**: lifecycle v2 propio con closing, subscription promise, conjunto de operaciones y cleanup promise compartida. AbortSignal propagada a `Exec` y requests v2; cancelar hijos, esperar su cierre y terminar registros/suscripción. Controlar deadline global de 5 s; timeout devuelve fallo depurado, nunca éxito con actividad pendiente. Cierre de sesión propio, si corresponde, usa presupuesto restante y una ejecución independiente de la señal ya abortada.
- **Motivo**: `execFileExec` usa 30 s sin señal; abortar solo stream no cancela un callback en curso. Rollback de setup no cierra sesiones ajenas.
- **Alternativas**: `Promise.race` sin cancelación deja trabajo vivo; invocar todos los comandos v1 con una señal ya abortada impediría cierre legítimo.
- **Fuentes**: [Node child_process](https://nodejs.org/api/child_process.html#child_processexecfilefile-args-options-callback), [OpenCode v2 lifecycle/events](https://dev.opencode.ai/v2/docs/build/plugins/). La implementación debe esperar `close` del hijo, no confundir rechazo por abort con proceso terminado.

## R8 — Wrap, fixtures y CI

- **Decisión**: `wrap` distingue error de arranque de salida del hijo, usa código 1 y diagnóstico estable depurado sin diálogo. Fixtures exigen conversación/proyecto y simulan error mediante prefijos semánticos; finally libera recursos. Nuevo workflow test en push/PR ejecuta Go, carreras dirigidas, cobertura combinada, plugin, installer y Windows.
- **Motivo**: `CmdWrap` solo maneja ExitError; las dos pruebas desactualizadas comparan toda la llamada. Solo existe `.github/workflows/release.yml`, activado por tags, sin suite de pruebas de cambios.
- **Alternativas**: ignorar fallos o reducir asserts ocultaría defectos. No cambiar release ni añadir lockfile.

## R9 — Runtimes y estado de investigación

- **Decisión**: mínimos según configuración existente: Go 1.27.0/toolchain 1.27.1, Node 22.20.0, Python 3.12 para sintaxis documentada. CI Python 3.13 y Node con tipos compatibles; PowerShell se valida en Windows. Runners fallan si faltan prerequisitos o ejecutan cero casos.
- **Motivo**: auditoría ejecutó Python 3.12 y encontró incompatibilidad en 3.9; instalador declara Node. `setup-plan.sh` devolvió BRANCH vacío, pero `git branch --show-current` confirma main; la feature se localiza mediante feature.json.
- **Alternativas**: inferir soporte por pruebas de sintaxis o por la versión instalada no valida plataformas ausentes.

Decisiones de diseño resueltas; ninguna aclaración de alcance pendiente. Permanece pendiente la evidencia de implementación, host y plataforma. Octopus autorizó un investigador de privacidad (DELEGATE); la investigación OpenCode se ejecutó aquí (INLINE por presupuesto). El diseño no afirma tests verdes ni saneamiento aplicado.
