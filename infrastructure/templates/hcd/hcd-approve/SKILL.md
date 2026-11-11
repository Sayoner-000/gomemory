---
name: hcd-approve
description: "Paso approve de HCD (solo ceremonia completa): la persona aprueba la revisión exacta de la definición."
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

1. Solo en el carril `full`, con `stage: approve` o con `norms_changed: true`, y nunca con faltantes ni aclaraciones abiertas. En la Vía rápida no hay este paso: se aprueba al cerrar.
2. **Revisión splash antes de aprobar.** Comprueba primero `data.definition_review.splash` en la salida del motor. Si ya está completada o decidida para la revisión actual, retómala sin repetir la oferta. Si falta decisión, Ofrece a la persona que el subagente `splash` revise el documento de la unidad (`doc_path`) antes de presentarlo: «¿Lanzo splash para revisar la definición?». Decide la persona en cada unidad.
   - Si la persona omite splash, registra su cita literal:
     `mem hcd review --kind splash --action skip --revision <definition_revision> --operation-id <id> --quote "<respuesta>" --client <agente>`
     Una vez registrada la omisión, la aprobación sigue sin la revisión en el paso 3.
   - Si la persona acepta, registra su cita literal:
     `mem hcd review --kind splash --action accept --revision <definition_revision> --operation-id <id> --quote "<respuesta>" --client <agente>`
   - Cómo se lanza: registra el intento con `mem hcd review --kind splash --action start --revision <definition_revision> --operation-id <id> --attempt <attempt_id> --client <agente>`. Lanza el subagente: en Claude Code y OpenCode, el subagente `splash`; en agy, `invoke_subagent` con `TypeName: "splash"`; en Codex, `spawn_agent(agent_type: "splash", fork_turns: "none")`. Aviso de aislamiento por instrucciones en Codex: el aislamiento de MCP es por instrucciones del prompt, no una capacidad impuesta demostrada. Pásale la ruta del documento y esta lente: rutas y nombres citados que no existen, criterios que no se pueden comprobar, supuestos sin evidencia y contradicciones entre secciones o con las aclaraciones.
   - Con el informe completado: registra el resultado con `mem hcd review --kind splash --action complete --revision <definition_revision> --operation-id <id> --input <informe.json>`. Muestra los hallazgos antes de la definición, tal como llegan (severidad, cita, problema y pregunta). Un Blocker impide pedir la cita: propón la corrección o no sigas hasta que se corrija o la persona lo descarta de forma explícita mediante `mem hcd review --kind splash --action dismiss --revision <definition_revision> --operation-id <id> --finding <id> --quote "<cita de descarte>"`. Major y Nit son informativos: propón corregirlos y deja que la persona decida. Corregir ahora no cuesta nada; después de aprobar, cambiar la definición obliga a aprobarla de nuevo y a verificar otra vez todas las tareas.
   - Si `splash` falla o no está disponible: registra `mem hcd review --kind splash --action fail` o `--action unavailable`. Si no está disponible (no existe ese subagente o el agente no puede lanzar subagentes), dilo en una línea: «splash no está disponible aquí; lo instala `mem setup`». Tras un fallo o indisponibilidad, pide a la persona una nueva decisión explícita para registrar `--action skip`; la aprobación sigue sin la revisión solo con esa omisión registrada.
3. Presenta a la persona **todo lo que se aprueba**, sección por sección: intención, reproducción (en un `fix`), alcance (incluido y excluido), «¿qué pasa si no lo hacemos?», «alternativa más barata», historias con sus criterios y aclaraciones resueltas, junto con `definition_revision` y las secciones de la Constitución y del Playbook que aplican. Si `norms_changed` es `true`, explica qué cambió en las normas y cómo afecta a la definición.
4. Pregunta: «¿Apruebas la definición <id> en la revisión <definition_revision>?».
5. Con su respuesta literal: `mem hcd approve --quote "<respuesta>" --json --client <agente>`. Se aceptan «apruebo», «aprobar» o la revisión; si el motor responde `invalid_quote`, vuelve a preguntar.
6. Si después cambia cualquier sección de la definición (intención, reproducción, alcance, «¿qué pasa si no lo hacemos?», «alternativa más barata», historias o aclaraciones), la aprobación deja de valer y el motor vuelve a este paso. Añadir slices o tareas no la invalida.
