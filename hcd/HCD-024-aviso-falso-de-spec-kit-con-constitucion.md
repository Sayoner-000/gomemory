# Aviso falso de Spec Kit con constitución personalizada y sincronizada

ID: HCD-024
Tipo: fix
Carril: fast
Motivo: Fallo con reproducción clara en la máquina de la persona y sin decisiones abiertas

## Intención
Que mem install, mem update y mem seed comparen .specify/memory/constitution.md con la constitución vigente del proyecto, no con la plantilla del binario, y solo avisen cuando de verdad estén desincronizadas

## Reproducción
1. En este repositorio, con gomemory 2.45.1, la constitución en memoria está personalizada (755 líneas, incluye §25 «Excepciones del proyecto») y `.specify/memory/constitution.md` es idéntico byte a byte a ella. Aun así, `mem update --yes` imprime `○ .specify/memory/constitution.md tiene ediciones del equipo y no se modificó; para reflejar la constitución vigente: mem constitution --sync`, y ese comando no cambia nada. Salida real del 2026-11-04.
2. Causa: `CmdSeed` (`adapters/primary/cli/seeds.go:96`) llama a `refreshSpeckitConstitution` con `embeddedTemplate(doc.Template)`, es decir, con la plantilla del binario y no con la constitución vigente. Si la copia de Spec Kit es una plantilla anterior intacta, la reescribe con la plantilla, aunque la constitución del proyecto esté personalizada.

## Alcance
Incluido: `CmdSeed` pasa a `refreshSpeckitConstitution` la constitución vigente del proyecto (la de la memoria o, si no existe, la plantilla), resuelta con `usecases.ResolvePinnedDoc`; el mensaje de actualización dice «sincronizada con la constitución vigente»; prueba con una constitución personalizada sincronizada (sin aviso) y con una copia de plantilla anterior (se reescribe con la vigente, no con la plantilla).
Excluido: el aviso de `printSeedReport` sobre la plantilla nueva disponible para una constitución personalizada, que es correcto; cambiar `mem constitution --sync`; tocar otros documentos fijados.

## ¿Qué pasa si no lo hacemos?
Cada `mem install` y `mem update` muestra un aviso falso que recomienda un comando inútil, y quien lo sigue no ve ningún cambio. Además, una copia de Spec Kit con una plantilla anterior se reescribiría con la plantilla del binario y perdería las secciones propias del proyecto.

## Alternativa más barata
Ignorar el aviso. No cubre la reescritura con la plantilla, que sí pierde contenido.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → `go test ./adapters/primary/cli -run TestSpeckitConstitution_UsaLaVigente -count=1`
- [x] 2. Verificación completa → `go build ./... && go vet ./... && go test ./adapters/primary/... -count=1`

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-04 06:04 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/cli -run TestSpeckitConstitution_UsaLaVigente -count=1` (salida 1)
      # mem/adapters/primary/cli [mem/adapters/primary/cli.test]
      adapters/primary/cli/hcd024_speckit_vigente_test.go:30:13: undefined: speckitConstitutionSource
      adapters/primary/cli/hcd024_speckit_vigente_test.go:50:12: undefined: speckitConstitutionSource
      FAIL	mem/adapters/primary/cli [build failed]
      FAIL
- 2026-11-04 06:04 -05:00 · tarea 1 · verde · vigente · `go test ./adapters/primary/cli -run TestSpeckitConstitution_UsaLaVigente -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	0.299s
- 2026-11-04 06:04 -05:00 · tarea 2 · verde · vigente · `go build ./... && go vet ./... && go test ./adapters/primary/... -count=1` (salida 0)
      ok  	mem/adapters/primary/cli	17.313s
      ok  	mem/adapters/primary/cli/clikit	0.189s
      ok  	mem/adapters/primary/cli/hcd	2.062s
      ok  	mem/adapters/primary/cli/review	1.433s
      ok  	mem/adapters/primary/cli/socratica	1.559s
      ok  	mem/adapters/primary/console	0.289s
      ok  	mem/adapters/primary/setup	2.183s
      ok  	mem/adapters/primary/tui	8.992s

## Siguiente
Cerrada con aprobación sobre la revisión c991bb7a6f7e.
