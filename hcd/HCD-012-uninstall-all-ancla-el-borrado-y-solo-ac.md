# uninstall --all ancla el borrado y solo aconseja sobre gomemory

ID: HCD-012
Tipo: fix
Carril: fast
Motivo: Fix con reproducción del ACR y sin decisiones abiertas

## Intención
El borrado de un binario no puede seguir un cambio de ruta fuera de HOME, y el aviso de retirada manual solo aparece si el ejecutable se identifica como gomemory

## Reproducción
ACR de `a7f2184`: un candidato dentro de `HOME` sustituye su directorio por un enlace externo al recibir `version`, y `uninstall --all --yes --no-scan` borra un centinela fuera de `HOME`. Un ejecutable ajeno llamado `mem` fuera de `HOME` recibe el aviso de borrarlo a mano. Las pruebas `TestAnclaBinario_` y `TestPlanBinaries_Ajeno` lo fijan.

## Alcance
Incluido: anclar el borrado del binario al directorio comprobado, sin seguir cambios de ruta; emitir el aviso de retirada manual solo si el ejecutable se identifica como gomemory.
Excluido: dejar de ejecutar `mem version` sobre candidatos del `PATH`, que es la forma de identificarlos; el comportamiento en Windows del binario en uso.

## ¿Qué pasa si no lo hacemos?
Un cambio de ruta durante la desinstalación puede borrar un archivo fuera de `HOME`, y el aviso puede inducir a borrar software ajeno.

## Alternativa más barata
Revalidar la ruta justo antes de borrar reduce la ventana, pero no elimina la carrera. `os.Root` la cierra con pocas líneas.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → `go test ./adapters/primary/cli/ -run 'TestAnclaBinario_|TestPlanBinaries_' -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-10-31 23:32 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestAnclaBinario_|TestPlanBinaries_' -count=1` (salida 1)
      --- FAIL: TestAnclaBinario_CambioDeRutaNoSaleDeHome (0.00s)
          uninstall_binary_anchor_unix_test.go:42: el borrado siguió el enlace fuera de HOME: stat /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestAnclaBinario_CambioDeRutaNoSaleDeHome2585283348/002/mem: no such file or directory
      --- FAIL: TestPlanBinaries_AjenoSinAviso (0.00s)
          uninstall_binary_anchor_unix_test.go:74: "soy otra herramienta": aviso=true, salida="  ℹ️  /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestPlanBinaries_AjenoSinAviso1902663218/002/mem está fuera de HOME y se conserva: bórralo a mano si quieres retirarlo\n"
      FAIL
      FAIL	mem/adapters/primary/cli	0.403s
      FAIL
- 2026-10-31 23:34 · tarea 1 · verde · caducada · `go test ./adapters/primary/cli/ -run 'TestAnclaBinario_|TestPlanBinaries_' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.375s
- 2026-10-31 23:42 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli/ -run 'TestAnclaBinario_|TestPlanBinaries_' -count=1` (salida 1)
      --- FAIL: TestAnclaBinario_DirectorioMovidoFueraNoSeBorra (0.00s)
          uninstall_binary_anchor_unix_test.go:61: se borró un archivo ya fuera de HOME: stat /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestAnclaBinario_DirectorioMovidoFueraNoSeBorra1861592415/002/bin/mem: no such file or directory
      FAIL
      FAIL	mem/adapters/primary/cli	0.503s
      FAIL
- 2026-10-31 23:49 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli/ -run 'TestAnclaBinario_|TestPlanBinaries_' -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.361s

## Siguiente
Cerrada con aprobación sobre la revisión 18ee65293bfd.
