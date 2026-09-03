# Guía de validación: Octopus AAR

Cómo comprobar que la funcionalidad hace lo que dice, contra el binario real y no solo contra la suite de pruebas. Sigue la regla de campo del proyecto: verde en tests no es "funciona".

## Requisitos previos

- Go 1.25 o superior.
- Repositorio de gomemory con memoria inicializada (basta ejecutar cualquier comando de `mem` una vez).
- Binario recién compilado. **Compilar es obligatorio en cada verificación**: `mem` es un binario de 17 MB versionado en la raíz, y validar contra la copia anterior es el equivalente a `docker compose up` sin `--build`.

```bash
go build -o mem ./infrastructure
./mem version
```

## 1. Estado inicial: el módulo nace apagado

```bash
./mem octopus status
```

**Esperado**: mensaje de módulo desactivado, indicación de cómo activarlo, código de salida distinto de cero.

```bash
cat .memory/settings.json | grep -c octopus_enabled   # esperado: 0
```

El ajuste no existe todavía. Ausente significa apagado.

## 2. Huella cero con el módulo apagado

La comprobación que de verdad importa (SC-001). Se contrasta contra el servidor MCP real, no contra un mock.

`mem mcp` no tiene bandera de listado: habla JSON-RPC por stdio y **el proceso
sale en cuanto se cierra stdin**, así que un `printf ... | mem mcp` no llega a
responder. La verificación que sí funciona es la prueba de contrato, que levanta
el servidor real y lo interroga manteniendo la conexión abierta:

```bash
go test ./tests/contract/ -run 'TestOctopusApagado|TestOctopusEncendido' -count=1 -v
```

**Esperado**: las dos en verde. La primera comprueba que con el módulo apagado la
superficie es EXACTAMENTE `domain.MCPAllTools()`, sin tolerancia; la segunda, que
con el módulo encendido es base + `domain.MCPOctopusTools`.

Contraste medido en esta implementación: **19 tools con el módulo apagado, 20 con
el módulo encendido**. La diferencia es exactamente `octopus_route_task`.

```bash
# El bloque de protocolo no menciona Octopus
./mem context | grep -ci octopus                               # esperado: 0
```

```bash
# Ninguna fila escrita. La base vive en el store global, no en .memory/:
#   ~/.local/share/gomemory/projects/<clave-de-proyecto>/mem.db
# El store global tiene una carpeta por proyecto, así que `find ... | head -1`
# devolvería la de CUALQUIERA. Tras ejecutar el comando anterior, la base de
# este proyecto es la modificada más recientemente:
DB=$(ls -t "$HOME/.local/share/gomemory/projects"/*/mem.db | head -1)
sqlite3 "$DB" 'SELECT COUNT(*) FROM octopus_executions;'       # esperado: 0
```

## 3. Encender el módulo

Por la TUI, que es la vía que pidió el usuario:

```bash
./mem
# → Configuración → fila "Octopus AAR: off" → enter
```

**Esperado**: la fila pasa a `on` y aparece un mensaje de estado. Al salir y volver a entrar, sigue en `on`.

Verificación de persistencia:

```bash
grep octopus_enabled .memory/settings.json    # esperado: "octopus_enabled": true
```

## 4. Decisión unitaria: lo trivial se queda en casa

```bash
./mem octopus route "Corregir una errata en un comentario" --class trivial --json
```

**Esperado**: `route: INLINE`, razón `el sobrecosto de delegar supera el beneficio esperado`. Cubre AC-001.

## 5. Decisión unitaria: la investigación aislable se delega

```bash
./mem octopus route "Determinar si la limpieza por expiración compite con el refresco de memorias" \
  --class investigation --read-only \
  --files internal/memory/expiration.go,internal/memory/store.go --json
```

**Esperado**: `route: DELEGATE` con presupuesto de contexto y de salida declarados, y permisos de solo lectura en el contrato. Cubre AC-002 y AC-020.

Sin capacidades de subagentes declaradas, la misma llamada debe devolver `INLINE` con razón `el runtime no declara soporte de subagentes` — cubre AC-003 y confirma que el default es conservador.

## 6. Plan completo: dependencias, paralelismo y topes

Con el escenario extremo a extremo de la especificación (§66):

```bash
./mem octopus plan --file specs/027-octopus-aar/ejemplo-plan.json --json
```

**Esperado**, según el escenario: T001 y T002 `INLINE`; T003 y T004 en el mismo grupo paralelo; T005 en `WAIT` con T002 y T003 como bloqueantes; T006 `DELEGATE`. Cubre AC-004 y AC-005.

Determinismo (SC-006):

```bash
# `sort -u` sobre la salida contaría LÍNEAS únicas, no salidas únicas: el JSON
# va indentado. Se compara el hash de cada corrida completa.
for i in $(seq 1 20); do ./mem octopus plan --file ejemplo-plan.json --json | md5sum; done | sort -u | wc -l
# esperado: 1
```

Medido en esta implementación: **1 hash único en 20 corridas**.

Fan-out (AC-009): un plan de 20 tareas independientes con `--max-agents 4` no debe producir más de cuatro delegaciones.

```bash
./mem octopus plan --file plan-20.json --max-agents 4 --json | grep -c '"route": *"DELEGATE"'
# esperado: <= 4
```

## 7. Presupuesto y reserva de validación

Con un presupuesto que solo deja tokens dentro de la reserva, una delegación opcional debe pasar a `INLINE` con razón `los tokens restantes pertenecen a la reserva de validación`. Cubre AC-007 y AC-008.

```bash
./mem octopus plan --file ejemplo-plan.json --budget 14000 --json \
  | grep -E 'validation_reserve_protected|budget_exhausted'
```

**Esperado**: la tarea **opcional** del escenario (T006, documentación) sale con
`validation_reserve_protected`, y una imprescindible con `budget_exhausted`. La
distinción importa: no es lo mismo "no hay presupuesto" que "lo que queda es la
reserva y no se toca". Medido: reserva intacta con 2100 tokens.

## 8. La simulación no ejecuta nada

```bash
ps -eo comm | sort > /tmp/antes.txt
./mem octopus plan --file plan-20.json > /dev/null
ps -eo comm | sort > /tmp/despues.txt
diff /tmp/antes.txt /tmp/despues.txt    # esperado: sin diferencias
```

Cubre AC-019 e INV-AAR-018.

## 9. Telemetría y privacidad

Tras reportar una ejecución:

```bash
./mem octopus usage
./mem octopus history -n 10
```

**Esperado**: agregados por ruta, consumo estimado frente a real, y toda cifra sin medición exacta marcada como estimada.

Auditoría de privacidad (SC-011, INV-AAR-013):

```bash
# El store global tiene una carpeta por proyecto, así que `find ... | head -1`
# devolvería la de CUALQUIERA. Tras ejecutar el comando anterior, la base de
# este proyecto es la modificada más recientemente:
DB=$(ls -t "$HOME/.local/share/gomemory/projects"/*/mem.db | head -1)
sqlite3 "$DB" '.schema octopus_executions'
```

**Esperado**: ninguna columna de texto libre alimentada por contenido. Solo identificadores, enums y cifras. Si alguna vez aparece una columna de contenido en ese esquema, la invariante está rota aunque las pruebas pasen.

## 10. Suite completa

```bash
go test ./... -count=1
go test ./domain/ -run Octopus -count=1 -v
gofumpt -l .          # esperado: sin salida
golangci-lint run
```

## Qué mirar si algo no cuadra

1. **"Encendí el módulo y no aparecen las tools"** → primero el binario, no el código: `go build -o mem ./infrastructure` y reinicia el cliente MCP, que cachea la lista de tools de la sesión.
2. **"El plan sale distinto en cada corrida"** → hay un mapa recorrido sin ordenar dentro de la política. Es el fallo previsto en el plan; se arregla ordenando por identificador de tarea, no envolviendo la salida.
3. **"Con el módulo apagado veo algo de Octopus"** → es un defecto, no una molestia estética: rompe INV-AAR-019 y SC-001.
