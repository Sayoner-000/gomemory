# Retirar helper adrProviderEspia y comentarios obsoletos tras HCD-069

ID: HCD-071
Tipo: technical
Carril: fast
Motivo: Higiene sin cambio de comportamiento, hallazgos reproducidos y sin decisiones abiertas

## Intención
Que el código no nombre símbolos ni claves retirados en HCD-069: borrar el helper de prueba huérfano adrProviderEspia y reescribir los seis comentarios obsoletos (H-01 y H-02 del informe ACR+ADM 2026-11-28)


## Alcance
Incluido: H-01, borrar `adrProviderEspia` y sus tres métodos (`adapters/secondary/persistence/memory_seed_test.go:233-247`), incluido el comentario que nombra a `ports.ADRSyncProvider`, y el import `context` si queda sin uso. H-02, reescribir o quitar los seis comentarios que nombran símbolos o claves retirados en HCD-069: `infrastructure/container.go:193` (`adrSyncProvider`), `adapters/primary/cli/seeds.go:37` (`ADRSyncProvider`), `adapters/secondary/persistence/settings.go:52` (`CodeImpactAnnotationDisabled`) y `:135` (`AdrSyncEnabled`), `application/ports/settings_repository.go:118` (`AdrSyncEnabled`) y `adapters/secondary/persistence/settings_test.go:525` (`adr_sync_enabled` como comparación). Cada comentario reescrito debe explicar el porqué con lo vigente (Constitución §16: los comentarios explican el porqué).
Excluido: cualquier cambio de código ejecutable o de comportamiento. La tabla `adr_sync_records` y sus índices en `adapters/secondary/persistence/db.go:439-454` se quedan, por HCD-069 Q1 (esquema solo aditivo, Constitución §25.1). `domain/impact_annotation.go` (`StripImpactAnnotation`) se queda: limpia la nota de las memorias antiguas y la usa `application/usecases/save_gate.go`. La prueba de claves retiradas de `adapters/secondary/persistence/settings_test.go:121-143` se queda: nombra `adr_sync_enabled` y `code_impact_annotation_disabled` a propósito. También quedan fuera los documentos históricos (`specs/`, `hcd/`, `CHANGELOG`, `docs/audits/`).

## ¿Qué pasa si no lo hacemos?
Quien lea el código, sea una persona o un agente, sigue encontrando referencias a una capacidad retirada. Un helper dice implementar un puerto que ya no existe, y hay comentarios que remiten a símbolos sin definición. Eso cuesta tiempo de navegación y lleva a pensar que ADR sync sigue vivo. Una auditoría futura volvería a reportar lo mismo, y el informe ACR+ADM 2026-11-28 lo seguiría marcando como deuda abierta.

## Alternativa más barata
Dejarlo como deuda documentada en el informe. Se descarta porque el arreglo es más barato que mantener la deuda. Solo hay que borrar líneas de comentario y un tipo de prueba sin llamadores, sin ningún riesgo de comportamiento, mientras que conservarlo obliga a cada auditoría a volver a explicarlo.

## Criterios de cierre
- La búsqueda de `adrProviderEspia`, `ADRSyncProvider`, `adrSyncProvider`, `CodeImpactAnnotationDisabled` y `AdrSyncEnabled` en `*.go` devuelve 0 coincidencias.
- `adr_sync_enabled` solo aparece en la prueba de claves retiradas de `adapters/secondary/persistence/settings_test.go`.
- `go vet ./...` y `go test ./adapters/secondary/persistence ./adapters/primary/cli ./infrastructure ./application/ports` pasan.
- El diff solo borra el helper de prueba y modifica líneas de comentario.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. Borrar el helper `adrProviderEspia` de `adapters/secondary/persistence/memory_seed_test.go` → ! git grep -q adrProviderEspia -- '*.go' && go vet ./adapters/secondary/persistence
- [x] 2. Reescribir el comentario de `infrastructure/container.go:193` sin `adrSyncProvider` → ! git grep -q adrSyncProvider -- infrastructure/container.go && go vet ./infrastructure
- [x] 3. Reescribir el comentario de `adapters/primary/cli/seeds.go:37` sin `ADRSyncProvider` → ! git grep -q ADRSyncProvider -- adapters/primary/cli/seeds.go && go vet ./adapters/primary/cli
- [x] 4. Reescribir el comentario de `adapters/secondary/persistence/settings.go:52` sin `CodeImpactAnnotationDisabled` → ! git grep -q CodeImpactAnnotationDisabled -- adapters/secondary/persistence/settings.go
- [x] 5. Reescribir el comentario de `adapters/secondary/persistence/settings.go:135` sin `AdrSyncEnabled` → ! git grep -q AdrSyncEnabled -- adapters/secondary/persistence/settings.go
- [x] 6. Reescribir el comentario de `application/ports/settings_repository.go:118` sin `AdrSyncEnabled` → ! git grep -q AdrSyncEnabled -- application/ports/settings_repository.go && go vet ./application/ports
- [x] 7. Reescribir el comentario de `adapters/secondary/persistence/settings_test.go:525` sin la comparación con `adr_sync_enabled` → ! git grep -q 'como adr_sync_enabled' -- adapters/secondary/persistence/settings_test.go
- [x] 8. Regresión completa sin referencias residuales → ! git grep -q -E 'adrProviderEspia|ADRSyncProvider|adrSyncProvider|CodeImpactAnnotationDisabled|AdrSyncEnabled' -- '*.go' && go vet ./... && go test ./... -count=1

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-28 07:23 -05:00 · tarea 1 · rojo observado · caducada · `! git grep -q adrProviderEspia -- '*.go' && go vet ./adapters/secondary/persistence` (salida 1)
- 2026-11-28 07:23 -05:00 · tarea 1 · rojo observado · caducada · `! git grep -q adrProviderEspia -- '*.go' && go vet ./adapters/secondary/persistence` (salida 1)
- 2026-11-28 07:24 -05:00 · tarea 1 · verde · caducada · `! git grep -q adrProviderEspia -- '*.go' && go vet ./adapters/secondary/persistence` (salida 0)
- 2026-11-28 07:24 -05:00 · tarea 2 · rojo observado · caducada · `! git grep -q adrSyncProvider -- infrastructure/container.go && go vet ./infrastructure` (salida 1)
- 2026-11-28 07:24 -05:00 · tarea 2 · verde · caducada · `! git grep -q adrSyncProvider -- infrastructure/container.go && go vet ./infrastructure` (salida 0)
- 2026-11-28 07:24 -05:00 · tarea 3 · rojo observado · caducada · `! git grep -q ADRSyncProvider -- adapters/primary/cli/seeds.go && go vet ./adapters/primary/cli` (salida 1)
- 2026-11-28 07:24 -05:00 · tarea 3 · verde · caducada · `! git grep -q ADRSyncProvider -- adapters/primary/cli/seeds.go && go vet ./adapters/primary/cli` (salida 0)
- 2026-11-28 07:24 -05:00 · tarea 4 · rojo observado · caducada · `! git grep -q CodeImpactAnnotationDisabled -- adapters/secondary/persistence/settings.go` (salida 1)
- 2026-11-28 07:24 -05:00 · tarea 4 · verde · caducada · `! git grep -q CodeImpactAnnotationDisabled -- adapters/secondary/persistence/settings.go` (salida 0)
- 2026-11-28 07:24 -05:00 · tarea 5 · rojo observado · caducada · `! git grep -q AdrSyncEnabled -- adapters/secondary/persistence/settings.go` (salida 1)
- 2026-11-28 07:24 -05:00 · tarea 5 · verde · caducada · `! git grep -q AdrSyncEnabled -- adapters/secondary/persistence/settings.go` (salida 0)
- 2026-11-28 07:24 -05:00 · tarea 6 · rojo observado · caducada · `! git grep -q AdrSyncEnabled -- application/ports/settings_repository.go && go vet ./application/ports` (salida 1)
- 2026-11-28 07:25 -05:00 · tarea 6 · verde · caducada · `! git grep -q AdrSyncEnabled -- application/ports/settings_repository.go && go vet ./application/ports` (salida 0)
- 2026-11-28 07:25 -05:00 · tarea 7 · rojo observado · caducada · `! git grep -q 'como adr_sync_enabled' -- adapters/secondary/persistence/settings_test.go` (salida 1)
- 2026-11-28 07:25 -05:00 · tarea 7 · verde · caducada · `! git grep -q 'como adr_sync_enabled' -- adapters/secondary/persistence/settings_test.go` (salida 0)
- 2026-11-28 07:25 -05:00 · tarea 8 · verde · caducada · `! git grep -q -E 'adrProviderEspia|ADRSyncProvider|adrSyncProvider|CodeImpactAnnotationDisabled|AdrSyncEnabled' -- '*.go' && go vet ./... && go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	8.826s
      ok  	mem/adapters/secondary/hostbind	0.303s
      ok  	mem/adapters/secondary/i18n	0.151s
      ok  	mem/adapters/secondary/launcher	0.605s
      ok  	mem/adapters/secondary/persistence	39.524s
      ok  	mem/adapters/secondary/release	0.443s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.340s
      ok  	mem/adapters/secondary/speckit	0.283s
      ok  	mem/adapters/secondary/tokens	0.177s
      ok  	mem/adapters/secondary/usage	0.157s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	55.763s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.907s
      ok  	mem/domain/i18n	0.153s
      ok  	mem/infrastructure	2.565s
      ok  	mem/tests/contract	115.693s
      ok  	mem/tests/integration	70.614s
      ?   	mem/version	[no test files]
- 2026-11-28 07:27 -05:00 · tarea 1 · verde · caducada · `! git grep -q adrProviderEspia -- '*.go' && go vet ./adapters/secondary/persistence` (salida 0)
- 2026-11-28 07:27 -05:00 · tarea 2 · verde · caducada · `! git grep -q adrSyncProvider -- infrastructure/container.go && go vet ./infrastructure` (salida 0)
- 2026-11-28 07:27 -05:00 · tarea 3 · verde · caducada · `! git grep -q ADRSyncProvider -- adapters/primary/cli/seeds.go && go vet ./adapters/primary/cli` (salida 0)
- 2026-11-28 07:27 -05:00 · tarea 4 · verde · caducada · `! git grep -q CodeImpactAnnotationDisabled -- adapters/secondary/persistence/settings.go` (salida 0)
- 2026-11-28 07:27 -05:00 · tarea 5 · verde · caducada · `! git grep -q AdrSyncEnabled -- adapters/secondary/persistence/settings.go` (salida 0)
- 2026-11-28 07:27 -05:00 · tarea 6 · verde · caducada · `! git grep -q AdrSyncEnabled -- application/ports/settings_repository.go && go vet ./application/ports` (salida 0)
- 2026-11-28 07:35 -05:00 · tarea 1 · verde · vigente · `! git grep -q adrProviderEspia -- '*.go' && go vet ./adapters/secondary/persistence` (salida 0)
- 2026-11-28 07:35 -05:00 · tarea 2 · verde · vigente · `! git grep -q adrSyncProvider -- infrastructure/container.go && go vet ./infrastructure` (salida 0)
- 2026-11-28 07:35 -05:00 · tarea 3 · verde · vigente · `! git grep -q ADRSyncProvider -- adapters/primary/cli/seeds.go && go vet ./adapters/primary/cli` (salida 0)
- 2026-11-28 07:35 -05:00 · tarea 4 · verde · vigente · `! git grep -q CodeImpactAnnotationDisabled -- adapters/secondary/persistence/settings.go` (salida 0)
- 2026-11-28 07:35 -05:00 · tarea 5 · verde · vigente · `! git grep -q AdrSyncEnabled -- adapters/secondary/persistence/settings.go` (salida 0)
- 2026-11-28 07:35 -05:00 · tarea 6 · verde · vigente · `! git grep -q AdrSyncEnabled -- application/ports/settings_repository.go && go vet ./application/ports` (salida 0)
- 2026-11-28 07:35 -05:00 · tarea 7 · verde · vigente · `! git grep -q 'como adr_sync_enabled' -- adapters/secondary/persistence/settings_test.go` (salida 0)
- 2026-11-28 07:35 -05:00 · tarea 8 · verde · vigente · `! git grep -q -E 'adrProviderEspia|ADRSyncProvider|adrSyncProvider|CodeImpactAnnotationDisabled|AdrSyncEnabled' -- '*.go' && go vet ./... && go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	6.130s
      ok  	mem/adapters/secondary/hostbind	0.307s
      ok  	mem/adapters/secondary/i18n	0.151s
      ok  	mem/adapters/secondary/launcher	0.607s
      ok  	mem/adapters/secondary/persistence	29.923s
      ok  	mem/adapters/secondary/release	0.364s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.181s
      ok  	mem/adapters/secondary/speckit	0.145s
      ok  	mem/adapters/secondary/tokens	0.123s
      ok  	mem/adapters/secondary/usage	0.126s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	37.787s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.272s
      ok  	mem/domain/i18n	0.114s
      ok  	mem/infrastructure	1.031s
      ok  	mem/tests/contract	97.454s
      ok  	mem/tests/integration	55.567s
      ?   	mem/version	[no test files]

## Siguiente
Cerrada con aprobación sobre la revisión 5aa31e1fd3c2.
