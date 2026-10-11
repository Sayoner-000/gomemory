# Implementation Plan: Cierre de brechas de auditoría defensiva

**Branch**: `main` | **Date**: 2026-10-11 | **Spec**: [spec.md](spec.md)

**Input**: `specs/037-close-audit-gaps/spec.md`

## Summary

Cerrar H-01–H-09 mediante protección uniforme del texto, saneamiento histórico explícito, aislamiento del estado de construcción, reemplazo Unix preparado antes de publicar, comprobación de integridad en scripts, adaptación efectiva y cancelable de OpenCode v2 y clasificación correcta de fallos de `wrap`. Mantener los contratos actuales de consola, herramientas del agente y compresión.

La fase de diseño entrega [research.md](research.md), [data-model.md](data-model.md), cuatro contratos y [quickstart.md](quickstart.md). No implementa las correcciones ni ejecuta saneamiento sobre datos reales. `tasks.md` corresponde a `$speckit-tasks`.

## Technical Context

**Language/Version**: Go mínimo 1.27.0 y toolchain 1.27.1 según `go.mod`; TypeScript 7.0.2 del instalador; Node mínimo 22.20.0 según `installer/package.json`; Bash y PowerShell existentes. Python mínimo 3.12 para los scripts de validación que usan su sintaxis; CI usará 3.13, sin migrar scripts ajenos al alcance.

**Primary Dependencies**: `modernc.org/sqlite` 1.58.0, SDK MCP Go 1.7.0, dependencias actuales de consola e instalador. Biblioteca estándar para redacción, archivos, hashes, procesos y cancelación. No agregar dependencias ni otro gestor de paquetes.

**Storage**: SQLite existente, JSON de copias administradas, `.memory/context.md` y almacén de originales comprimidos. Nuevo informe de mantenimiento JSON depurado en `DataHome/maintenance/privacy/<run_id>.json`; sin tabla ni migración de esquema nueva.

**Testing**: `go test`, detector de carreras, contrato sobre binario compilado y MCP stdio; `node --test` del plugin; `npm ci`, typecheck y pruebas del instalador; fixtures de descargas sin red pública; validación real de Windows y OpenCode v2. Los runners nuevos definidos en quickstart son artefactos de implementación futuros.

**Target Platform**: macOS/Linux para reemplazo Unix y scripts Bash; Windows para instalador PowerShell y preservación de contratos. OpenCode v1 y v2 mantienen export dual. El OpenCode local observado es 1.18.35; no constituye evidencia v2.

**Project Type**: CLI/TUI con servidor MCP stdio y plugins de agentes; arquitectura hexagonal existente.

**Performance Goals**: Tres solicitudes consecutivas y ocho simultáneas generan métricas independientes; cierre normal del plugin <=5 s con evento en curso. Redacción lineal respecto al texto, sin acumulación entre solicitudes. No se introduce un objetivo artificial de latencia para mantenimiento offline.

**Constraints**: Sin contenido protegido en salidas ni nuevas escrituras; saneamiento solo mediante acción explícita, acotado al proyecto resuelto; sin copias crudas nuevas. Mantener identidad, fechas, relaciones, salida pública y política de compresión. No prometer borrado físico de SSD ni durabilidad nueva frente a pérdida de energía.

**Scale/Scope**: Siete historias, 27 requisitos y nueve hallazgos. Procesar todos los registros históricos del proyecto, sin usar los límites de recencia de las consultas normales. Otros proyectos permanecen sin cambios lógicos.

## Constitution Check

Constitución vigente recuperada mediante `mem constitution`, versión 2.0.0, y archivo `.specify/memory/constitution.md`. El catálogo guía este cambio a un proyecto existente; no fuerza una renovación de todas sus dependencias.

| Puerta | Antes de investigación | Después del diseño |
| --- | --- | --- |
| Capas y composición | Mantener domain/application/ports/adapters/infrastructure | Cumple: redacción pura en dominio; saneamiento por caso de uso y puertos; SQL y archivos en adaptadores; wiring en `container.go` |
| Transacciones y consultas | Sin SQL en CLI ni application, valores parametrizados | Cumple: caso de uso controla `WithTransaction`, callback recibe interfaz sin driver; adapter implementa BEGIN/commit/rollback y timeouts |
| Privacidad y mínimo privilegio | Ningún dato real necesario para reproducciones | Cumple: preview sin texto, apply explícito, informe depurado, destino acotado, sin respaldos crudos |
| Esquema y migraciones | No modificar migraciones existentes | Cumple: el diseño no requiere esquema nuevo ni saneamiento al arrancar |
| Dependencias y runtimes | Conservar módulos y lockfile actuales | Cumple: sin paquetes nuevos de producción; mínimos declarados antes de validar |
| Pruebas y cobertura | No desactivar tests; >=80 % global y no reducir línea base | Cumple como diseño: nueva CI ejecuta `scripts/coverage.sh`; la medición real es puerta de implementación, no se presume aprobada |
| Contratos y compatibilidad | Mantener formatos públicos y export dual | Cumple: cambio solo en texto protegido y resultados defectuosos; JSON depurado para comando nuevo |
| Documentación | Español latino; tareas verificables | Cumple: artefactos y árbol con resultados verificables |

No se solicita una excepción constitucional. La memoria 305 documentó una brecha de cobertura con una medición antigua distinta; no se reutiliza como resultado actual ni se reduce el umbral. Si el perfil combinado actual no alcanza 80 %, la puerta de entrega falla y deberá cerrarse con cobertura real antes de declarar implementación lista.

## Project Structure

### Documentation (this feature)

```text
specs/037-close-audit-gaps/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── checklists/requirements.md
└── contracts/
    ├── privacy-maintenance.md
    ├── context-wrap.md
    ├── installation.md
    └── opencode-lifecycle.md
```

`tasks.md` se generará en la fase siguiente. Los nombres nuevos del árbol de código indican archivos previstos, no archivos implementados.

### Source Code (repository root)

```text
domain/
  redact.go, compression.go, privacy_maintenance.go (nuevo)
application/
  ports/privacy_maintenance.go (nuevo)
  usecases/build_context.go, portability.go, backup.go
  usecases/sanitize_project.go (nuevo)
adapters/
  primary/cli/cmd_session.go, cmd_update.go, cmd_wrap.go, dispatcher.go
  primary/cli/cmd_sanitize.go (nuevo)
  secondary/persistence/session.go, relation.go, channel_activity.go
  secondary/persistence/memory.go, compression_originals.go
  secondary/persistence/privacy_maintenance.go (nuevo)
infrastructure/
  container.go
  plugin/opencode/gomemory.ts, gomemory.test.mjs
scripts/
  install.sh, install.ps1, coverage.sh
  audit_037_smoke.py (nuevo)
  audit_037_windows_smoke.ps1 (nuevo)
  audit_037_opencode_smoke.mjs (nuevo)
tests/
  contract/, integration/ y pruebas unitarias junto a los paquetes
.github/workflows/
  release.yml (existente)
  test.yml (nuevo)
installer/
  package.json y suite existente, sin segundo lockfile
```

**Structure Decision**: Ampliar las capas actuales. `PrivacyMaintenanceUnitOfWork` devuelve un repositorio transaccional tipado y un puerto separado controla artefactos e informe; no exponer `sql.DB`, cursores ni drivers. Reutilizar el composition root existente y la presentación compartida de CLI.

## Diseño por área

1. **Privacidad (FR-001–005)**: un recorrido de etiquetas con profundidad sustituye la regex de bloques privados. Aplicar composición `RedactSecrets(RedactPrivate(text))` en escrituras afectadas y materialización de lecturas históricas, antes de recortar o exportar. Depurar también la respuesta inmediata de `session end`. El almacén de originales rechaza contenido protegido al escribir y al recuperar, sin cambiar recuperación exacta de originales públicos.
2. **Historia (FR-006–008)**: `mem sanitize` presenta diagnóstico; `--apply` crea un informe pendiente antes de tocar datos, vuelve a evaluar filas en transacción inmediata y conserva metadatos. Limpieza posterior idempotente de JSON administrados, contexto generado y originales privados; compactación y checkpoint verificado fuera de la transacción. Ante busy o fallo, conservar `pending_cleanup`, salir no cero y permitir reintento. El cierre físico exige una ventana de mantenimiento sin clientes antiguos.
3. **Contexto (FR-009–010)**: `Build` copia configuración inmutable a una instancia local y reinicia el contador de descartes en esa copia antes de construir. No resetear el receptor compartido. Verificar además que puertos de contador y recorder usados por MCP soportan concurrencia.
4. **Actualización e instalación (FR-011–015)**: en Unix preparar archivo temporal en directorio del ejecutable, copiar, establecer permisos, sincronizar y cerrar antes de rename. Conservar comportamiento Windows de update. Los scripts resuelven un release estable concreto, verifican SHA-256 y validan ejecutable antes de publicar. Extraer únicamente el miembro esperado y rechazar enlaces o rutas inseguras.
5. **OpenCode (FR-016–021)**: extraer texto por representación, pasar el resultado mutable al hook v1 y publicar el resultado reducido en `event.result`. Preservar partes no textuales y metadatos; la ausencia de texto es no-op. Lifecycle propio con estado closing, promesa de suscripción retenida, operaciones rastreadas y señales de cancelación para subprocess/requests; cleanup idempotente y rollback de setup. No usar un timeout que solo abandone promesas como prueba de liberación.
6. **Wrap (FR-022–023)**: error de arranque distinto de `exec.ExitError` produce código 1 y `WRAP_START_FAILED` depurado en stderr antes del diálogo. El hijo iniciado conserva código/salida y flujo actual.
7. **Regresiones (FR-024–027)**: comparar prefijo semántico de argumentos y exigir conversation/project; fallos inyectados efectivos y cleanup en finally. Agregar CI de pruebas porque hoy solo existe workflow de release. Mantener comprobaciones externas pendientes hasta obtener evidencia real.

## Árbol de trabajo verificable

Objetivo: H-01–H-09 reproducidos y cerrados con evidencia, sin modificar comportamiento público ajeno al alcance. `✓` significa hoja atómica propuesta, no tarea realizada; `∥` indica independencia lógica, no autorización automática para delegar. Cada hoja incluye su regresión en el mismo resultado.

```text
🎯 Cerrar nueve hallazgos con evidencia por requisito
├─ [1] Proteger texto nuevo e histórico en las interfaces
│  ├─ [1.1] ✓ Corregir parser privado → casos anidados/incompletos verdes
│  ├─ [1.2] ✓ Depurar escrituras auxiliares → registros protegidos (dep: 1.1)
│  ├─ [1.3] ✓ Depurar lectores/exportación → salidas históricas protegidas (dep: 1.1)
│  └─ [1.4] ✓ Rechazar originales privados → recuperación privada denegada (dep: 1.1)
├─ [2] Sanear historia mediante mantenimiento explícito (dep: 1)
│  ├─ [2.1] ✓ Definir puertos de mantenimiento → contratos tipados compilables
│  ├─ [2.2] ✓ Implementar aplicación transaccional → rollback/ids/fechas probados (dep: 2.1)
│  ├─ [2.3] ✓ Registrar estado durable depurado → reintento posterior a crash probado (dep: 2.1)
│  ├─ [2.4] ✓ Limpiar artefactos y residuos → checkpoint/copies verificados (dep: 2.2, 2.3)
│  └─ [2.5] ✓ Exponer mem sanitize → preview/apply/JSON probados (dep: 2.4)
├─ [3] Aislar construcción de contexto (∥ con 2)
│  ├─ [3.1] ✓ Localizar contador por Build → tres emisiones iguales
│  └─ [3.2] ✓ Verificar servicio MCP compartido → ocho llamadas sin carreras (dep: 3.1)
├─ [4] Publicar artefactos verificados (∥ con 2, 3)
│  ├─ [4.1] ✓ Preparar reemplazo Unix → invocaciones ven versión completa
│  ├─ [4.2] ✓ Verificar descarga Bash → matriz de integridad verde
│  └─ [4.3] ✓ Verificar descarga PowerShell → matriz real Windows verde
├─ [5] Corregir adaptación OpenCode (∥ con 2, 3, 4)
│  ├─ [5.1] ✓ Publicar texto reducido → formas mixtas conservadas
│  ├─ [5.2] ✓ Cancelar lifecycle propio → rollback/cierre <=5s probados
│  └─ [5.3] ✓ Verificar consumidor v2 → evidencia real de reducción/cierre (dep: 5.1, 5.2)
├─ [6] Reportar arranque fallido (∥ con 2–5)
│  └─ [6.1] ✓ Clasificar error de wrap → códigos y ausencia de diálogo probados
└─ [7] Consolidar regresión y evidencia
   ├─ [7.1] ✓ Corregir fixtures de sesiones → ID/proyecto/fallos efectivos (∥)
   ├─ [7.2] ✓ Publicar CI de pruebas → Go/plugin/installer/Windows ejecutados (dep: 1–6, 7.1)
   ├─ [7.3] ✓ Declarar runtimes mínimos → preflight incompatible falla claramente (∥)
   └─ [7.4] ✓ Registrar cierre por hallazgo → 9/9 evidencias y cobertura aprobada (dep: 7.2, 7.3)
```

Autoverificación: 22 hojas, sin decisiones de diseño pendientes ni dependencias circulares. La prioridad de implementación es [1], luego [2]; las ramas independientes pueden avanzar tras asegurar los contratos de privacidad. La evidencia de plataforma/host es una dependencia de ejecución explícita, no un motivo para marcar el hallazgo cerrado sin probarlo.

## Validación y puertas de entrega

| Cobertura | Requisitos | Evidencia mínima |
| --- | --- | --- |
| US1 / H-01–02 | FR-001–005 | CLI/MCP/hook/import/export, lectores históricos, parser y originales públicos/privados |
| US2 / H-01 | FR-006–008 | preview, apply, rollback, reintento, copia inaccesible, busy, segundo apply |
| US3 / H-03 | FR-009–010 | tres construcciones iguales y ocho MCP solapadas bajo detector de carreras |
| US4 / H-04, H-08 | FR-011–015 | reemplazo observable; fixtures de hash/red/permisos en Unix y Windows |
| US5 / H-05–06 | FR-016–021 | contrato simulado más host v2 real; falla cada registro y cierre con operación en curso |
| US6 / H-07 | FR-022–023 | binario real: no existe, sin permisos, cero y no cero |
| US7 / H-09 | FR-024–027 | fixtures efectivos, CI, preflight y registro por hallazgo |

Ejecutar pruebas dirigidas en rojo antes de corregir cada comportamiento. Después ejecutar suite completa, vet, carreras sobre rutas afectadas, suite plugin e instalador, perfil combinado >=80 % y comprobación de impacto con grafo externo sobre el diff de implementación. Este diseño no tiene cambios de código que ameriten afirmar un impacto ejecutable medido. Una prueba seleccionada sin casos ejecutados no es evidencia.

La ausencia de Windows o OpenCode v2 mantiene su evidencia pendiente. El plan resuelve qué comprobar y cómo registrarlo; no afirma que los entornos hayan sido ejercitados en esta fase.
