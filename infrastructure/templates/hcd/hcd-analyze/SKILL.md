---
name: hcd-analyze
description: Analiza la coherencia de una unidad HCD (trazabilidad, aprobación, aclaraciones y evidencia) sin escribir nada.
---

# HCD — analizar

## Reglas comunes

1. **Motor primero.** Al empezar, ejecuta `mem hcd show --json --client <agente>` (o `./mem` si `mem` no está en el PATH). El paso (`stage`) y la siguiente skill (`next_action`) salen del motor, nunca de la memoria de la conversación.
2. **Sin motor no hay avance.** Si el motor no existe o responde `internal`, informa la limitación y detente. No edites `hcd/` para simular un estado.
3. **Las mutaciones van por el motor.** Redactas el documento de la unidad (`doc_path`), pero las aclaraciones, aprobaciones, evidencias y cierres solo se registran con `mem hcd clarify`, `approve`, `verify`, `close` o `cancel`. No marques casillas ni escribas en la sección Evidencia a mano.
4. **Preguntar antes que suponer.** Ante una ambigüedad que cambie comportamiento, contrato, datos, seguridad o alcance, abre una aclaración con `mem hcd clarify --ask` y pregunta con 2 a 4 opciones y una recomendación justificada. No elijas la interpretación más probable.
5. **Aprobación citada.** Para aprobar o cerrar, muestra la revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal de la persona en `--quote`. Nunca redactes tú la cita.
6. **La casilla no es la prueba.** Una tarea solo está hecha cuando `mem hcd verify` registró su comprobación vigente. Si cambias código después, la evidencia caduca y hay que volver a verificar.
7. **Siguiente paso.** Termina cada respuesta con el `next_action` del motor: la skill siguiente y por qué.
8. **Privacidad.** No pongas secretos en el documento, las evidencias ni las citas.

1. Ejecuta `mem hcd analyze --json --client <agente>`. Es una consulta: no cambia estado, documento ni evidencia.
2. Presenta los `findings` agrupados por severidad:
   - **CRITICAL** bloquea el cierre: faltantes, aclaraciones abiertas, aprobación ausente o caducada, y evidencia pendiente cuando ya toca verificar o cerrar.
   - **HIGH**: trazabilidad rota (historia sin criterio, criterio que ningún slice cubre, tarea fuera de todo slice, referencia inexistente).
   - **MEDIUM**: evidencia manual, proyecto sin git, tareas aún sin evidencia durante la implementación.
3. Para cada hallazgo, propón la corrección concreta y la skill que la hace (`/hcd-define`, `/hcd-slice`, `/hcd-tasks`, `/hcd-verify`…). No lo corrijas sin decírselo a la persona.
4. Úsala antes de implementar y antes de cerrar.
