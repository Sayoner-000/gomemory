---
name: hcd-close
description: Paso close de HCD: cierra o cancela la unidad con la aprobación citada de la persona.
---

# HCD — cerrar

## Reglas comunes

1. **Motor primero.** Al empezar, ejecuta `mem hcd show --json --client <agente>` (o `./mem` si `mem` no está en el PATH). El paso (`stage`) y la siguiente skill (`next_action`) salen del motor, nunca de la memoria de la conversación.
2. **Sin motor no hay avance.** Si el motor no existe o responde `internal`, informa la limitación y detente. No edites `hcd/` para simular un estado.
3. **Las mutaciones van por el motor.** Redactas el documento de la unidad (`doc_path`), pero las aclaraciones, aprobaciones, evidencias y cierres solo se registran con `mem hcd clarify`, `approve`, `verify`, `close` o `cancel`. No marques casillas ni escribas en la sección Evidencia a mano.
4. **Preguntar antes que suponer.** Ante una ambigüedad que cambie comportamiento, contrato, datos, seguridad o alcance, abre una aclaración con `mem hcd clarify --ask` y pregunta con 2 a 4 opciones y una recomendación justificada. No elijas la interpretación más probable.
5. **Aprobación citada.** Para aprobar o cerrar, muestra la revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal de la persona en `--quote`. Nunca redactes tú la cita.
6. **La casilla no es la prueba.** Una tarea solo está hecha cuando `mem hcd verify` registró su comprobación vigente. Si cambias código después, la evidencia caduca y hay que volver a verificar.
7. **Siguiente paso.** Termina cada respuesta con el `next_action` del motor: la skill siguiente y por qué.
8. **Privacidad.** No pongas secretos en el documento, las evidencias ni las citas.

1. Solo con `stage: close`. Ejecuta antes `/hcd-analyze`: con algún hallazgo `CRITICAL` el motor rechaza el cierre.
2. Presenta a la persona qué se cierra:
   - unidad, `doc_path` y `work_revision` (12 hexadecimales);
   - cada tarea con su evidencia, incluido el rojo y el verde de un `fix`;
   - las evidencias manuales y los `warnings`, como límites de lo que se demostró.
3. Pregunta: «¿Apruebas el cierre de <id> sobre la revisión <work_revision>?».
4. Con su respuesta literal: `mem hcd close --quote "<respuesta>" --json --client <agente>`. Si responde `invalid_quote`, vuelve a preguntar.
5. **Cancelar**: solo si la persona lo pide, `mem hcd cancel --quote "<su motivo>" --json --client <agente>`.

El documento en `hcd/` viaja en el commit o el PR del cambio.
