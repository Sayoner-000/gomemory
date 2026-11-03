# pack retrieve no pierde el original si no puede renovarlo

ID: HCD-018
Tipo: fix
Carril: fast
Motivo: CI de Windows falló en TestHookContextCompression: con la base ocupada, renovar la caducidad falla y descarta el original ya leído.

## Intención
Devolver el original aunque no se pueda renovar su caducidad y dar a la recuperación un plazo suficiente.

## Reproducción
`go test ./adapters/secondary/persistence -run '^TestHCD018' -count=1`. Hoy falla: con otra conexión reteniendo la escritura, `Get` devuelve `renovar original: context deadline exceeded`. Reproducido también con `mem pack retrieve`: código 1 con la base ocupada, y CI de Windows falló con código 1.

## Alcance
Incluido: devolver el original si falla la renovación, ampliar el plazo de recuperación por CLI y MCP, y mostrar stderr en `TestHookContextCompression`.
Excluido: cambiar `StoreTimeout` de los hooks y publicar una versión.

## ¿Qué pasa si no lo hacemos?
CI de Windows sigue fallando de forma intermitente, y un agente puede perder la recuperación de un original existente mientras otro proceso escribe en la base.

## Alternativa más barata
Reintentar el job o relajar la prueba. No corrige la pérdida de la recuperación y ocultaría un defecto real.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → `go test ./adapters/secondary/persistence -run '^TestHCD018' -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-03 01:34 · tarea 1 · rojo observado · caducada · `go test ./adapters/secondary/persistence -run '^TestHCD018' -count=1` (salida 1)
      --- FAIL: TestHCD018GetReturnsOriginalWhenRenewalFails (5.08s)
          hcd018_retrieve_test.go:33: Get: "" {Ref: Project: ContentType: Compressor: RawBytes:0 CreatedAt:0001-01-01 00:00:00 +0000 UTC LastAccessAt:0001-01-01 00:00:00 +0000 UTC ExpiresAt:0001-01-01 00:00:00 +0000 UTC} false renovar original: context deadline exceeded
      FAIL
      FAIL	mem/adapters/secondary/persistence	5.339s
      FAIL
- 2026-11-03 01:35 · tarea 1 · verde · caducada · `go test ./adapters/secondary/persistence -run '^TestHCD018' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	5.295s
- 2026-11-03 01:48 · tarea 1 · verde · vigente · `go test ./adapters/secondary/persistence -run '^TestHCD018' -count=1` (salida 0)
      ok  	mem/adapters/secondary/persistence	5.523s

## Siguiente
Cerrada con aprobación sobre la revisión 7dd3a3ae51e4.
