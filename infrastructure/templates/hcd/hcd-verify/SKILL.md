---
name: hcd-verify
description: Paso verify de HCD: registra la evidencia de cada tarea ejecutando su comprobación con el motor.
---

# HCD — verificar

## Reglas comunes

1. **Motor primero.** Al empezar, ejecuta `mem hcd show --json --client <agente>` (o `./mem` si `mem` no está en el PATH). El paso (`stage`) y la siguiente skill (`next_action`) salen del motor, nunca de la memoria de la conversación.
2. **Sin motor no hay avance.** Si el motor no existe o responde `internal`, informa la limitación y detente. No edites `hcd/` para simular un estado.
3. **Las mutaciones van por el motor.** Redactas el documento de la unidad (`doc_path`), pero las aclaraciones, aprobaciones, evidencias y cierres solo se registran con `mem hcd clarify`, `approve`, `verify`, `close` o `cancel`. No marques casillas ni escribas en la sección Evidencia a mano.
4. **Preguntar antes que suponer.** Ante una ambigüedad que cambie comportamiento, contrato, datos, seguridad o alcance, abre una aclaración con `mem hcd clarify --ask` y pregunta con 2 a 4 opciones y una recomendación justificada. No elijas la interpretación más probable.
5. **Aprobación citada.** Para aprobar o cerrar, muestra la revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal de la persona en `--quote`. Nunca redactes tú la cita.
6. **La casilla no es la prueba.** Una tarea solo está hecha cuando `mem hcd verify` registró su comprobación vigente. Si cambias código después, la evidencia caduca y hay que volver a verificar.
7. **Siguiente paso.** Termina cada respuesta con el `next_action` del motor: la skill siguiente y por qué.
8. **Privacidad.** No pongas secretos en el documento, las evidencias ni las citas.
9. **La Constitución y el Playbook mandan.** Antes de definir, aprobar, implementar y cerrar, lee `mem constitution` y `mem playbook` y cita las secciones que aplican. Si `norms_changed` es `true`, las normas cambiaron desde la aprobación: revisa la definición con las reglas nuevas y vuelve a aprobarla con `/hcd-approve`.

1. Elige una tarea con estado `pending`, `unverified` o `failed`.
2. Ejecuta su comprobación a través del motor:
   - **Verde**: `mem hcd verify --task N --json --client <agente> -- '<comando>'`
   - **Rojo** (antes del cambio): añade `--expect-fail`.
   - **Manual** (no automatizable): `mem hcd verify --task N --manual '<qué observaste>' --json`. Queda marcada como manual.
   - Una evidencia puede cubrir varias tareas: `--task 2,3`.
3. `--json` y `--client` van **antes** de `--`: todo lo que sigue es el comando.
4. Lee `data.evidence`: `passed`, `red_observed`, `exit_code` y `excerpt`. Si falló, corrige y vuelve a verificar. No reinterpretes un fallo como éxito.
5. Cuando el paso sea `close`, sigue con `/hcd-analyze` y `/hcd-close`.
