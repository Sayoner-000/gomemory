# Contrato de CLI: `install`, `update`, `update-check`, `uninstall`, `doctor`, `export`

Convenciones comunes:
- **Modo interactivo**: stdin **y** stdout son TTY, y no se pasó `--yes`.
- **Modo no interactivo**: cualquier otro caso. Nunca lee stdin (FR-022, SC-005).
- **Texto plano**: `NO_COLOR` definida, `TERM=dumb` o menos de 60 columnas. Muestra la misma información sin estilos (FR-027).
- **Resumen**: toda operación termina con una línea por paso, `✓ <paso>` o `⚠ <paso>: <motivo> → <comando manual>` (FR-024).

## `mem install [dir] [--yes|-y] [--agents a,b] [--scope project|global]`

| Flag | Efecto |
|---|---|
| `--yes`, `-y` | Acepta los valores recomendados y la selección guardada, sin preguntar |
| `--agents` | Lista explícita de nombres de `domain.KnownAgents`. Un nombre desconocido produce un error de uso (código 2) antes de escribir nada |
| `--scope` | `project` (por defecto) o `global` |

**Interactivo** (FR-020), en este orden:
1. `¿Qué agentes quieres conectar?`: selección múltiple, con los detectados o guardados ya marcados.
2. Solo si no hay binario global: `¿Dónde instalar el binario?` · `Global en ~/.local/bin (Recomendado)` / `Copia en este proyecto`.
3. `¿Alcance de la configuración?` · `Este proyecto (Recomendado)` / `Global para mi usuario`.
4. Resumen de lo que se escribirá y `¿Continuar? [S/n]`.

**Efectos**:
- no copia el binario si hay global (FR-001);
- retira la copia local si procede (FR-003, FR-004a);
- solo configura los agentes elegidos (FR-021);
- guarda `agents` y `agent_scope` (FR-023);
- registra la ruta del proyecto (FR-013);
- mantiene el resto de pasos actuales (semillas, `.gitignore`, envoltorios, `HardenGlobalStore`).

**Códigos de salida**: 0 éxito (aunque haya ⚠), 1 fallo fatal (destino inexistente) y 2 uso inválido. Una cancelación interactiva devuelve 0 sin cambios.

## `mem update [--version vX.Y.Z] [--check] [--yes|-y]`

1. Resuelve el destino: el global, si existe y no es el ejecutable en curso; si no, el ejecutable en curso (FR-006).
2. Si el destino no admite escritura: `⚠` con `sudo mem update` o el comando del instalador, sin descargar nada. Código 1.
3. Muestra `Actual vX → Disponible vY`. En modo interactivo pide confirmación (FR-025).
4. Pasos con progreso: descarga · **checksum** (FR-032) · sustitución · refresco del proyecto (`install` sin TTY, con la selección guardada) · retirada de la copia local.
5. Un checksum que no coincide o que falta: `✗ checksum` y código 1. **El binario instalado no se toca.**

`--check`: solo muestra `Actual → Disponible` y renueva la caché de versiones. Código 0.

## `mem update-check` (interno, lo lanza `session-start`)

- Sin salida por stdout. Código 0 siempre (fire-and-forget).
- Con `GOMEMORY_NO_UPDATE_CHECK=1` o `update_check_disabled`: termina sin hacer ninguna consulta de red (FR-031, SC-008).
- Una consulta con tiempo límite de 5 s y ETag. Escribe `update-check.json` de forma atómica (FR-029).

## `mem uninstall [dir] [--all] [--yes] [--keep-memory] [--export <dir>] [--scan <dir>] [--no-scan] [--dry-run]`

| Flag | Efecto |
|---|---|
| `--all` | Alcance de sistema. Sin él, alcance de proyecto |
| `--yes` | Confirma sin preguntar. **Obligatorio** en modo no interactivo |
| `--keep-memory` | Memoria = conservar |
| `--export <dir>` | Memoria = exportar, al destino indicado |
| *(ninguno de los dos)* | Interactivo: pregunta (recomendada: exportar). No interactivo: **borrar** |
| `--scan <dir>` | Raíz del escaneo (solo con `--all`; por defecto `~`) |
| `--no-scan` | Solo proyectos registrados |
| `--dry-run` | Imprime el inventario y no modifica nada (SC-004) |

`--keep-memory` y `--export` son incompatibles (código 2).

**Interactivo** (FR-015): alcance → inventario agrupado con tamaños → memoria → confirmación (alcance de sistema: escribir `gomemory`; proyecto: `[s/N]`, con no por defecto) → pasos → resumen.

**No interactivo sin `--yes`** (FR-018): imprime el inventario, `✗ se requiere --yes para desinstalar sin terminal` y código 1, sin borrar nada.

**Garantías**:
- una exportación fallida impide borrar la memoria de ese proyecto (FR-009);
- los archivos compartidos solo pierden las entradas de gomemory (FR-012);
- ningún paso fallido detiene los siguientes (FR-015);
- el texto de confirmación enumera exactamente las categorías que se borrarán (FR-019).

**Códigos de salida**: 0 si todo es ✓, 3 si terminó con algún ⚠, 1 sin `--yes` en modo no interactivo o con una confirmación inválida, y 2 con un uso inválido.

## `mem doctor` (ampliación)

Sección nueva `Versión y binario`:
```
Versión y binario:
  binario global: /Users/x/.local/bin/mem (2.26.4)
  copias locales: ninguna | /ruta/proyecto/mem (2.8.0) — se retirará al abrir el proyecto
  aviso de versión: activo | desactivado (GOMEMORY_NO_UPDATE_CHECK | ajuste)
  última consulta: 2026-09-26T10:52:37-05:00 → v2.26.4 (al día) | error: <last_error>
```
En `--json`: objeto `binary` (la clave `version` ya existe y es la versión de gomemory) con `global_path`, `global_version`, `local_copies[]`, `update_check{enabled, checked_at, latest, error}` (FR-033).

## `mem export [--out <archivo>]` (corrección)

Sin cambios de interfaz. El archivo se crea con **0600** (R12).
