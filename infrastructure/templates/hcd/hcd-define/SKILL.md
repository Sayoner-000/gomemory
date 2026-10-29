---
name: hcd-define
description: Paso define de HCD: redacta el documento de la unidad hasta que el motor no liste faltantes.
---

# HCD — definir

## Reglas comunes

1. **Motor primero.** Al empezar, ejecuta `mem hcd show --json --client <agente>` (o `./mem` si `mem` no está en el PATH). El paso (`stage`) y la siguiente skill (`next_action`) salen del motor, nunca de la memoria de la conversación.
2. **Sin motor no hay avance.** Si el motor no existe o responde `internal`, informa la limitación y detente. No edites `hcd/` para simular un estado.
3. **Las mutaciones van por el motor.** Redactas el documento de la unidad (`doc_path`), pero las aclaraciones, aprobaciones, evidencias y cierres solo se registran con `mem hcd clarify`, `approve`, `verify`, `close` o `cancel`. No marques casillas ni escribas en la sección Evidencia a mano.
4. **Preguntar antes que suponer.** Ante una ambigüedad que cambie comportamiento, contrato, datos, seguridad o alcance, abre una aclaración con `mem hcd clarify --ask` y pregunta con 2 a 4 opciones y una recomendación justificada. No elijas la interpretación más probable.
5. **Aprobación citada.** Para aprobar o cerrar, muestra la revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal de la persona en `--quote`. Nunca redactes tú la cita.
6. **La casilla no es la prueba.** Una tarea solo está hecha cuando `mem hcd verify` registró su comprobación vigente. Si cambias código después, la evidencia caduca y hay que volver a verificar.
7. **Siguiente paso.** Termina cada respuesta con el `next_action` del motor: la skill siguiente y por qué.
8. **Privacidad.** No pongas secretos en el documento, las evidencias ni las citas.

1. Abre `doc_path` y completa cada `pending_checks`:
   - **Intención** y **Alcance** (`Incluido:` y `Excluido:`).
   - **¿Qué pasa si no lo hacemos?** y **Alternativa más barata**: si la respuesta honesta es «nada» o hay una alternativa más barata, dilo a la persona antes de seguir.
   - **Reproducción** (solo `fix`): el comando o los pasos que hoy fallan.
   - **Historias** (solo carril `full`): `### US1. título`, una línea «Como…, quiero…, para…» y criterios `- C-01: Dado…, cuando…, entonces…`. Cada historia necesita al menos un criterio observable.
2. Lo que no sepas no lo inventes: escribe `[NEEDS CLARIFICATION: pregunta]` en su lugar y el motor te llevará a `/hcd-clarify`.
3. Vuelve a ejecutar `mem hcd show --json` hasta que el paso deje de ser `define`. El motor solo comprueba que el contenido esté presente; que sea correcto lo decides con la persona.
