# agy recibe rutas absolutas en skills.json

ID: HCD-016
Tipo: fix
Carril: fast
Motivo: agy 1.4.12 descarta las rutas con ~ de skills.json y mem install --agents agy escribe ~/.agents/skills.

## Intención
mem install --agents agy escribe rutas absolutas en skills.json y repara las entradas con ~.

## Reproducción
`go test ./adapters/primary/setup -run '^TestHCD016' -count=1`. Hoy falla: `InstallAgyGlobal` escribe `~/.agents/skills` y conserva otras entradas con `~`. En agy 1.4.12, el log muestra `must be an absolute path` y esas entradas no se cargan.

## Alcance
Incluido: instalar las skills de gomemory en `~/.gemini/config/skills`, la carpeta global documentada de agy, y convertir a absolutas las entradas `~/` de `skills.json`, sin duplicados.
Excluido: publicar una versión y cambiar las entradas de `skills.json` que no empiecen por `~/`.

## ¿Qué pasa si no lo hacemos?
Cada `mem install --agents agy` deja una entrada que agy descarta sin avisar en la sesión, y las entradas `~` existentes siguen sin cargar.

## Alternativa más barata
Corregir `skills.json` a mano en cada máquina. Sirve una vez, pero la siguiente instalación vuelve a añadir la entrada rota.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. La reproducción ya no falla → `go test ./adapters/primary/setup -run '^TestHCD016' -count=1`
- [x] 2. Instalar las skills en `~/.gemini/config/skills` → `go test ./adapters/primary/setup -run '^TestHCD016' -count=1`, con destino que no depende de Claude y retirada al desinstalar; comprobar además en agy real que aparecen.

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-03 00:32 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/setup -run '^TestHCD016' -count=1` (salida 1)
      --- FAIL: TestHCD016InstallWritesAbsoluteSkillPaths (0.00s)
          hcd015_agy_setup_test.go:145: rutas de skills: [~/.agents/skills]
      --- FAIL: TestHCD016InstallRepairsTildeSkillPaths (0.00s)
          hcd015_agy_setup_test.go:164: rutas de skills:
               got ~/.claude/skills|~/.agents/skills|/var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestHCD016InstallRepairsTildeSkillPaths3307763496/001/.agents/skills|/opt/skills
              want /var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestHCD016InstallRepairsTildeSkillPaths3307763496/001/.claude/skills|/var/folders/34/jwxjynms09b21_4rb72hqxk80000gn/T/TestHCD016InstallRepairsTildeSkillPaths3307763496/001/.agents/skills|/opt/skills
      FAIL
      FAIL	mem/adapters/primary/setup	0.478s
      FAIL
- 2026-11-03 00:32 · tarea 1 · verde · caducada · `go test ./adapters/primary/setup -run '^TestHCD016' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.187s
- 2026-11-03 00:35 · tarea 2 · rojo observado · caducada · `go test ./adapters/primary/setup -run '^TestHCD016' -count=1` (salida 1)
      --- FAIL: TestHCD016InstallDoesNotAddSkillsEntry (0.00s)
          hcd015_agy_setup_test.go:146: la carpeta global de agy no necesita skills.json: <nil>
      --- FAIL: TestHCD016AgySkillTargetDoesNotDependOnClaude (0.00s)
          hcd015_agy_setup_test.go:193: destinos: []
      --- FAIL: TestHCD016AgySkillsAreManagedForUninstall (0.00s)
          hcd015_agy_setup_test.go:208: la retirada global no cubre las skills de agy
      FAIL
      FAIL	mem/adapters/primary/setup	0.439s
      FAIL
- 2026-11-03 00:38 · tarea 1,2 · verde · vigente · `go test ./adapters/primary/setup -run '^TestHCD016' -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.260s

## Siguiente
Cerrada con aprobación sobre la revisión 847b1489ae6c.
