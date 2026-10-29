# Contrato: skills HCD (v2)

**Cubre**: FR-001, FR-024, FR-025. **v1**: `git show a0503b0:specs/045-hcd-protocol/contracts/skills.md`.

## 1. Catálogo

| Skill | Claude Code / OpenCode | Codex | Paso | Comandos del motor |
|---|---|---|---|---|
| `hcd` | `/hcd <pedido o id>` | `$hcd …` | Entrada, decisor de carril, retomar, promover | `start`, `show`, `promote` |
| `hcd-define` | `/hcd-define` | `$hcd-define` | define | `show` |
| `hcd-clarify` | `/hcd-clarify` | `$hcd-clarify` | clarify | `clarify` |
| `hcd-approve` | `/hcd-approve` | `$hcd-approve` | approve (solo `full`) | `approve` |
| `hcd-slice` | `/hcd-slice` | `$hcd-slice` | slice (solo `full`) | `show`, `analyze` |
| `hcd-tasks` | `/hcd-tasks` | `$hcd-tasks` | tasks | `show` |
| `hcd-implement` | `/hcd-implement` | `$hcd-implement` | implement | `verify --expect-fail` |
| `hcd-verify` | `/hcd-verify` | `$hcd-verify` | verify | `verify` |
| `hcd-analyze` | `/hcd-analyze` | `$hcd-analyze` | antes de implementar y de cerrar | `analyze` |
| `hcd-close` | `/hcd-close` | `$hcd-close` | close | `close`, `cancel` |

Se retiran `hcd-exception`, `hcd-playbook` y `hcd-migrate`. El Playbook sigue como documento fijado (`mem playbook`).

## 2. Reglas comunes (texto obligatorio en cada `SKILL.md`)

1. **Motor primero.** Al empezar, ejecuta `mem hcd show --json --client <agente>` (o `./mem` si `mem` no está en el PATH). El paso (`stage`) y la siguiente skill (`next_action`) salen del motor, nunca de la memoria de la conversación.
2. **Sin motor no hay avance.** Si el motor no existe o responde `internal`, informa la limitación y detente. No edites `hcd/` para simular un estado.
3. **Las mutaciones van por el motor.** Redactas el documento de la unidad (`doc_path`), pero las aclaraciones, aprobaciones, evidencias y cierres solo se registran con `mem hcd clarify`, `approve`, `verify`, `close` o `cancel`. No marques casillas ni escribas en la sección Evidencia a mano.
4. **Preguntar antes que suponer.** Ante una ambigüedad que cambie comportamiento, contrato, datos, seguridad o alcance, abre una aclaración con `mem hcd clarify --ask` y pregunta con 2 a 4 opciones y una recomendación justificada. No elijas la interpretación más probable.
5. **Aprobación citada.** Para aprobar o cerrar, muestra la revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal de la persona en `--quote`. Nunca redactes tú la cita.
6. **La casilla no es la prueba.** Una tarea solo está hecha cuando `mem hcd verify` registró su comprobación vigente. Si cambias código después, la evidencia caduca y hay que volver a verificar.
7. **Siguiente paso.** Termina cada respuesta con el `next_action` del motor: la skill siguiente y por qué.
8. **Privacidad.** No pongas secretos en el documento, las evidencias ni las citas.
9. **La Constitución y el Playbook mandan.** Antes de definir, aprobar, implementar y cerrar, lee `mem constitution` y `mem playbook` y cita las secciones que aplican. Si `norms_changed` es `true`, las normas cambiaron desde la aprobación: revisa la definición con las reglas nuevas y vuelve a aprobarla con `/hcd-approve`.

## 3. Instalación

`mem install` y `mem mcp-setup` escriben las 10 skills en los agentes presentes y los 10 comandos de OpenCode. No hay retirada de skills v1 instaladas: ninguna release las publicó y no había ninguna en la máquina de desarrollo (comprobado el 2026-10-29).

## 4. Aviso pre-edición

En Claude Code, el hook `PreToolUse` (`mem hook expand pre-tool`) avisa una vez por sesión, sin bloquear, cuando se edita un archivo de código sin unidad HCD activa. En Codex y OpenCode la regla viaja en la skill `hcd` (§4).
