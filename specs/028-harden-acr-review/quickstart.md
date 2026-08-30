# Guía de validación: Fortalecimiento de la revisión ACR

Escenarios ejecutables que demuestran que la funcionalidad 028 hace lo que dice. No
son los tests unitarios: son la comprobación contra el binario real y la base de datos
real, que es lo que el proyecto exige antes de dar algo por terminado ("verde en tests"
no es "funciona").

Cada escenario cita el criterio de éxito que valida.

## Prerrequisitos

- Go 1.25 o superior en el `PATH`
- Un repositorio git (los targets `--diff`, `--commit` y `--pending` lo requieren)
- Un directorio de datos aislado para no tocar la memoria real del proyecto

```bash
cd /Users/josegomezj/home/rcw/gomemory
export GOMEMORY_DATA_HOME="$(mktemp -d)/gomemory"
go build -o /tmp/mem-028 ./infrastructure/...   # ajustar a la ruta real del main
```

> El aislamiento de `GOMEMORY_DATA_HOME` no es opcional: sin él la validación escribe
> en la base de memorias real del proyecto.

---

## Escenario 0 — La migración no rompe una base existente

**Valida**: la restricción de migración aditiva del plan.

```bash
cp ~/.local/share/gomemory/projects/<proyecto>/mem.db /tmp/mem-antes.db
GOMEMORY_DATA_HOME=/tmp/migracion /tmp/mem-028 review history --limit 1
sqlite3 /tmp/mem-antes.db "SELECT COUNT(*) FROM reviews;"
```

**Esperado**: la orden abre la base sin error, `PRAGMA table_info(reviews)` muestra las
columnas nuevas con `notnull = 0`, la tabla `rejudgments` existe y el número de filas de
`reviews`, `findings` y `consensus_findings` es idéntico al de antes.

---

## Escenario 1 — Un hallazgo sin clasificar impide aprobar

**Valida**: SC-001, FR-001, FR-004.

1. Inicia una revisión y anota `review_id` y `target_digest`.
2. Envía dos resultados `success`: el revisor A con dos hallazgos (uno `HIGH`), el
   revisor B con uno.
3. Llama a `review_consensus` clasificando **solo dos** de los tres hallazgos.

**Esperado**: la llamada se rechaza con `quedan 1 hallazgos sin clasificar: A-002`.
Nada se escribe en `consensus_findings`, y `review_finalize` no devuelve `APPROVED`.

---

## Escenario 2 — La severidad no se puede degradar

**Valida**: SC-001, FR-003.

Con dos hallazgos `HIGH` corroborados, llama a `review_consensus` declarando
`"severity": "LOW"` en el `match`.

**Esperado**: rechazo con `la severidad declarada LOW no coincide con la derivada de
las fuentes HIGH`. Si se omite el campo, la fila se persiste con `HIGH`, no con vacío.

---

## Escenario 3 — El consenso es idempotente pero no reemplazable

**Valida**: FR-005.

1. Registra una clasificación completa. Anota los `consensus_local_id`.
2. Reenvía **exactamente** la misma, con los `matches` en otro orden.
3. Reenvía una distinta (cambia un `SUSPECT` a `INFO`).

**Esperado**: (2) devuelve `idempotent: true`, los mismos identificadores y ninguna
escritura; (3) se rechaza con `la ronda 0 ya tiene un consenso registrado y no admite
reemplazo` y el ledger conserva la clasificación original.

---

## Escenario 4 — Un revisor solo no resuelve nada

**Valida**: SC-002, FR-013, FR-014.

1. Registra una corrección con `review_fix_record` que aborde `C-001`.
2. Llama a `review_rejudge` con `reviewer: "A"` y `C-001: RESOLVED`.
3. Consulta `review_status`.

**Esperado**: `aggregate_state` es `UNRESOLVED`. Solo tras el re-juicio de B con
`RESOLVED` pasa a `RESOLVED`. Si B declara `REGRESSED`, el agregado es `REGRESSED`
aunque A dijera `RESOLVED`.

También: intentar re-juzgar `C-002`, que la corrección no incluye, se rechaza con
`el hallazgo C-002 no forma parte de la corrección de la ronda 1`.

---

## Escenario 5 — La cadena de targets no admite saltos

**Valida**: FR-009.

Tras una primera corrección `e7114e8c… → a91f3b02…`, intenta registrar una segunda
corrección con `base_target_digest: e7114e8c…`.

**Esperado**: rechazo con `la corrección parte de e7114e8c… pero el target vigente es
a91f3b02…`.

---

## Escenario 6 — Cien correcciones concurrentes dejan una sola ronda

**Valida**: SC-003, FR-010.

```bash
for i in $(seq 1 100); do
  /tmp/mem-028 review fix-record --review "$REVIEW_ID" \
    --addressed C-001 --base "$BASE" --fixed "fixed-$i" &
done
wait
sqlite3 "$DB" "SELECT round, COUNT(*) FROM fix_rounds GROUP BY round;"
```

**Esperado**: exactamente una fila para la ronda 1. Las 99 restantes fallan con
`la ronda 1 ya fue registrada por otra corrección`. `reviews.round` vale 1 y
`current_target_digest` corresponde a la corrección ganadora, no a otra.

---

## Escenario 7 — Una revisión de solo lectura termina, no se bloquea

**Valida**: SC-005, FR-019. **Este es el defecto reproducido en la revisión original.**

```bash
/tmp/mem-028 review --diff --read-only
# …enviar dos resultados success con un HIGH corroborado, clasificarlo CONFIRMED…
# …llamar a review_finalize una sola vez…
```

**Esperado**: `verdict: ESCALATED` y `status: escalated` en la primera llamada. El
comportamiento actual —`review is not ready to finalize` con la revisión atascada en
`consensus_ready`— es la regresión que este escenario detecta.

Contraste: la misma revisión con `fix_authorized: true` y rondas disponibles **sigue**
devolviendo que no está lista. Ese caso no cambia.

---

## Escenario 8 — Un estado terminal es inmutable

**Valida**: FR-016.

Sobre una revisión `escalated`, intenta `review_submit`, `review_consensus`,
`review_fix_record`, `review_rejudge` y `review_promote_memory`.

**Esperado**: las cinco fallan con `la revisión está en estado terminal escalated y no
admite cambios`, y `review_status` devuelve exactamente lo mismo que antes de los cinco
intentos.

---

## Escenario 9 — Las métricas coinciden con el contrato

**Valida**: SC-007, FR-024.

```bash
# desde el cliente MCP, capturar la respuesta cruda de review_finalize
jq -S 'keys' <<< "$RESPUESTA_METRICS"
```

**Esperado**: exactamente `["contradictions","duration","findings_confirmed",
"findings_suspect","findings_total","fix_rounds","memory_deduplicated",
"memory_promoted"]`. Ni un campo en PascalCase, ni uno de los ocho ausente.
`duration` es un entero de segundos mayor que cero.

---

## Escenario 10 — El target pendiente incluye lo que `--diff` no ve

**Valida**: SC-004, FR-025, FR-026.

```bash
echo "contenido" > "archivo nuevo con espacios.txt"   # sin seguimiento
git add -N . 2>/dev/null || true
/tmp/mem-028 review --pending     # anotar digest D1
echo "otro contenido" >> "archivo nuevo con espacios.txt"
/tmp/mem-028 review --pending     # anotar digest D2
```

**Esperado**: `D1 != D2`. Repetir la orden sin tocar nada devuelve el mismo digest. Un
archivo ignorado por `.gitignore` no altera el digest. En un árbol limpio, la orden
falla con `no hay cambios pendientes que revisar` y código de salida 1.

---

## Escenario 11 — Un auditor reconstruye el linaje con una sola consulta

**Valida**: SC-006, FR-023.

```bash
/tmp/mem-028 review show "$REVIEW_ID"
```

**Esperado**: la salida muestra, para cada hallazgo de consenso, sus IDs fuente, su
severidad derivada, la ronda de corrección que lo abordó, el re-juicio de cada revisor
y el estado agregado — sin necesidad de abrir `mem.db` ni ningún archivo interno.

---

## Escenario 12 — La promoción exige una revisión aprobada

**Valida**: FR-021.

Con un hallazgo `CONFIRMED` y `RESOLVED` en una revisión que aún no ha finalizado,
llama a `review_promote_memory`.

**Esperado**: rechazo con `la revisión no está aprobada (…)`. Tras finalizar como
`APPROVED`, la misma llamada tiene éxito y `review_finalize` refleja
`memory_promoted: 1`.

---

## Escenario 13 — La política del proyecto se respeta

**Valida**: FR-017.

```bash
# fijar review_max_fix_rounds = 1 en los settings del proyecto
/tmp/mem-028 review --diff
```

**Esperado**: la salida imprime `max_fix_rounds: 1`, y la revisión persiste ese valor.
Cambiar la política después **no** altera la revisión ya iniciada. Sin política
configurada, el valor es `2` (el defecto del dominio), no un número escrito a mano en
el código.

---

## Escenario 14 — Rendimiento con 1.000 hallazgos

**Valida**: SC-008.

Genera una revisión con 1.000 hallazgos de consenso y mide:

```bash
time /tmp/mem-028 review status "$REVIEW_ID"
time /tmp/mem-028 review show "$REVIEW_ID" > /dev/null
```

**Esperado**: ambas por debajo de 2 s en un entorno local.

---

## Escenario 15 — Cierre real: la revisión original y una nueva

**Valida**: SC-009, SC-010, FR-028, FR-029, FR-030. Este escenario es el objetivo de la
funcionalidad, no una prueba sintética.

1. Sobre `acr_96710834-8273-49f3-bd11-42764b2f11d4`, registra el delta de corrección
   que cierra sus hallazgos confirmados con `review_fix_record`.
2. Obtén dos re-juicios independientes con `review_rejudge` (revisor A y revisor B).
3. Finaliza la revisión.
4. Inicia una revisión adversarial **nueva** sobre el target corregido y ejecútala
   completa con dos revisores independientes.

**Esperado**:
- La revisión original conserva **todos** sus hallazgos —ninguno borrado— con los
  confirmados en `RESOLVED`.
- La revisión nueva finaliza `APPROVED`, sin defectos severos confirmados ni
  contradicciones severas.

Si la revisión nueva confirma un defecto severo, la funcionalidad **no** está
terminada: eso es exactamente lo que este escenario existe para detectar.

---

## Validación integral

**Valida**: SC-011.

```bash
gofumpt -l .
golangci-lint run
go test ./...
go vet ./...
```

**Esperado**: sin fallos atribuibles a esta funcionalidad. Los tests que se hayan
modificado por exigir el comportamiento defectuoso deben estar listados en `tasks.md`
con su justificación; ningún otro test existente cambia.
