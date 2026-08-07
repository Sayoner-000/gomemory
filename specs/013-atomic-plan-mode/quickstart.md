# Guía de validación — Modo Plan Atómico con Memoria

**Feature**: 013-atomic-plan-mode

Escenarios ejecutables que demuestran que la funcionalidad opera de punta a punta. Siguen
la regla de trabajo del proyecto: **"verde en tests" no es "funciona"** — cada escenario se
verifica contra el binario realmente construido, no solo con pruebas unitarias.

## Prerrequisitos

```bash
cd /Users/josegomezj/home/rcw/go_memory
go build -o mem ./infrastructure    # el binario que se va a verificar
```

Verificar que se está probando el binario recién compilado y no uno viejo del `PATH`:

```bash
./mem version
```

---

## Escenario 1 — La ruta de contexto de planificación devuelve método y contexto

**Cubre**: FR-001, FR-005, FR-006, FR-007 · Historia 1

```bash
./mem plan-context | head -40
echo "código de salida: $?"
```

**Resultado esperado**:

- El documento empieza por el método de descomposición atómica.
- Más abajo aparece el contexto histórico del proyecto, con sus secciones por tipo
  (decisiones, patrones, bugfixes).
- Código de salida `0`.

**Comprobar el presupuesto** (FR-007) — la salida no debe exceder el techo configurado:

```bash
./mem plan-context | wc -c
grep -o '"budget":[0-9]*' .memory/settings.json 2>/dev/null || echo "sin techo configurado"
```

---

## Escenario 2 — Proyecto sin historial previo

**Cubre**: FR-034, SC-009 · Historia 1, escenario 2

Prueba clave: sin historial, el **método debe seguir llegando**. Es lo que mantiene la
Historia 2 independiente de la Historia 1.

```bash
tmp=$(mktemp -d) && cd "$tmp"
/Users/josegomezj/home/rcw/go_memory/mem plan-context
echo "código de salida: $?"
cd - >/dev/null && rm -rf "$tmp"
```

**Resultado esperado**: se emite el método, código de salida `0`, sin mensajes de error.

> **Corregido tras verificar contra el binario.** La versión anterior de este escenario
> esperaba que **no** se emitiera contexto. Es falso en este sistema: gomemory inicializa
> el store de forma perezosa al primer uso, así que en un directorio limpio
> `ContextBuilder.Build()` no falla — devuelve un contexto mínimo, con solo los
> encabezados. La rama estrictamente degradada (solo método, sin contexto) existe y está
> cubierta, pero solo se alcanza cuando `Build()` devuelve error; se prueba de forma
> determinista en el test unitario del caso de uso, no aquí.

---

## Escenario 3 — El interruptor de apagado silencia la salida

**Cubre**: FR-032, SC-012 · Historia 4

```bash
tmp=$(mktemp -d) && cd "$tmp"
/Users/josegomezj/home/rcw/go_memory/mem init
printf '{"atomic_plan_disabled":true}' > .memory/settings.json

out=$(/Users/josegomezj/home/rcw/go_memory/mem plan-context); code=$?
[ -z "$out" ] && echo "OK: salida vacía" || echo "FALLO: emitió contenido"
echo "código de salida: $code (debe ser 0)"

printf '{"atomic_plan_disabled":false}' > .memory/settings.json
/Users/josegomezj/home/rcw/go_memory/mem plan-context | head -3
cd - >/dev/null && rm -rf "$tmp"
```

**Resultado esperado**: apagado ⇒ salida vacía con código `0`; reactivado ⇒ vuelve el
método sin reinstalar nada.

**Distinción que este escenario valida**: comparado con el Escenario 2, confirma que
"sin memoria" (degrada a solo método) y "apagado" (silencia todo) son ramas distintas.

---

## Escenario 4 — El bloque de protocolo se actualiza de v5 a v6

**Cubre**: FR-002, FR-029, FR-030 · Historia 3

```bash
tmp=$(mktemp -d) && cd "$tmp"
git init -q
printf '# Instrucciones\n\n<!-- gomemory-protocol-v5 -->\n## Memoria Persistente (`mem`) — Protocolo Activo\n\ncontenido viejo\n' > AGENTS.md

/Users/josegomezj/home/rcw/go_memory/mem install . >/dev/null 2>&1

echo "--- marcadores presentes ---"
grep -c "gomemory-protocol-v5" AGENTS.md   # debe ser 0: no quedan restos
grep -c "gomemory-protocol-v6" AGENTS.md   # debe ser 1
echo "--- sección de modo plan ---"
grep -A8 "[Mm]odo plan" AGENTS.md | head -12
cd - >/dev/null && rm -rf "$tmp"
```

**Resultado esperado**: cero apariciones de `v5` (sin restos de la versión anterior,
FR-030), exactamente una de `v6`, y la sección de modo plan presente con las tres formas
del disparador.

**Idempotencia** (FR-029) — reinstalar sobre un proyecto sin cambios no debe modificar
nada:

```bash
tmp=$(mktemp -d) && cd "$tmp" && git init -q
/Users/josegomezj/home/rcw/go_memory/mem install . >/dev/null 2>&1
sum1=$(md5 -q AGENTS.md 2>/dev/null || md5sum AGENTS.md | cut -d' ' -f1)
/Users/josegomezj/home/rcw/go_memory/mem install . >/dev/null 2>&1
sum2=$(md5 -q AGENTS.md 2>/dev/null || md5sum AGENTS.md | cut -d' ' -f1)
[ "$sum1" = "$sum2" ] && echo "OK: idempotente" || echo "FALLO: reescribió contenido idéntico"
cd - >/dev/null && rm -rf "$tmp"
```

---

## Escenario 5 — Cobertura universal: un agente sin integración dedicada

**Cubre**: FR-002, FR-027, SC-005 · Historia 3, escenario 2

Es el escenario que valida la decisión de diseño central: la cobertura llega por el
protocolo común, no por integración por agente.

```bash
tmp=$(mktemp -d) && cd "$tmp" && git init -q
/Users/josegomezj/home/rcw/go_memory/mem install . >/dev/null 2>&1

echo "--- archivos de agente que recibieron el disparador ---"
for f in AGENTS.md CLAUDE.md .cursorrules .windsurfrules; do
  [ -f "$f" ] && printf "%-18s %s\n" "$f" "$(grep -c 'plan-context' "$f") referencia(s)"
done
cd - >/dev/null && rm -rf "$tmp"
```

**Resultado esperado**: los archivos de agente presentes contienen la referencia al
disparador. Cursor y Windsurf quedan cubiertos sin una sola línea de código específica
para ellos — que es exactamente lo que SC-005 mide.

---

## Escenario 6 — Pre-aprobación de la herramienta en Claude Code

**Cubre**: FR-001, D10 de `research.md`

Sin esto la activación autónoma queda bloqueada pidiendo permiso en cada planificación.

```bash
tmp=$(mktemp -d) && cd "$tmp" && git init -q
/Users/josegomezj/home/rcw/go_memory/mem install . >/dev/null 2>&1
grep -o "mcp__gomemory__get_plan_context" .claude/settings.json || echo "FALLO: no pre-aprobada"
cd - >/dev/null && rm -rf "$tmp"
```

**Resultado esperado**: la herramienta aparece en `permissions.allow`.

---

## Escenario 6b — Pre-aprobación en OpenCode

**Cubre**: FR-001, D11 de `research.md`

OpenCode parte de cero: hoy no tiene ninguna gestión de permisos (verificado en D11). Sin
esto, la activación autónoma no funciona en OpenCode aunque todo lo demás esté bien.

```bash
tmp=$(mktemp -d) && cd "$tmp" && git init -q
/Users/josegomezj/home/rcw/go_memory/mem install . >/dev/null 2>&1

echo "--- esquema de permisos ---"
python3 -c "
import json
cfg = json.load(open('opencode.json'))
p = cfg.get('permission', {})
print('permission presente:', bool(p))
print('gomemory_* =', p.get('gomemory_*'))
print('gomemory_forget_memory =', p.get('gomemory_forget_memory'))
assert p.get('gomemory_*') == 'allow', 'FALLO: comodín no pre-aprobado'
assert p.get('gomemory_forget_memory') == 'ask', 'FALLO: forget_memory no está protegida'
assert 'autoApprove' not in json.dumps(cfg), 'FALLO: se escribió el esquema de Claude, que OpenCode ignora'
print('OK')
"
cd - >/dev/null && rm -rf "$tmp"
```

**Resultado esperado**: `permission.gomemory_*` en `allow`, `permission.gomemory_forget_memory`
en `ask`, y **ninguna** aparición de `autoApprove` — esa clave sería el esquema de Claude
Code, que OpenCode ignora por completo.

**Idempotencia**: reinstalar no debe duplicar ni alterar la configuración, ni perder las
claves de `permission` que la persona haya añadido por su cuenta.

**Verificación contra OpenCode en ejecución** (no basta con inspeccionar el archivo — regla
de trabajo del proyecto):

```bash
opencode debug config | grep -A5 permission
```

Es la misma vía con la que la feature 005 confirmó empíricamente el comportamiento de
configuración de OpenCode. Después, planificar en OpenCode dentro de un proyecto con
gomemory y comprobar que **no aparece ningún diálogo de aprobación**.

---

## Escenario 7 — La herramienta MCP devuelve lo mismo que el comando

**Cubre**: FR-003 · contrato "las dos vías devuelven el mismo documento"

```bash
./mem plan-context > /tmp/via-cli.md
echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_plan_context","arguments":{}}}' \
  | ./mem mcp 2>/dev/null | head -5
```

**Resultado esperado**: la herramienta aparece registrada y su contenido coincide con el
del comando. La equivalencia entre ambas vías es lo que permite que un agente sin MCP no
quede con una versión degradada (SC-006).

---

## Escenario 8 — El plan aprobado se registra conservando su árbol

**Cubre**: FR-035, FR-036 · Historia 5

Verifica lo que D9 de `research.md` identificó como el único punto a comprobar de la ruta
existente: que el árbol con caracteres de dibujo sobrevive intacto.

```bash
cat <<'PLAN' | ./mem hook plan-approved
{"plan":"🎯 Objetivo de prueba\n├─ [1] Primera subtarea\n│  └─ [1.1] ✓ hoja atómica → resultado\n└─ [2] ✓ hoja atómica (dep: 1.1)"}
PLAN

./mem list --limit 1
```

**Resultado esperado**: se guarda una memoria `type=decision`; al recuperarla, los
caracteres de dibujo (`├─`, `│`, `└─`), las marcas `✓` y la anotación `dep: 1.1` están
intactos, y el título derivado de la primera línea es legible.

**Plan rechazado** (FR-036): no se ejecuta ninguna acción, así que no hay nada que probar
positivamente — la garantía es estructural (en Claude Code, el disparador no se ejecuta si
la persona rechaza).

---

## Escenario 9 — Verificación de punta a punta en el agente real

**Cubre**: Historias 1 y 2 en conjunto · SC-001, SC-002, SC-003

Los ocho escenarios anteriores prueban la maquinaria. Este prueba el resultado, y es el
único que no se puede automatizar.

1. En este mismo repositorio (que tiene historial acumulado), entrar en modo plan en Claude
   Code y pedir un plan sobre un área con decisiones y bugs registrados.
2. **Verificar (SC-001)**: el plan referencia al menos un elemento del historial —una
   decisión previa, una causa raíz o una convención— que no se mencionó en la solicitud.
3. **Verificar (SC-002)**: cada hoja del árbol declara un resultado verificable, o está
   marcada como no atómica con su motivo.
4. **Verificar (SC-003)**: se puede recorrer el plan y determinar, tarea por tarea, si está
   cumplida, sin preguntarle nada al agente.
5. **Verificar (FR-020)**: el agente entrega el árbol y **se detiene**. No empieza a
   ejecutar.
6. Repetir en OpenCode y comparar: el comportamiento debe ser equivalente (SC-006).

---

## Pruebas automatizadas

Según el principio III de la constitución (TDD no negociable, cobertura ≥ 80 %):

```bash
go test ./... -cover
go test ./application/usecases/... -run PlanContext -v
go test ./adapters/primary/setup/... -v
golangci-lint run
```

**Advertencia registrada en las reglas de trabajo del proyecto**: un test vale lo que vale
su fixture. Los escenarios 1 a 8 de esta guía se ejecutan contra el binario construido
precisamente porque las pruebas unitarias con dobles de prueba pueden pasar en verde
mientras la ruta real falla.
