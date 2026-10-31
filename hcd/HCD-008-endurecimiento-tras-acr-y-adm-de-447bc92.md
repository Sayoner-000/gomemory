# Endurecimiento tras ACR y ADM de 447bc92

ID: HCD-008
Tipo: fix
Carril: fast
Motivo: Correcciones acotadas e independientes, con reproducción y sin decisiones interdependientes

## Intención
Cerrar C-001, C-002, C-008, C-009 y H-02 a H-05: truncado UTF-8 válido, timestamps corruptos rechazados, borrado del índice junto a la memoria, chmod de export comprobado, escapes de terminal saneados, escritura global atómica y sin código muerto

## Reproducción
Cada defecto tiene una prueba roja: `TestTruncateUTF8`, `TestSetSessionLastPrompt_CortaEnRuna`, `TestPlanTitle_CortaEnRuna` (bytes inválidos; `last_prompt` terminó en `0xC3` en el binario real), `TestHCDStore_TimestampCorruptoFalla`, `TestDeleteMemory_NoDejaIndice`, `TestRelation_ReasoningSinEscapes` (H-03 reproducido por la ADM), `TestExportMode_FalloDeChmod` y `TestGlobalInstructions_Atomico`.

## Alcance
Incluido: C-001, C-002 y los dos sitios más de la familia (`cmd_compare.go`, `goparse.go`); C-008; C-009; H-02; H-03 al leer relaciones en persistencia; H-04 reutilizando `WriteConfigFile` y `GlobalConfigPath`; H-05 eliminando `GenerateConfig`.
Excluido: C-007 (cerrado sin código); recomendaciones de CI de la ADM; rotación del token de la memoria 1114, externa al repositorio.

## ¿Qué pasa si no lo hacemos?
Persisten bytes inválidos en memorias y exports, un export puede quedar legible por otros informando éxito, una relación almacenada puede inyectar escapes en la terminal, un corte deja a medias el CLAUDE.md global y lo olvidado sigue en el índice de búsqueda.

## Alternativa más barata
Solo H-02 y H-03: dos cambios de pocas líneas, pero dejaría abiertos defectos ya verificados cuya corrección también es de pocas líneas.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → `go test ./domain/ ./adapters/secondary/persistence/ ./adapters/primary/cli/ ./adapters/primary/setup/ -run 'TestTruncateUTF8|TestSetSessionLastPrompt_CortaEnRuna|TestPlanTitle_CortaEnRuna|TestHCDStore_TimestampCorruptoFalla|TestDeleteMemory_NoDejaIndice|TestRelation_ReasoningSinEscapes|TestExportMode_FalloDeChmod|TestGlobalInstructions_Atomico' -count=1`
- [x] 2. `GenerateConfig` eliminado sin romper la compilación (dep: 1) → `go build ./... && ! grep -rn 'GenerateConfig(' --include='*.go' .`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-31 15:12 · tarea 1 · rojo observado · caducada · `go test ./domain/ ./adapters/secondary/persistence/ ./adapters/primary/cli/ ./adapters/primary/setup/ -run 'TestTruncateUTF8|TestSetSessionLastPrompt_CortaEnRuna|TestPlanTitle_CortaEnRuna|TestHCDStore_TimestampCorruptoFalla|TestDeleteMemory_NoDejaIndice|TestRelation_ReasoningSinEscapes|TestExportMode_FalloDeChmod|TestGlobalInstructions_Atomico' -count=1` (salida 1)
      --- FAIL: TestDeleteMemory_NoDejaIndice (0.01s)
          hardening_test.go:89: DeleteMemory debía fallar si no puede limpiar el índice
      --- FAIL: TestRelation_ReasoningSinEscapes (0.01s)
          hardening_test.go:138: pair conserva controles: "razon \x1b]0;pwned\a \x1b[2J fin"
          hardening_test.go:138: list conserva controles: "razon \x1b]0;pwned\a \x1b[2J fin"
          hardening_test.go:138: all conserva controles: "razon \x1b]0;pwned\a \x1b[2J fin"
          hardening_test.go:138: get conserva controles: "razon \x1b]0;pwned\a \x1b[2J fin"
      FAIL
      FAIL	mem/adapters/secondary/persistence	0.171s
      --- FAIL: TestPlanTitle_CortaEnRuna (0.00s)
          hardening_test.go:14: título inválido: "Plan aprobado: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\xc3…"
      --- FAIL: TestExportMode_FalloDeChmod (0.00s)
          hardening_test.go:22: err = <nil>
      FAIL
      FAIL	mem/adapters/primary/cli	0.275s
      --- FAIL: TestGlobalInstructions_Atomico (0.00s)
          global_atomic_test.go:27: debía fallar sin temporal
      FAIL
      FAIL	mem/adapters/primary/setup	0.173s
      FAIL
- 2026-10-31 15:15 · tarea 1 · verde · vigente · `go test ./domain/ ./adapters/secondary/persistence/ ./adapters/primary/cli/ ./adapters/primary/setup/ -run 'TestTruncateUTF8|TestSetSessionLastPrompt_CortaEnRuna|TestPlanTitle_CortaEnRuna|TestHCDStore_TimestampCorruptoFalla|TestDeleteMemory_NoDejaIndice|TestRelation_ReasoningSinEscapes|TestExportMode_FalloDeChmod|TestGlobalInstructions_Atomico' -count=1` (salida 0)
      ok  	mem/domain	0.119s
      ok  	mem/adapters/secondary/persistence	0.174s
      ok  	mem/adapters/primary/cli	0.230s
      ok  	mem/adapters/primary/setup	0.124s
- 2026-10-31 15:15 · tarea 2 · verde · vigente · `go build ./... && ! grep -rn 'GenerateConfig(' --include='*.go' .` (salida 0)

## Siguiente
Cerrada con aprobación sobre la revisión 0ae73bb8ffb2.
