---
name: hcd-tasks
description: Paso tasks de HCD: escribe tareas atómicas, cada una con la comprobación que la demuestra.
---

# HCD — tareas

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

1. En la sección **Checklist** de `doc_path`, una línea por tarea: `- [ ] N. verbo + objeto → comprobación que la demuestra`.
2. **Test de atomicidad**: un verbo y un objeto concretos, un único resultado inspeccionable, sin «cómo» por decidir y ejecutable de una vez. Si una tarea falla el test, divídela.
3. La comprobación es lo que ejecutará `/hcd-verify`: preferiblemente un comando (`go test ./… -run …`); si no es automatizable, di qué se observará.
4. En un `fix`, la tarea 1 ya existe: «La reproducción ya no falla → re-ejecutar la reproducción».
5. En la ceremonia completa, cita cada tarea en un slice (`Tareas:`).
