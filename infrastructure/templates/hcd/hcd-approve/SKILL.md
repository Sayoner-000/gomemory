---
name: hcd-approve
description: Paso approve de HCD (solo ceremonia completa): la persona aprueba la revisión exacta de la definición.
---

# HCD — aprobar la definición

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

1. Solo en el carril `full`, con `stage: approve` o con `norms_changed: true`. En la Vía rápida no hay este paso: se aprueba al cerrar.
2. Presenta a la persona la definición: intención, alcance, historias con sus criterios y aclaraciones resueltas, junto con `definition_revision` y las secciones de la Constitución y del Playbook que aplican. Si `norms_changed` es `true`, explica qué cambió en las normas y cómo afecta a la definición.
3. Pregunta: «¿Apruebas la definición <id> en la revisión <definition_revision>?».
4. Con su respuesta literal: `mem hcd approve --quote "<respuesta>" --json --client <agente>`. Se aceptan «apruebo», «aprobar» o la revisión; si el motor responde `invalid_quote`, vuelve a preguntar.
5. Si después cambia la definición (intención, alcance, historias o aclaraciones), la aprobación deja de valer y el motor vuelve a este paso. Añadir slices o tareas no la invalida.
