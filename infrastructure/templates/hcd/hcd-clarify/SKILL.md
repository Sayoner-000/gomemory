---
name: hcd-clarify
description: Paso clarify de HCD: resuelve con la persona las aclaraciones abiertas, una pregunta a la vez.
---

# HCD — aclarar

## Reglas comunes

1. **Motor primero.** Al empezar, ejecuta `mem hcd show --json --client <agente>` (o `./mem` si `mem` no está en el PATH). El paso (`stage`) y la siguiente skill (`next_action`) salen del motor, nunca de la memoria de la conversación.
2. **Sin motor no hay avance.** Si el motor no existe o responde `internal`, informa la limitación y detente. No edites `hcd/` para simular un estado.
3. **Las mutaciones van por el motor.** Redactas el documento de la unidad (`doc_path`), pero las aclaraciones, aprobaciones, evidencias y cierres solo se registran con `mem hcd clarify`, `approve`, `verify`, `close` o `cancel`. No marques casillas ni escribas en la sección Evidencia a mano.
4. **Preguntar antes que suponer.** Ante una ambigüedad que cambie comportamiento, contrato, datos, seguridad o alcance, abre una aclaración con `mem hcd clarify --ask` y pregunta con 2 a 4 opciones y una recomendación justificada. No elijas la interpretación más probable.
5. **Aprobación citada.** Para aprobar o cerrar, muestra la revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal de la persona en `--quote`. Nunca redactes tú la cita.
6. **La casilla no es la prueba.** Una tarea solo está hecha cuando `mem hcd verify` registró su comprobación vigente. Si cambias código después, la evidencia caduca y hay que volver a verificar.
7. **Siguiente paso.** Termina cada respuesta con el `next_action` del motor: la skill siguiente y por qué.
8. **Privacidad.** No pongas secretos en el documento, las evidencias ni las citas.

1. Lista las aclaraciones abiertas del documento: líneas `- [ ] Qn.` y marcadores `[NEEDS CLARIFICATION: …]`.
2. Para un marcador sin pregunta registrada, regístrala: `mem hcd clarify --ask "pregunta" --json --client <agente>`. Responde con el `clarification` que devuelve (`Q1`, `Q2`…).
3. Haz **una pregunta a la vez**, con 2 a 4 opciones y tu recomendación justificada. Espera la respuesta de la persona.
4. Registra su respuesta: `mem hcd clarify --id Qn --answer "<respuesta de la persona>" --json --client <agente>`. El motor la escribe en el documento y la guarda como decisión.
5. Si la respuesta resolvía un marcador, sustituye el marcador en su sección por el contenido acordado.
6. Repite hasta que el paso deje de ser `clarify`.
