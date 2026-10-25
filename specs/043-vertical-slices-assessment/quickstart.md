# Quickstart: ejecutar y validar la evaluación

**Feature**: 043-vertical-slices-assessment

Guía de ejecución y validación. El método detallado está en [research.md](research.md); los criterios, en [contracts/evaluation-criteria.md](contracts/evaluation-criteria.md); la forma del informe, en [contracts/report-contract.md](contracts/report-contract.md).

## Requisitos previos

- Go 1.27 en PATH (`go version`).
- Repositorio en el `head` que se va a evaluar.
- Directorio de salida **fuera** del repositorio (`$OUT`), por ejemplo el scratchpad de la sesión.

## 1. Línea base (C6)

```sh
git rev-parse HEAD            > "$OUT/baseline-head.txt"
git status --porcelain        > "$OUT/baseline-status.txt"
```

Resultado esperado: dos archivos en `$OUT`; el repositorio no cambia.

## 2. Grafo de importaciones de producción (C1, L1–L3)

```sh
go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... > "$OUT/imports.txt"
```

Comprobaciones rápidas:

```sh
grep '^mem/domain ' "$OUT/imports.txt" | tr ' ' '\n' | grep '^mem/'           # L1: solo debe aparecer mem/domain
grep '^mem/application/' "$OUT/imports.txt" | grep -E 'mem/(adapters|infrastructure)'   # L2: sin salida
```

## 3. Testabilidad (T1–T2)

```sh
go vet ./...
go test ./... -count=1 > "$OUT/test.txt" 2>&1; echo "exit=$?"
```

Resultado esperado: `exit=0`. Si falla por red o caché, se registra y los criterios afectados se marcan como no verificados.

## 4. Recorridos y matriz

Para cada slice de research R2, seguir la entrada hasta persistencia (CodeGraph para localizar, el código para citar) y rellenar la matriz criterio × slice.

## 5. Validación de cierre (C6)

```sh
diff <(git rev-parse HEAD) "$OUT/baseline-head.txt" && \
diff <(git status --porcelain) "$OUT/baseline-status.txt" && echo "solo lectura: OK"
```

Resultado esperado: `solo lectura: OK`.

## Validación del informe

El informe es válido si cumple las reglas de [contracts/report-contract.md](contracts/report-contract.md) y si:

- tiene al menos 3 flujos y entre ellos aparecen los cuatro tipos de entrada (SC-002);
- cada hallazgo tiene criterio, evidencia, consecuencia y cierre (SC-003);
- elige exactamente una opción (SC-005);
- el paso 5 imprime `solo lectura: OK` (SC-007).
