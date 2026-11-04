# Cerrar hallazgos ADM-06, ADM-07 de la auditoría v2.44.3 y admitir cliente antigravity en HCD

ID: HCD-019
Tipo: fix
Carril: fast
Motivo: Correcciones defensivas acotadas con pruebas directas y sin decisiones de diseño abiertas

## Intención
Sanear comillas en rutas de hook_agy (ADM-06), flexibilizar comprobación de versión en agy_setup (ADM-07) y admitir antigravity/gemini en hcd/execute.

## Reproducción
1. ADM-06: En `adapters/primary/cli/hook_agy.go:128-133`, argumentos con comillas como `"\"/path/file.go\""` hacen que `filepath.Ext` devuelva `".go\""` en lugar de `".go"`, provocando que `reindexTouchedGoFiles` salte los archivos tocados en `agy`.
2. ADM-07: En `adapters/primary/setup/agy_setup.go:187`, `InspectAgyEntry` compara rígidamente `version != agyCheckedVersion` (`"1.4.12"`), degradando `mem doctor` a `StateOutdated` ante parches menores compatibles (ej. 1.4.13).
3. HCD client: `mem hcd show --client antigravity` responde `cliente inválido: antigravity` al no estar registrado en `clients` en `adapters/primary/cli/hcd/execute.go:16`.

## Alcance
Incluido: limpieza de comillas en rutas y comandos (`cleanAgyArg`) en `adapters/primary/cli/hook_agy.go` con pruebas en `hook_agy_test.go`; flexibilización de compatibilidad de versiones de agy en `adapters/primary/setup/agy_setup.go` aceptando parches de la serie 1.4.x con pruebas en `agy_setup_test.go`; e inclusión de `antigravity` y `gemini` en `clients` de `adapters/primary/cli/hcd/execute.go` con prueba de comando.
Excluido: modificar el esquema de hooks de Antigravity en `hooks.json`; tocar el motor de compresión o persistencia; alterar la constitución técnica.

## ¿Qué pasa si no lo hacemos?
Los archivos `.go` editados en Antigravity no se reindexan automáticamente en el grafo de código en el `turn-end`, `mem doctor` genera falsas alarmas ante actualizaciones menores de `agy`, y `mem hcd` rechaza llamadas con `--client antigravity`.

## Alternativa más barata
Forzar reindexaciones manuales con `mem index` tras cada edición y usar `--client terminal` en llamadas manuales. Es frágil, propenso a desincronización y degrada la experiencia de usuario.

## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->

## Checklist
- [x] 1. ADM-06: rutas y comandos en transcript de agy se sanean sin comillas externas → `go test ./adapters/primary/cli -run TestAgyCleanArgQuotes -count=1`
- [x] 2. ADM-07: inspección de agy tolera versiones semánticas compatibles de la serie 1.4.x → `go test ./adapters/primary/setup -run TestInspectAgyEntry_FlexibleVersion -count=1`
- [x] 3. HCD client: mem hcd admite antigravity y gemini como clientes válidos → `go test ./adapters/primary/cli/hcd -run TestHCDClient_Antigravity -count=1`
- [x] 4. Verificación completa → `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash -n scripts/install.sh`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-04 00:15 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestAgyCleanArgQuotes -count=1` (salida 1)
      --- FAIL: TestAgyCleanArgQuotes (0.00s)
          hcd015_agy_test.go:123: files esperado [/workspace/a.go], obtenido: ["/workspace/a.go"]
      FAIL
      FAIL	mem/adapters/primary/cli	0.544s
      FAIL
- 2026-11-04 00:17 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli -run TestAgyCleanArgQuotes -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.531s
- 2026-11-04 00:18 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/setup -run TestInspectAgyEntry_FlexibleVersion -count=1` (salida 1)
      --- FAIL: TestInspectAgyEntry_FlexibleVersion (0.00s)
          hcd015_agy_setup_test.go:224: 1.4.12 debe ser StateOK, obtenido: not_applicable
      FAIL
      FAIL	mem/adapters/primary/setup	0.350s
      FAIL
- 2026-11-04 00:19 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/setup -run TestInspectAgyEntry_FlexibleVersion -count=1` (salida 1)
      --- FAIL: TestInspectAgyEntry_FlexibleVersion (0.00s)
          hcd015_agy_setup_test.go:229: 1.4.13 debe ser StateOK, obtenido: outdated (agy "1.4.13" sin comprobar; contrato observado en 1.4.12)
      FAIL
      FAIL	mem/adapters/primary/setup	0.250s
      FAIL
- 2026-11-04 00:20 · tarea 2 · verde · caducada · `go test ./adapters/primary/setup -run TestInspectAgyEntry_FlexibleVersion -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.151s
- 2026-11-04 00:20 · tarea 3 · rojo observado · caducada · `go test ./adapters/primary/cli/hcd -run TestHCDClient_Antigravity -count=1` (salida 1)
      --- FAIL: TestHCDClient_Antigravity (0.00s)
          execute_test.go:186: parse(show --client antigravity) = {command:show id: client:terminal run: flags:map[] json:false} cliente inválido: antigravity
          execute_test.go:186: parse(show --client gemini) = {command:show id: client:terminal run: flags:map[] json:false} cliente inválido: gemini
      FAIL
      FAIL	mem/adapters/primary/cli/hcd	0.344s
      FAIL
- 2026-11-04 00:20 · tarea 3 · verde · vigente · `go test ./adapters/primary/cli/hcd -run TestHCDClient_Antigravity -count=1` (salida 0)
      ok  	mem/adapters/primary/cli/hcd	0.230s
- 2026-11-04 00:20 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli -run TestAgyCleanArgQuotes -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.330s
- 2026-11-04 00:20 · tarea 2 · verde · vigente · `go test ./adapters/primary/setup -run TestInspectAgyEntry_FlexibleVersion -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.228s
- 2026-11-04 00:20 · tarea 4 · verde · vigente · `go build ./... && go vet ./... && go test ./... -count=1 && bash scripts/test-install-uninstall.sh && bash -n scripts/install.sh` (salida 0)
      ok  	mem/adapters/secondary/expandstate	0.224s
      ok  	mem/adapters/secondary/generator	1.466s
      ok  	mem/adapters/secondary/hcdfiles	0.812s
      ok  	mem/adapters/secondary/launcher	0.844s
      ok  	mem/adapters/secondary/persistence	31.835s
      ok  	mem/adapters/secondary/release	0.380s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.230s
      ok  	mem/adapters/secondary/speckit	0.160s
      ok  	mem/adapters/secondary/tokens	0.130s
      ok  	mem/adapters/secondary/usage	0.119s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	12.740s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.216s
      ok  	mem/infrastructure	0.387s
      ok  	mem/tests/contract	90.012s
      ok  	mem/tests/integration	39.078s
      ?   	mem/version	[no test files]
      OK: --uninstall solo retira binarios propios

## Siguiente
Cerrada con aprobación sobre la revisión ecb38c8adbbc.
