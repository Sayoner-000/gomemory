# Pruebas unitarias de update y release sin socket de loopback (C-002)

ID: HCD-049
Tipo: fix
Carril: fast
Motivo: Fix de pruebas con reproducción clara y sin decisiones abiertas: la persona eligió el RoundTripper en memoria

## Intención
Que las pruebas unitarias de mem update y del adaptador de releases atiendan sus handlers en memoria con un http.RoundTripper, sin httptest.NewServer, para que pasen en sandboxes sin permiso de bind en loopback

## Reproducción
En macOS, con un sandbox que prohíbe el bind de red:
`sandbox-exec -p '(version 1)(allow default)(deny network-bind)(deny network-outbound (remote ip "localhost:*"))' go test ./adapters/primary/cli -run 'TestLatestReleaseTag|TestDownloadFileRejectsOversizedBody' -count=1`
y lo mismo con `./adapters/secondary/release`. Los dos entran en pánico con «httptest: failed to listen on a port: listen tcp6 [::1]:0: bind: operation not permitted». El pánico aborta el paquete entero, no solo esa prueba.

## Alcance
Incluido: `adapters/primary/cli/cmd_update_test.go` (los 5 `httptest.NewServer`: TestLatestReleaseTag, TestLatestReleaseTagErrorStatus, TestDownloadFileRejectsOversizedBody y TestLatestReleaseTagBoundsMetadata) y `adapters/secondary/release/github_test.go` (`newServer`). Las dos usan un `http.Client` cuyo `Transport` atiende el handler en memoria con `httptest.NewRecorder` y respeta la cancelación del contexto. Ninguna prueba pierde aserciones.
Excluido: el código de producción; `tests/contract` y `tests/integration`, que lanzan el binario `mem` como subproceso y le pasan la URL por `GOMEMORY_RELEASE_API_BASE`, así que necesitan un socket real.

## ¿Qué pasa si no lo hacemos?
En cualquier sandbox sin bind en loopback (harnesses de agentes, CI restringido), los paquetes `cli` y `release` fallan enteros por un pánico ajeno al código que prueban, y el fallo tapa a las demás pruebas de esos paquetes.

## Alternativa más barata
Saltar las pruebas cuando falla el bind (`t.Skip`) es más barato, pero las deja sin ejecutar justo donde fallan. El RoundTripper en memoria cuesta poco más y las pruebas corren en todas partes: es el cierre que eligió la persona.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → las pruebas de update de `adapters/primary/cli/cmd_update_test.go` y el paquete `adapters/secondary/release` pasan bajo `sandbox-exec` sin bind de red
- [x] 2. Retirar `httptest.NewServer` de `adapters/primary/cli/cmd_update_test.go` y `adapters/secondary/release/github_test.go` → `! grep -n httptest.NewServer` en los dos archivos
- [x] 3. Mantener en verde los dos paquetes fuera del sandbox → `go vet` y `go test ./adapters/primary/cli ./adapters/secondary/release -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-07 05:26 -05:00 · tarea 1 · rojo observado · caducada · `sandbox-exec -p '(version 1)(allow default)(deny network-bind)(deny network-outbound (remote ip "localhost:*"))' go test ./adapters/primary/cli -run 'TestLatestReleaseTag|TestDownloadFileRejectsOversizedBody' -count=1 && sandbox-exec -p '(version 1)(allow default)(deny network-bind)(deny network-outbound (remote ip "localhost:*"))' go test ./adapters/secondary/release -count=1` (salida 1)
      testing.tRunner.func1.2({0x1036351b8, 0x328184fe1e80})
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2125 +0x1a0
      testing.tRunner.func1()
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2128 +0x2c8
      panic({0x1036351b8?, 0x328184fe1e80?})
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/runtime/panic.go:859 +0x120
      net/http/httptest.newLocalListener()
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/net/http/httptest/server.go:226 +0xd4
      net/http/httptest.NewUnstartedServer(...)
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/net/http/httptest/server.go:284
      net/http/httptest.NewServer({0x103705688, 0x328184fe1e30})
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/net/http/httptest/server.go:267 +0x28
      mem/adapters/primary/cli.TestLatestReleaseTag(0x328184edcd88)
      	/Users/josegomezj/home/rcw/gomemory/adapters/primary/cli/cmd_update_test.go:41 +0x7c
      testing.tRunner(0x328184edcd88, 0x10370fdd0)
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2195 +0xc4
      created by testing.(*T).Run in goroutine 1
      	/opt/homebrew/Cellar/go/1.27.3/libexec/src/testing/testing.go:2260 +0x3b8
      FAIL	mem/adapters/primary/cli	0.296s
      FAIL
- 2026-11-07 05:26 -05:00 · tarea 2 · rojo observado · caducada · `! grep -n httptest.NewServer adapters/primary/cli/cmd_update_test.go adapters/secondary/release/github_test.go` (salida 1)
      adapters/primary/cli/cmd_update_test.go:41:	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      adapters/primary/cli/cmd_update_test.go:65:	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
      adapters/primary/cli/cmd_update_test.go:356:		srv := httptest.NewServer(h)
      adapters/primary/cli/cmd_update_test.go:363:	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(make([]byte, 1024)) }))
      adapters/primary/cli/cmd_update_test.go:423:	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
      adapters/secondary/release/github_test.go:18:	srv := httptest.NewServer(h)
- 2026-11-07 05:27 -05:00 · tarea 2 · verde · vigente · `! grep -n httptest.NewServer adapters/primary/cli/cmd_update_test.go adapters/secondary/release/github_test.go` (salida 0)
- 2026-11-07 05:27 -05:00 · tarea 1 · verde · vigente · `sandbox-exec -p '(version 1)(allow default)(deny network-bind)(deny network-outbound (remote ip "localhost:*"))' go test ./adapters/primary/cli -run 'TestLatestReleaseTag|TestDownloadFileRejectsOversizedBody' -count=1 && sandbox-exec -p '(version 1)(allow default)(deny network-bind)(deny network-outbound (remote ip "localhost:*"))' go test ./adapters/secondary/release -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.287s
      ok  	mem/adapters/secondary/release	0.358s
- 2026-11-07 05:27 -05:00 · tarea 3 · verde · vigente · `go vet ./adapters/primary/cli ./adapters/secondary/release && GOOS=windows go vet ./adapters/primary/cli ./adapters/secondary/release && go test ./adapters/primary/cli ./adapters/secondary/release -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	18.882s
      ok  	mem/adapters/secondary/release	0.363s

## Siguiente
Cerrada con aprobación sobre la revisión 8d234b6a24ca.
