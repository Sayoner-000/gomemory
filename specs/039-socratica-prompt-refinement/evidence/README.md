# Evidencia de Socratica (039)

## Registro por agente

Cada registro debe incluir fecha, sistema operativo, versión literal del binario,
orden ejecutada, salida literal depurada, resultado esperado y veredicto
(PASS, FAIL o no ejecutado, con causa). No registrar credenciales ni contenido
privado. Una prueba simulada no acredita una capacidad del agente real.

## Referencias fijas de SC-005

Ejecutar los cinco prompts en cada agente, sin aportar los datos faltantes.
Los datos esenciales deben quedar como marcadores explícitos; no inventarlos.

| Prompt | Datos ausentes |
|---|---|
| Envía el informe al cliente antes del cierre. | [informe], [cliente], [fecha de cierre], [canal] |
| Reduce la latencia del servicio hasta el objetivo acordado. | [servicio], [métrica de latencia], [objetivo] |
| Prepara una oferta para el producto con el precio aprobado. | [producto], [precio aprobado], [destinatario] |
| Publica la nueva versión cuando tengamos permiso. | [versión], [destino], [autorización] |
| Migra los datos al sistema nuevo y valida que esté terminado. | [origen], [destino], [criterios de validación] |

## Validación de la habilidad (T069)

`command -v skills-ref` no devuelve nada: **no ejecutado: skills-ref no instalado**. No se afirma validación externa; el contenido y el frontmatter los fijan `TestSocraticaSkill_*` y `TestSkillSourcesInSync`.

## Regresión final (T070) — 2026-10-16

| Orden | Resultado |
|---|---|
| `go vet ./...` | código 0, sin salida |
| `go test ./... -count=1` | código 0, 20 paquetes `ok`, ningún `FAIL` |
| `bash scripts/coverage.sh` | `Cobertura global combinada: 80.4%` (umbral 80 %, línea base 80,4 % de 037: no baja) |
| `node --test infrastructure/plugin/opencode/gomemory.test.mjs` | 46/46 |
| `go test -bench 'Socratica\|UserPromptSubmit' -run '^$' -benchtime=200x` | `BenchmarkSocraticaApprove` 203 340 ns/op (objetivo < 50 ms); `BenchmarkUserPromptSubmit_Off` 40 205 ns/op y `_On` 66 392 ns/op (objetivo < 20 ms, coste de Socratica dentro del hook) |

Sin regresiones en MCP, sesiones, distribución de habilidades, hooks ni Octopus (SC-009). Detalle de los defectos encontrados y cerrados por esta regresión en `mvp.md`.

## Estado

Aceptación real por agente (T060 agentes, T061–T063) pendiente: requiere sesiones interactivas con la persona. Hasta entonces la matriz no se completa (T064) y ninguna capacidad pasa a `true` (T065).
