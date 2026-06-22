# Quickstart: Validar Mantenimiento de Memoria

Guía de validación manual end-to-end de las 4 capacidades (purgar, compactar, GC, desinstalar).
No reemplaza los tests automatizados de `tasks.md` — sirve para comprobar el comportamiento
real tras implementar, igual que pide `CLAUDE.md`: "verificar contra el contenedor/binario en
ejecución, no solo unit tests".

## Prerrequisitos

- Go 1.25 instalado
- Repo compilable: `go build -o mem ./infrastructure`
- Un directorio temporal de pruebas (no usar el `.memory/` real del repo)

## 1. Preparar un escenario con datos de prueba

```bash
mkdir -p /tmp/mem-maintenance-test && cd /tmp/mem-maintenance-test
/ruta/al/repo/mem init

# Memorias "recientes" (tipo variado)
./mem save -y decision "decisión reciente A"
./mem save -y bugfix "bug reciente B"

# Para simular memorias "viejas" sin esperar 90 días reales, insertar directo en SQLite
# de prueba con created_at retrocedido (solo para esta validación manual):
sqlite3 .memory/mem.db "UPDATE memories SET created_at = datetime('now','-120 days') WHERE content LIKE '%vieja%'"
./mem save -y learning "memoria vieja C"
sqlite3 .memory/mem.db "UPDATE memories SET created_at = datetime('now','-120 days') WHERE content = 'memoria vieja C'"

./mem list   # debe mostrar las 3
```

**Esperado**: `mem list` muestra 3 memorias; referencia: [data-model.md](./data-model.md#entidades-existentes-reutilizadas-sin-cambios).

## 2. Validar `mem purge` (US1)

```bash
./mem purge --project mem-maintenance-test --type bugfix
# Debe pedir confirmación. Responder "no" primero:
#   -> verificar que ./mem list sigue mostrando las 3 memorias (nada se borró)
./mem purge --project mem-maintenance-test --type bugfix --yes
./mem list   # debe mostrar 2 (la de tipo bugfix desapareció, las demás intactas)
```

**Esperado**: coincide con Acceptance Scenarios US1.1–US1.4 en [spec.md](./spec.md). Contrato
completo de flags en [contracts/cli-tui-contracts.md](./contracts/cli-tui-contracts.md#mem-purge).

## 3. Validar relaciones huérfanas (FR-004)

```bash
# Guardar dos memorias y relacionarlas, luego purgar una de las dos
./mem save -y pattern "memoria D" && ./mem save -y pattern "memoria E"
./mem compare -r related -c 0.9 -m "prueba" <id_D> <id_E>
./mem compare list   # debe mostrar 1 relación
./mem purge --project mem-maintenance-test --type pattern --yes
./mem compare list   # debe mostrar 0 relaciones — sin filas huérfanas
```

**Esperado**: ninguna fila de `memory_relations` referencia un `id` inexistente en `memories`.

## 4. Validar `mem compact` (US2)

```bash
du -h .memory/mem.db   # tamaño ANTES
./mem compact          # debe imprimir tamaño antes/después y espacio liberado
du -h .memory/mem.db   # tamaño DESPUÉS — debe ser menor o igual
./mem compact           # correr de nuevo sobre una BD ya compacta
# debe informar que no había espacio significativo que liberar, sin error
```

**Esperado**: coincide con Acceptance Scenarios US2.1–US2.3. SC-002 verificado comparando los
dos `du -h`.

## 5. Validar `mem gc` (US3)

```bash
./mem save -y learning "memoria reciente F"
sqlite3 .memory/mem.db "UPDATE memories SET created_at = datetime('now','-120 days') WHERE content = 'memoria vieja C'"
./mem gc --project mem-maintenance-test --older-than-days 90 --yes
./mem list   # "memoria vieja C" debe haber desaparecido; "memoria reciente F" debe permanecer
```

**Esperado**: coincide con Acceptance Scenarios US3.1–US3.3. SC-004 verificado: cero memorias
recientes eliminadas.

## 6. Validar `mem uninstall` (US4)

```bash
mkdir -p /tmp/mem-uninstall-test && cd /tmp/mem-uninstall-test
/ruta/al/repo/mem install .
ls -la   # debe existir: mem, .memory/, AGENTS.md o CLAUDE.md, .mcp.json, .claude/

./mem uninstall . --yes
ls -la   # mem, .memory/, .claude/plugins/gomemory, entradas MCP y bloque de AGENTS.md/CLAUDE.md
         # deben haber desaparecido
```

**Esperado**: coincide con Acceptance Scenarios US4.1–US4.3 y SC-006. Detalle completo de qué
se elimina en [contracts/cli-tui-contracts.md](./contracts/cli-tui-contracts.md#mem-uninstall-dir).

Para validar la cancelación (Acceptance Scenario US4.2 — sin `--yes`, nada se borra):

```bash
/ruta/al/repo/mem install .
./mem uninstall .   # sin --yes: pide escribir "si"; responde "no" o solo enter
ls -la               # mem, .memory/ y el resto deben seguir intactos
```

**Nota**: `mem uninstall` no modifica `~/.codex/config.toml` automáticamente — a diferencia de
`.mcp.json`, `.opencode.json`, etc. (que son por proyecto), ese archivo es global y compartido
entre todos los proyectos instalados con el agente Codex; editarlo a ciegas arriesga corromper
TOML de otros proyectos. Si instalaste el agente Codex, remueve la tabla
`[mcp_servers."gomemory_*"]` correspondiente a mano.

## 7. Validar acciones en la TUI (FR-010)

```bash
cd /tmp/mem-maintenance-test
./mem tui
```

- El header debe mostrar conteo de memorias **y** tamaño en disco (FR-008/SC-003).
- Presionar `m` debe abrir la pantalla de mantenimiento con las 3 opciones (Purgar/Compactar/GC).
- Confirmar que Purgar y GC exigen escribir el nombre del proyecto antes de ejecutar; Compactar
  se ejecuta directo y muestra el resultado en `statusMsg`.

**Esperado**: comportamiento descrito en [contracts/cli-tui-contracts.md](./contracts/cli-tui-contracts.md#acciones-nuevas-en-la-tui-adaptersprimarytuituigo).
