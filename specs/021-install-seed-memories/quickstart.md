# Quickstart — Validación de la feature 021

**Spec**: [spec.md](./spec.md) · **Contratos**: [contracts/](./contracts/)

Guía de validación de punta a punta. Sigue la regla de campo 2 del proyecto:
**verde en tests no es "funciona"** — los escenarios 2 a 7 corren contra el binario
real, no contra mocks.

---

## Prerrequisitos

```bash
cd /Users/josegomezj/home/rcw/go_memory
go build -o /tmp/mem_seed ./infrastructure
```

Todos los escenarios usan directorios desechables fuera del repositorio, para no
ensuciar el proyecto ni disparar la limpieza sobre su propio `CLAUDE.md`.

---

## 1. Suite automatizada

```bash
go build ./... && go vet ./... && go test ./... 2>&1 | tail -25
```

**Esperado**: todo verde, incluidos los tests nuevos de siembra, emisión íntegra,
limpieza, comando de constitución y arranque MCP.

---

## 2. Instalación limpia — cero artefactos (US2, SC-001)

```bash
rm -rf /tmp/probe_limpio && mkdir -p /tmp/probe_limpio
/tmp/mem_seed install /tmp/probe_limpio
ls -a /tmp/probe_limpio
```

**Esperado**: la raíz contiene `mem`, `.gitignore`, `.memory/`, `.claude/`,
`.opencode/`, `.cursor/`.
**No debe existir**: `AGENTS.md`, `CLAUDE.md`, `speckit-constitution-gen.md`,
`.windsurf/`, `.cline/`.

Verificación en un solo comando:

```bash
for f in AGENTS.md CLAUDE.md CLAUDE.txt speckit-constitution-gen.md .windsurf .cline; do
  [ -e "/tmp/probe_limpio/$f" ] && echo "FALLA: $f existe" || echo "ok: sin $f"
done
```

El mensaje final debe nombrar `get_context()` y `/constitution`, y **no** debe
mencionar `AGENTS.md` ni el aviso de v1.9.

---

## 3. Reglas íntegras en el contexto (US1, SC-002)

```bash
cd /tmp/probe_limpio && ./mem context | head -30
```

**Esperado**: `## Reglas de trabajo (memoria fijada)` aparece justo después de
`# Memoria del Proyecto`, con el texto completo del preámbulo.

**Comprobación dura** — el contenido no debe estar recortado:

```bash
cd /tmp/probe_limpio
./mem context | grep -A 200 "Reglas de trabajo (memoria fijada)" | grep -c "Corrección Autónoma de Bugs"
```

**Esperado**: `1`. Ese encabezado vive al final del preámbulo; si aparece, no hubo
truncado a 200 caracteres. Si devuelve `0`, la sección pasó por `acota()`.

Y no debe estar duplicada:

```bash
cd /tmp/probe_limpio && ./mem context | grep -c "Reglas de trabajo del proyecto"
```

**Esperado**: `0` — el título de la semilla no debe reaparecer en
`## Preferencias del Usuario`.

---

## 4. Idempotencia y respeto a la edición (US1, SC-003)

```bash
cd /tmp/probe_limpio
./mem search "Reglas de trabajo"          # anotar el id
./mem save -t "Reglas de trabajo del proyecto" -y preference "TEXTO EDITADO A MANO"
```

> La edición real se hace desde la TUI (`./mem`). Para el guion automatizable,
> editar la fila por su `topic_key` produce el mismo estado de partida.

Reinstalar 5 veces y comprobar que la edición sobrevive:

```bash
for i in 1 2 3 4 5; do /tmp/mem_seed install /tmp/probe_limpio > /dev/null; done
cd /tmp/probe_limpio && ./mem context | grep -c "TEXTO EDITADO A MANO"
```

**Esperado**: `1`. Si devuelve `0`, la siembra hizo `UPDATE` en vez de
comprobar-y-omitir (violación de C2 del contrato de siembra).

---

## 5. Limpieza de una instalación previa (US3, SC-004, SC-008)

Simular un proyecto instalado con la versión anterior:

```bash
rm -rf /tmp/probe_legado && mkdir -p /tmp/probe_legado/.windsurf /tmp/probe_legado/.cline
cd /tmp/probe_legado
printf '# Mi proyecto\n\nInstrucciones que escribí yo.\n' > CLAUDE.md
printf '# Agentes\n\nOtro texto propio.\n' > AGENTS.md
printf 'constitucion vieja\n' > speckit-constitution-gen.md
# .windsurf: solo gomemory -> debe desaparecer entero
printf '{"mcpServers":{"gomemory":{"command":"./mem","args":["mcp"]}}}\n' > .windsurf/mcp_config.json
# .cline: gomemory + un servidor ajeno -> el ajeno debe sobrevivir
printf '{"mcpServers":{"gomemory":{"command":"./mem"},"otro":{"command":"otra-cosa"}}}\n' > .cline/mcp_settings.json

/tmp/mem_seed install /tmp/probe_legado
```

**Esperado**:

```bash
ls /tmp/probe_legado/.memory/backups/agent-files/     # CLAUDE.md y AGENTS.md
cat /tmp/probe_legado/.memory/backups/agent-files/CLAUDE.md   # texto propio íntegro
[ -e /tmp/probe_legado/.windsurf ] && echo FALLA || echo "ok: .windsurf eliminada"
grep -c gomemory /tmp/probe_legado/.cline/mcp_settings.json   # -> 0
grep -c '"otro"' /tmp/probe_legado/.cline/mcp_settings.json   # -> 1
```

La salida de la instalación debe informar la ruta de cada respaldo.

**Segunda pasada — idempotencia**:

```bash
/tmp/mem_seed install /tmp/probe_legado 2>&1 | grep -i "respald\|elimin"
```

**Esperado**: sin salida. No hay nada que limpiar y la limpieza calla.

---

## 6. JSON ajeno e ilegible (FR-021, SC-008)

```bash
rm -rf /tmp/probe_json && mkdir -p /tmp/probe_json/.cline
printf 'esto no es json {{{\n' > /tmp/probe_json/.cline/mcp_settings.json
/tmp/mem_seed install /tmp/probe_json 2>&1 | grep -i "cline"
cat /tmp/probe_json/.cline/mcp_settings.json
```

**Esperado**: el archivo queda byte por byte igual y la salida informa que se dejó
sin tocar.

---

## 7. Constitución bajo demanda (US4, SC-006)

```bash
cd /tmp/probe_limpio
./mem constitution | wc -l          # ~635
./mem constitution | head -3        # encabezado de la constitución
ls .claude/skills/constitution/SKILL.md
grep -c "Constitución Genérica" .claude/skills/constitution/SKILL.md
```

**Esperado**: el documento completo por stdout; el envoltorio existe y su
`grep` devuelve `0` — el envoltorio **no** lleva copia del texto (contrato de
`/constitution`).

Sincronización con y sin spec-kit:

```bash
cd /tmp/probe_limpio && ./mem constitution --sync    # sin .specify -> informa que no aplica
mkdir -p /tmp/probe_limpio/.specify/memory
./mem constitution --sync && head -3 .specify/memory/constitution.md
```

Y que sirve la versión editada, no la plantilla:

```bash
cd /tmp/probe_limpio && ./mem constitution | grep -c "TEXTO EDITADO"   # tras editar la semilla
```

---

## 8. Siembra sin `mem install` (US1, FR-004)

```bash
rm -rf /tmp/probe_mcp && mkdir -p /tmp/probe_mcp && cd /tmp/probe_mcp
echo '' | /tmp/mem_seed mcp 2>&1 | head -5      # arranca y termina al cerrar stdin
/tmp/mem_seed context | grep -c "Reglas de trabajo (memoria fijada)"
```

**Esperado**: `1`. Las semillas existen sin haber ejecutado `install` ni una vez.

---

## 9. Diagnóstico sin falsas alarmas (FR-028, SC-007)

```bash
cd /tmp/probe_limpio && ./mem doctor
```

**Esperado**: ningún canal de instrucciones de ámbito **proyecto** reportado como
`missing`. Debe leerse como no aplicable, con su motivo. El ámbito **usuario** se
sigue evaluando normalmente.

---

## 10. `mem update` hereda todo (FR-015)

`mem update` delega en `install` (`cmd_update.go:116`), así que basta con comprobar
que la delegación sigue en pie. Con el `httptest` de `cmd_update_test.go`:

```bash
go test ./adapters/primary/cli/ -run TestCmdUpdate -v 2>&1 | tail -20
```

---

## 11. Defecto de la clave de tópico corregido (FR-030, SC-010)

Antes del arreglo, listar memorias devolvía la clave de tópico siempre vacía. Prueba
directa contra el binario:

```bash
cd /tmp/probe_limpio
./mem save -t "prueba de clave" -y learning "contenido de prueba"   # sin topic_key
./mem list --limit 5
```

La comprobación real es sobre la vía que estaba rota — la tool MCP `list_memories`,
que renderiza lo que devuelve `List`:

```bash
cd /tmp/probe_limpio && ./mem context | grep -c "Reglas de trabajo del proyecto"
```

**Esperado**: `0`. Ese `0` **es** la prueba del arreglo: la exclusión de FR-009
compara por clave de tópico, y solo puede funcionar si `List` la trae. Si el defecto
siguiera vivo, la comparación daría falso y el título aparecería en
`## Preferencias del Usuario`, además de la sección fijada.

Y en la suite:

```bash
go test ./adapters/secondary/persistence/ -run TopicKey -v 2>&1 | tail -15
```

---

## 12. Semilla no desaparece por antigüedad (FR-031, SC-009)

```bash
rm -rf /tmp/probe_ventana && mkdir -p /tmp/probe_ventana
/tmp/mem_seed install /tmp/probe_ventana > /dev/null
cd /tmp/probe_ventana
for i in $(seq 1 200); do ./mem save -t "ruido $i" -y learning "memoria de relleno $i" > /dev/null; done
./mem context | grep -c "Reglas de trabajo (memoria fijada)"
```

**Esperado**: `1`. Con 200 memorias más recientes que la semilla, las reglas siguen
ahí. Si devuelve `0`, la sección depende de la ventana de recencia y FR-031 no se
cumplió.

---

## 13. Siembra inerte — las tres brechas cerradas (FR-032..FR-034, SC-011)

### G1 — el texto guardado es idéntico al origen

```bash
cd /tmp/probe_limpio
./mem context | grep -c "REDACTED"
```

**Esperado**: `0`. La comprobación estricta vive en la suite, que compara byte a byte
lo persistido con la plantilla embebida:

```bash
go test ./application/usecases/ -run SeedDefaults -v 2>&1 | tail -15
```

### G2 — sembrar no crea relaciones automáticas

```bash
rm -rf /tmp/probe_inerte && mkdir -p /tmp/probe_inerte
/tmp/mem_seed install /tmp/probe_inerte > /dev/null
cd /tmp/probe_inerte && ./mem context | grep -c "Sinapsis"
```

**Esperado**: `0`. Un proyecto recién sembrado no tiene ninguna sinapsis: las dos
semillas no están enlazadas entre sí ni con nada.

### G3 — sembrar no publica en el ADR externo (la brecha activa)

Este es el escenario que la primera lectura del diseño daba por inofensivo:

```bash
rm -rf /tmp/probe_adr && mkdir -p /tmp/probe_adr/.memory
printf '{"adr_sync_enabled": true}
' > /tmp/probe_adr/.memory/settings.json
/tmp/mem_seed install /tmp/probe_adr 2>&1 | grep -i "adr" || echo "ok: sin actividad ADR"
cd /tmp/probe_adr && ./mem adr-sync status 2>&1 | tail -5
```

**Esperado**: ninguna exportación registrada para las semillas. Con la vía normal, la
constitución de 635 líneas se habría publicado en el documento ADR externo, síncrona,
sin que nadie lo pidiera.

---

## 14. Reemplazar las reglas del equipo en 3 pasos (US5, SC-012)

El recorrido completo que legitima el modelo de semillas: el contenido pasa a ser
del equipo, no el que trae la herramienta.

```bash
cd /tmp/probe_limpio
./mem docs list
```

**Esperado**: dos filas — `rules` y `constitution` — ambas en estado `por defecto`,
con su número de líneas y fecha.

```bash
# 1. Exportar
./mem docs export rules -o /tmp/reglas_equipo.md
# 2. Editar
printf '\n## Regla propia del equipo\n\n- Nada se despliega en viernes.\n' >> /tmp/reglas_equipo.md
# 3. Importar
./mem docs import rules /tmp/reglas_equipo.md
```

**Esperado**: confirmación con la ruta y el número de líneas. Y el cambio llega al
contexto:

```bash
./mem context | grep -c "Nada se despliega en viernes"   # -> 1
./mem docs list | grep rules                              # -> personalizado
```

**Esperado**: `1` y `personalizado`. Si el `grep` sobre el contexto da `0`, el
documento importado perdió su identidad de documento fijado (FR-038).

### stdout limpio (contrato de `show`)

```bash
./mem docs show rules > /tmp/solo_reglas.md
diff <(./mem docs show rules) /tmp/solo_reglas.md && echo "ok: stdout sin ruido"
```

**Esperado**: sin diferencias. Ningún aviso debe contaminar stdout.

---

## 15. El documento importado es intocable (US5, SC-014, FR-003)

```bash
cd /tmp/probe_limpio
for i in 1 2 3; do /tmp/mem_seed install /tmp/probe_limpio > /dev/null; done
./mem context | grep -c "Nada se despliega en viernes"    # -> 1
```

**Esperado**: `1`. Reinstalar no revierte lo que el equipo puso.

### Restaurar cuando se quiere volver atrás

```bash
./mem docs reset rules
./mem docs list | grep rules                              # -> por defecto
./mem context | grep -c "Nada se despliega en viernes"    # -> 0
```

---

## 16. Rechazos y paridad (US5, SC-013, FR-040, FR-042)

### Archivo vacío: se rechaza y no destruye nada

```bash
cd /tmp/probe_limpio
./mem docs import rules /tmp/reglas_equipo.md > /dev/null    # dejar contenido conocido
: > /tmp/vacio.md
./mem docs import rules /tmp/vacio.md; echo "exit=$?"
./mem context | grep -c "Nada se despliega en viernes"       # -> 1
```

**Esperado**: `exit` distinto de 0, motivo claro, y el documento anterior **intacto**.
Un `0` en el último `grep` significa que un import fallido borró el documento — el
peor modo de fallo posible de esta capacidad.

### Alias desconocido y clave arbitraria

```bash
./mem docs show inexistente; echo "exit=$?"        # error + lista de alias válidos
./mem docs import --topic "equipo:runbook" /tmp/reglas_equipo.md
./mem search "runbook" | head -3
```

**Esperado**: el alias desconocido falla informando los válidos; la importación por
clave arbitraria funciona aunque `equipo:runbook` no esté en el catálogo (FR-042).

### Paridad con la TUI

```bash
cd /tmp/probe_limpio && ./mem
```

En la pantalla de configuración, al final del menú, deben aparecer
`Actualizar Reglas IA: <estado>` y `Actualizar Constitución: <estado>`. Cada una abre
una pantalla con **ver, exportar, importar y restaurar** — las mismas cuatro
operaciones de la consola. `restaurar` debe pedir confirmación.

El test de contrato lo verifica sin intervención manual:

```bash
go test ./adapters/primary/tui/ -run Docs -v 2>&1 | tail -15
```

---

## Limpieza

```bash
rm -rf /tmp/probe_limpio /tmp/probe_legado /tmp/probe_json /tmp/probe_mcp \
       /tmp/probe_ventana /tmp/probe_inerte /tmp/probe_adr \
       /tmp/reglas_equipo.md /tmp/solo_reglas.md /tmp/vacio.md /tmp/mem_seed
```

---

## Matriz de trazabilidad

| Escenario | Historia | Requisitos | Criterios |
|---|---|---|---|
| 1 | todas | — | — |
| 2 | US2 | FR-011, FR-012, FR-013, FR-014 | SC-001 |
| 3 | US1 | FR-007, FR-008, FR-009 | SC-002 |
| 4 | US1 | FR-003, FR-005 | SC-003 |
| 5 | US3 | FR-016..FR-020, FR-022, FR-023 | SC-004, SC-008 |
| 6 | US3 | FR-021 | SC-008 |
| 7 | US4 | FR-024, FR-025, FR-026, FR-027 | SC-006 |
| 8 | US1 | FR-001, FR-002, FR-004, FR-006 | SC-002 |
| 9 | US2 | FR-028, FR-029 | SC-007 |
| 10 | US2 | FR-015 | SC-001 |
| 11 | US1 | FR-030, FR-009 | SC-010 |
| 12 | US1 | FR-031, FR-006 | SC-009 |
| 13 | US1 | FR-032, FR-033, FR-034 | SC-011 |
| 14 | US5 | FR-035..FR-038, FR-044 | SC-012 |
| 15 | US5 | FR-003, FR-043 | SC-014 |
| 16 | US5 | FR-040, FR-041, FR-042 | SC-013 |
