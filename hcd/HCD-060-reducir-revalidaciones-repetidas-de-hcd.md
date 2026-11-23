# Reducir revalidaciones repetidas de HCD

ID: HCD-060
Tipo: technical
Carril: fast
Motivo: Ajuste conservador de instrucciones sin cambiar gates

## Intención
Evitar ejecutar dos veces las comprobaciones y diferir revalidación acumulada hasta el cierre


## Alcance
Incluido: instrucciones canónicas implement/verify, contrato automatizado y comparación explícita del flujo.
Excluido: cambiar vigencia, gates, esquema, cachear resultados o atribuir ahorro de tokens sin medición.

## ¿Qué pasa si no lo hacemos?
El agente puede repetir comprobaciones anteriores tras cada edición y ejecutar el mismo test fuera y dentro del motor.

## Alternativa más barata
Ajustar las instrucciones sin cambiar el motor. Constitución §10 y §21; Playbook HCD-EVIDENCE-01 y HCD-TDD-01 conservados.


## Aclaraciones
<!-- - [ ] Q1. pregunta abierta   ·   - [x] Q1. pregunta → respuesta -->


## Checklist
- [x] 1. Evitar revalidaciones acumuladas durante implementación → go test ./adapters/primary/setup -run TestHCDVerificationScheduling -count=1
- [x] 2. Validar regresión del motor y comparar flujo (sin rojo: regresión y análisis) → go test ./domain ./application/usecases ./adapters/primary/setup -run 'HCD|EvidenceGoesStale' -count=1

## Evidencia
<!-- La escribe `mem hcd verify`. Lo escrito a mano aquí no cuenta. -->
- 2026-11-23 18:47 -05:00 · tarea 1 · rojo observado · caducada · `go test ./adapters/primary/setup -run TestHCDVerificationScheduling -count=1` (salida 1)
      --- FAIL: TestHCDVerificationScheduling (0.00s)
          hcd_verification_scheduling_test.go:14: hcd-implement: falta "No ejecutes la misma comprobación fuera y dentro del motor"
          hcd_verification_scheduling_test.go:14: hcd-implement: falta "No revalides tareas anteriores solo por caducidad"
          hcd_verification_scheduling_test.go:14: hcd-implement: falta "Antes del cierre"
          hcd_verification_scheduling_test.go:14: hcd-verify: falta "No ejecutes la misma comprobación fuera y dentro del motor"
          hcd_verification_scheduling_test.go:14: hcd-verify: falta "No revalides tareas anteriores solo por caducidad"
          hcd_verification_scheduling_test.go:14: hcd-verify: falta "Antes del cierre"
      FAIL
      FAIL	mem/adapters/primary/setup	0.924s
      FAIL
- 2026-11-23 18:47 -05:00 · tarea 1 · verde · caducada · `go test ./adapters/primary/setup -run TestHCDVerificationScheduling -count=1` (salida 0)
      ok  	mem/adapters/primary/setup	0.218s
- 2026-11-23 18:47 -05:00 · tarea 2 · verde · caducada · `go test ./domain ./application/usecases ./adapters/primary/setup -run 'HCD|EvidenceGoesStale' -count=1` (salida 0)
      ok  	mem/domain	0.211s
      ok  	mem/application/usecases	18.366s
      ok  	mem/adapters/primary/setup	0.252s
- 2026-11-23 18:48 -05:00 · tarea 1,2 · verde · vigente · `go test ./domain ./application/usecases ./adapters/primary/setup -run 'HCD|EvidenceGoesStale' -count=1 -v` (salida 0)
          --- PASS: TestInstallHCDSkillsRefusesSymlinkDestinations/.config/opencode/commands/hcd.md (0.00s)
          --- PASS: TestInstallHCDSkillsRefusesSymlinkDestinations/.codex/skills/hcd/SKILL.md (0.00s)
      === RUN   TestHCD057InstalledSkills
      --- PASS: TestHCD057InstalledSkills (0.02s)
      === RUN   TestHCD057SocraticSkills
      === RUN   TestHCD057SocraticSkills/hcd-define_connects_socratic_exam
      === RUN   TestHCD057SocraticSkills/hcd_connects_socratic_exam_and_plan_mode_deferral
      --- PASS: TestHCD057SocraticSkills (0.00s)
          --- PASS: TestHCD057SocraticSkills/hcd-define_connects_socratic_exam (0.00s)
          --- PASS: TestHCD057SocraticSkills/hcd_connects_socratic_exam_and_plan_mode_deferral (0.00s)
      === RUN   TestHCD057SplashSkill
      --- PASS: TestHCD057SplashSkill (0.00s)
      === RUN   TestHCDSkillsShareRulesAndCoverMVPCommands
      --- PASS: TestHCDSkillsShareRulesAndCoverMVPCommands (0.00s)
      === RUN   TestHCDSkillsApproveOfreceSplash
      --- PASS: TestHCDSkillsApproveOfreceSplash (0.00s)
      === RUN   TestHCDVerificationScheduling
      --- PASS: TestHCDVerificationScheduling (0.00s)
      PASS
      ok  	mem/adapters/primary/setup	0.251s

## Siguiente
Cerrada con aprobación sobre la revisión 16e158e4002e.
