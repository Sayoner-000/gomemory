# mem update y el aviso de versión resuelven latest sin la API anónima de GitHub

ID: HCD-072
Tipo: fix
Carril: fast
Motivo: Fallo con causa raíz conocida y arreglo ya probado en install.sh; una sola decisión abierta (ETag) que se resuelve con clarify

## Intención
Que mem update y el aviso de versión obtengan la última release por la redirección de github.com/<repo>/releases/latest, con la API como respaldo autenticado con GITHUB_TOKEN, para que un 403 por límite anónimo en una IP compartida no impida actualizar

## Reproducción
`mem update` sin `--version` llama a `latestReleaseTag` (`adapters/primary/cli/cmd_update.go:314`). Esa función consulta `https://api.github.com/repos/Sayoner-000/gomemory/releases/latest` sin autenticar y, si la API responde 403, termina con `fail("no se pudo consultar la última versión: …")`. El aviso de versión (`adapters/secondary/release/github.go:46`, `GitHub.Latest`, que se lanza desde `spawnUpdateCheck`) usa la misma API anónima. Con un 403 registra el fallo y conserva la última versión conocida, así que el aviso deja de actualizarse sin que nadie se entere. El límite anónimo es de 60 peticiones por hora por IP, compartido por todos los usuarios de una NAT o un runner de CI. Es la misma causa raíz que rompió el CI en el run 45994905354 (memoria 2295), que `scripts/install.sh` ya resolvió en `cf19e0b`.

Prueba de reproducción: un test en `adapters/secondary/release/github_test.go` en el que la API responde 403 y `<downloadBase>/latest` redirige a `<downloadBase>/tag/v9.9.9`. Debe obtener `v9.9.9` de `GitHub.Latest`. Hoy falla con `API de releases: HTTP 403`.

## Alcance
Incluido: (1) `release.GitHub.Latest` resuelve primero la redirección de `<downloadBase>/latest`, sin seguirla, y acepta solo un `Location` que termine en `/tag/vX.Y.Z` (sin prerelease: `releases/latest` nunca apunta a una), con el mismo criterio que `latest_tag_from_redirect` de `scripts/install.sh`. Si no obtiene un tag válido, recurre a la API y envía `Authorization: Bearer $GITHUB_TOKEN` cuando la variable existe. El ETag sigue Q1: solo se guarda cuando la versión llega por la API de respaldo; si sale de la redirección se guarda vacío y basta la caché de 24 h. (2) `mem update` usa `ReleasePort.Latest` del `Deps` (`releasePortOf`) y se borra `latestReleaseTag`, para que la última versión se consulte en un solo sitio. (3) Pruebas del adaptador: la redirección resuelve el tag; una redirección inválida (sin `/tag/`, con un tag mal formado o un 200 sin `Location`) pasa a la API; la API recibe el token cuando existe; el error de un 403 sin token dice que se puede definir `GITHUB_TOKEN`. Se ajustan `TestLatestReleaseTag*` de `adapters/primary/cli/cmd_update_test.go` al puerto. Las pruebas que solo apuntaban la API a un servidor falso (`tests/contract/hook_session_start_notice_test.go` y `TestUpdateCheckDoesNotMutate` en `tests/integration/update_integration_test.go`) apuntan también la base de descargas a ese servidor. Si no, la redirección saldría a github.com y recibirían la versión publicada en lugar de la de prueba. (4) La variable `GITHUB_TOKEN` se documenta en `docs/MANUAL.md`, junto al resto de la configuración por entorno (FR-031 de `specs/034-console-lifecycle/spec.md` pide documentar ahí las variables).
Excluido: `scripts/install.sh` y el instalador de Windows, ya corregidos o no afectados, porque Windows descarga por la URL releases/latest/download. La descarga de assets y del archivo checksums.txt, que ya van por la URL de descargas de releases y no tienen límite de API. La política de 24 h del aviso (FR-028) y `DecideUpdateNotice`, que no cambian. El instalador npm (`installer/src/bootstrap.ts`), que queda para otra unidad si tiene el mismo problema.

## ¿Qué pasa si no lo hacemos?
Un usuario detrás de una IP compartida (NAT corporativo, VPN, runner de CI) que ejecute `mem update` puede encontrarse con «GitHub API respondió 403: API rate limit exceeded» y quedarse sin poder actualizar, salvo que pase `--version` a mano. El aviso de versión nueva deja de enterarse de las releases sin dar ninguna señal. El aviso consulta como mucho una vez cada 24 h, así que gasta poco del límite, pero en una IP compartida el límite lo agotan otros.

## Alternativa más barata
Añadir solo `GITHUB_TOKEN` a la llamada actual de la API. Se descarta porque casi ningún usuario tiene la variable definida, y sin ella el 403 sigue igual. La redirección web no tiene límite de API y ya se probó en `scripts/install.sh`. Otra opción sería arreglar solo `latestReleaseTag` y dejar el adaptador. Se descarta porque mantendría dos implementaciones de la misma consulta y el aviso seguiría expuesto.

## Criterios de cierre
- Con la API en 403 y una redirección válida, `GitHub.Latest` y `mem update --check` obtienen el tag.
- `latestReleaseTag` ya no existe y `mem update` consulta la última versión solo a través de `ReleasePort`.
- `go vet ./...` y `go test ./...` pasan.


## Aclaraciones
- [x] Q1. Con la redirección no hay ETag del servidor. ¿Qué hace el aviso de versión con el validador de caché que pide FR-028? (1) Guardarlo solo cuando la respuesta viene de la API de respaldo; si la versión sale de la redirección, se guarda vacío y vale la caché de 24 h. (2) Usar siempre la API cuando hay un ETag guardado. (3) Quitar el ETag por completo. → opción 1: guardar el ETag solo cuando la respuesta viene de la API de respaldo; si la versión sale de la redirección se guarda vacío y vale la caché de 24 h

## Checklist
- [x] 1. La reproducción ya no falla: con la API en 403, `GitHub.Latest` obtiene el tag de la redirección de `<downloadBase>/latest` → go test ./adapters/secondary/release -run '^TestLatest_RedireccionSinAPI$' -count=1
- [x] 2. Hacer que una redirección inválida (sin `/tag/`, con un tag mal formado o un 200 sin `Location`) pase a la API (sin rojo: el respaldo entró con la tarea 1; esta prueba lo fija) → go test ./adapters/secondary/release -run '^TestLatest_RedireccionInvalidaUsaAPI$' -count=1
- [x] 3. Hacer que la API de respaldo envíe `Authorization: Bearer` cuando `GITHUB_TOKEN` existe → go test ./adapters/secondary/release -run '^TestLatest_APIConToken$' -count=1
- [x] 4. Hacer que el error de un 403 sin token sugiera definir `GITHUB_TOKEN` → go test ./adapters/secondary/release -run '^TestLatest_403SugiereToken$' -count=1
- [x] 5. Devolver el ETag vacío cuando la versión sale de la redirección (Q1) (sin rojo: la tarea 1 ya devuelve el ETag vacío por esa vía; esta prueba lo fija) → go test ./adapters/secondary/release -run '^TestLatest_RedireccionSinETag$' -count=1
- [x] 6. Hacer que `mem update` consulte la última versión con `ReleasePort.Latest` y borrar `latestReleaseTag` → ! git grep -q latestReleaseTag -- '*.go' && go test ./adapters/primary/cli -run '^TestCmdUpdateCheck_UsaReleasePort$' -count=1
- [x] 7. Documentar `GITHUB_TOKEN` en `docs/MANUAL.md` junto a la configuración por entorno → grep -q 'GITHUB_TOKEN' docs/MANUAL.md
- [x] 8. Regresión completa → go vet ./... && go test ./... -count=1

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-28 08:01 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionSinAPI$' -count=1` (salida 1)
      --- FAIL: TestLatest_RedireccionSinAPI (0.00s)
          github_test.go:80: tag="" nm=false err=API de releases: HTTP 403
      FAIL
      FAIL	mem/adapters/secondary/release	0.341s
      FAIL
- 2026-11-28 08:02 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionSinAPI$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.157s
- 2026-11-28 08:02 -05:00 · tarea 2 · rojo NO observado (el comando pasó) · caducada · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionInvalidaUsaAPI$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.164s
- 2026-11-28 08:03 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionInvalidaUsaAPI$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.159s
- 2026-11-28 08:03 -05:00 · tarea 3 · rojo observado · caducada · `go test ./adapters/secondary/release -run '^TestLatest_APIConToken$' -count=1` (salida 1)
      --- FAIL: TestLatest_APIConToken (0.00s)
          github_test.go:128: tag="" err=API de releases: HTTP 403
      FAIL
      FAIL	mem/adapters/secondary/release	0.154s
      FAIL
- 2026-11-28 08:03 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/secondary/release -run '^TestLatest_APIConToken$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.164s
- 2026-11-28 08:03 -05:00 · tarea 4 · rojo observado · caducada · `go test ./adapters/secondary/release -run '^TestLatest_403SugiereToken$' -count=1` (salida 1)
      --- FAIL: TestLatest_403SugiereToken (0.00s)
          github_test.go:139: err = API de releases: HTTP 403, quiero que sugiera GITHUB_TOKEN
      FAIL
      FAIL	mem/adapters/secondary/release	0.163s
      FAIL
- 2026-11-28 08:03 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/secondary/release -run '^TestLatest_403SugiereToken$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.166s
- 2026-11-28 08:03 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionSinETag$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.161s
- 2026-11-28 08:04 -05:00 · tarea 6 · rojo observado · caducada · `! git grep -q latestReleaseTag -- '*.go' && go test ./adapters/primary/cli -run '^TestCmdUpdateCheck_UsaReleasePort$' -count=1` (salida 1)
- 2026-11-28 08:05 -05:00 · tarea 6 · verde · caducada · `! git grep -q latestReleaseTag -- '*.go' && go test ./adapters/primary/cli -run '^TestCmdUpdateCheck_UsaReleasePort$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.417s
- 2026-11-28 08:06 -05:00 · tarea 7 · verde · caducada · `grep -q 'GITHUB_TOKEN' docs/MANUAL.md` (salida 0)
- 2026-11-28 08:08 -05:00 · tarea 8 · verde · caducada · `go vet ./... && go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	5.648s
      ok  	mem/adapters/secondary/hostbind	0.280s
      ok  	mem/adapters/secondary/i18n	0.125s
      ok  	mem/adapters/secondary/launcher	0.542s
      ok  	mem/adapters/secondary/persistence	29.175s
      ok  	mem/adapters/secondary/release	0.568s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.160s
      ok  	mem/adapters/secondary/speckit	0.123s
      ok  	mem/adapters/secondary/tokens	0.120s
      ok  	mem/adapters/secondary/usage	0.130s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	26.637s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.229s
      ok  	mem/domain/i18n	0.133s
      ok  	mem/infrastructure	0.807s
      ok  	mem/tests/contract	84.199s
      ok  	mem/tests/integration	40.543s
      ?   	mem/version	[no test files]
- 2026-11-28 08:11 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionSinAPI$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.165s
- 2026-11-28 08:11 -05:00 · tarea 2 · verde · caducada · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionInvalidaUsaAPI$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.124s
- 2026-11-28 08:11 -05:00 · tarea 3 · verde · caducada · `go test ./adapters/secondary/release -run '^TestLatest_APIConToken$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.137s
- 2026-11-28 08:11 -05:00 · tarea 4 · verde · caducada · `go test ./adapters/secondary/release -run '^TestLatest_403SugiereToken$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.137s
- 2026-11-28 08:11 -05:00 · tarea 5 · verde · caducada · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionSinETag$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.130s
- 2026-11-28 08:11 -05:00 · tarea 6 · verde · caducada · `! git grep -q latestReleaseTag -- '*.go' && go test ./adapters/primary/cli -run '^TestCmdUpdateCheck_UsaReleasePort$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.293s
- 2026-11-28 08:11 -05:00 · tarea 7 · verde · caducada · `grep -q 'GITHUB_TOKEN' docs/MANUAL.md` (salida 0)
- 2026-11-28 08:11 -05:00 · tarea 2 · rojo observado · caducada · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionInvalidaUsaAPI$' -count=1` (salida 1)
      --- FAIL: TestLatest_RedireccionInvalidaUsaAPI (0.00s)
          --- FAIL: TestLatest_RedireccionInvalidaUsaAPI/sin_/tag/ (0.00s)
              github_test.go:106: tag="" etag="" err=<nil>
          --- FAIL: TestLatest_RedireccionInvalidaUsaAPI/tag_mal_formado (0.00s)
              github_test.go:106: tag="latest" etag="" err=<nil>
          --- FAIL: TestLatest_RedireccionInvalidaUsaAPI/200_sin_Location (0.00s)
              github_test.go:106: tag="" etag="" err=<nil>
      FAIL
      FAIL	mem/adapters/secondary/release	0.201s
      FAIL
- 2026-11-28 08:12 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionSinAPI$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.152s
- 2026-11-28 08:12 -05:00 · tarea 2 · verde · vigente · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionInvalidaUsaAPI$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.137s
- 2026-11-28 08:12 -05:00 · tarea 3 · verde · vigente · `go test ./adapters/secondary/release -run '^TestLatest_APIConToken$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.128s
- 2026-11-28 08:12 -05:00 · tarea 4 · verde · vigente · `go test ./adapters/secondary/release -run '^TestLatest_403SugiereToken$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.120s
- 2026-11-28 08:12 -05:00 · tarea 5 · verde · vigente · `go test ./adapters/secondary/release -run '^TestLatest_RedireccionSinETag$' -count=1` (salida 0)
      ok  	mem/adapters/secondary/release	0.138s
- 2026-11-28 08:12 -05:00 · tarea 6 · verde · vigente · `! git grep -q latestReleaseTag -- '*.go' && go test ./adapters/primary/cli -run '^TestCmdUpdateCheck_UsaReleasePort$' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.270s
- 2026-11-28 08:12 -05:00 · tarea 7 · verde · vigente · `grep -q 'GITHUB_TOKEN' docs/MANUAL.md` (salida 0)
- 2026-11-28 08:12 -05:00 · tarea 8 · verde · vigente · `go vet ./... && go test ./... -count=1` (salida 0)
      ok  	mem/adapters/secondary/hcdfiles	5.769s
      ok  	mem/adapters/secondary/hostbind	0.235s
      ok  	mem/adapters/secondary/i18n	0.123s
      ok  	mem/adapters/secondary/launcher	0.546s
      ok  	mem/adapters/secondary/persistence	27.707s
      ok  	mem/adapters/secondary/release	0.539s
      ?   	mem/adapters/secondary/requestcompression	[no test files]
      ok  	mem/adapters/secondary/rulesfs	0.138s
      ok  	mem/adapters/secondary/speckit	0.099s
      ok  	mem/adapters/secondary/tokens	0.106s
      ok  	mem/adapters/secondary/usage	0.089s
      ?   	mem/application/ports	[no test files]
      ok  	mem/application/usecases	27.293s
      ?   	mem/assets	[no test files]
      ok  	mem/domain	0.208s
      ok  	mem/domain/i18n	0.085s
      ok  	mem/infrastructure	0.841s
      ok  	mem/tests/contract	82.496s
      ok  	mem/tests/integration	40.075s
      ?   	mem/version	[no test files]

## Siguiente
Cerrada con aprobación sobre la revisión 88b5ce9ef376.
