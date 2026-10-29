---
name: hcd
description: Entrada de HCD: elige el carril (Vía rápida o ceremonia completa), crea o retoma la unidad y dice el siguiente paso, mediante el motor mem hcd.
---

# HCD — entrada y decisor de carril

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

## 1. Elige el carril y dilo en una línea

- **Responder sin unidad**: la persona pide información (explicar, comparar, auditar, revisar). No crees unidad ni escribas archivos.
- **Vía rápida** (`--lane fast`, por defecto para cualquier cambio ordinario): `define → tasks → implement → verify → close`, sin `approve` ni `slice`; la única aprobación es la del cierre.
- **Ceremonia completa** (`--lane full`): `define → clarify → approve → slice → tasks → implement → verify → close`. Elígela si hay **varias decisiones abiertas que se afectan entre sí**, riesgo alto (datos, seguridad, contratos públicos) o si la persona la pide. **El tamaño nunca elige el carril.**

Anuncia el carril y el motivo, por ejemplo: «Carril: Vía rápida — fix con reproducción clara, sin decisiones abiertas». Si la persona pide el otro carril, cámbialo.

## 2. Inicia o retoma

- **Sin argumentos**: `mem hcd show --json`. Si responde `not_found`, pregunta qué se quiere hacer.
- **Con id**: `mem hcd show <id> --json`.
- **Pedido nuevo**: propone tipo y título, confírmalos y ejecuta `mem hcd start --kind <fix|feature|evolution|technical> --lane <fast|full> --title "…" --why "<motivo del carril>" --intent "…" --json --client <agente>`.
  - `fix` corrige un fallo observado; `feature` añade un comportamiento; `evolution` cambia uno existente; `technical` mejora una propiedad técnica medible.
- Continúa con la skill de `next_action` (normalmente `/hcd-define`).

## 3. Promueve si la Vía rápida se queda corta

Si en una unidad `fast` aparecen decisiones interdependientes, `mem hcd promote --json`: el documento gana Historias y Slices, la evidencia se conserva y la unidad entra en la ceremonia completa.

## 4. Edición sin unidad activa

En Codex y OpenCode no hay hook que avise: antes de editar código para un cambio, comprueba que hay una unidad activa (`mem hcd show --json`). Si no la hay, empieza por el paso 1.
