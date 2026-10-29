---
name: hcd-implement
description: Conduce el paso hcd-implement del protocolo HCD mediante el motor mem hcd.
---

# HCD — implementación

## Reglas comunes

1. **Motor primero.** Al empezar, la skill ejecuta `mem hcd resume <unit> --json --client <agente>` (o `./mem` si `mem` no está en PATH) y actúa solo dentro de `allowed_actions`.
2. **Sin motor no hay avance.** Si ni `mem` ni `./mem` existen, o si el motor responde `internal`, la skill informa la limitación y se detiene. No edita artefactos para simular un estado.
3. **Las mutaciones van por el motor.** La skill redacta el contenido de `definition.md`, `delivery.md` o la evidencia y lo pasa al motor por stdin con `--file -` (por ejemplo, `mem hcd define HCD-001 --file - <<'EOF'`), registrándolo con `define`, `slice`, `tasks` o `report`. Si necesita un archivo, lo escribe fuera del repositorio. Nunca escribe estados, aprobaciones, evidencias ni `verification.md` a mano.
4. **Preguntar antes que suponer** (FR-016a). Ante una ambigüedad que cambia comportamiento, contrato, datos, seguridad o alcance, la skill abre `hcd-clarify`: hace una pregunta a la vez, con 2 a 4 opciones y una recomendación justificada, y registra la respuesta. No elige la interpretación más probable.
5. **Aprobación citada** (FR-017a). Para aprobar, la skill muestra el objeto y su revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal de la persona en `--quote`. Nunca redacta la cita ni aprueba sin una respuesta en la conversación.
6. **Detención localizada** (US8). Ante una ampliación de alcance, una excepción no aprobada, una revisión incompatible, un permiso insuficiente o un resultado incierto, la skill registra un bloqueo o una aclaración sobre la parte afectada (que genera una decisión al resolverse) y continúa solo con trabajo independiente autorizado.
7. **Siguiente paso.** Toda respuesta termina con el `next_action` del motor: nombra la skill siguiente y por qué.
8. **Privacidad.** No pone secretos en artefactos, evidencias ni citas.

## Ejecución autorizada

1. Ejecuta `check --task T`. Si responde `atomicity` o `blocked`, no ejecutes y propone dividir la tarea o abrir una aclaración.
2. Ejecuta `run --task T` una vez y conserva el `execution_id`.
3. Implementa solo el resultado y el alcance de la tarea; ejecuta la verificación declarada.
4. Escribe la evidencia (procedimiento, entorno, resultado y `subject_ref` del commit o hash) y regístrala con `report --run X --file -` en un heredoc. El motor lee estas claves, una por línea, hasta `---`; si falta alguna responde `EVIDENCE-FIELDS` con el campo. Obtén el valor de `Sujeto` con `subject --files <archivos tocados> --json` (campo `data.subject_ref`), después de la última edición y sin inventar el hash; también vale `commit <sha>` si el árbol queda limpio:

   ```markdown
   Cubre: T-001, C-01
   Resultado: pass
   Procedimiento: go test ./...
   Entorno: darwin/arm64, go1.x
   Sujeto: tree:<hash> (archivos: store.go, store_test.go)
   Inspeccionable: salida adjunta abajo
   Ejecutor: claude
   ---
   {salida del comando}
   ```
5. Si se pierde la respuesta de `report`, repite con el mismo `--operation-id`; nunca repitas `run` sobre la misma tarea.
6. Si aparece ampliación de alcance, una excepción no aprobada, revisión incompatible, permiso insuficiente o resultado incierto, detén esa parte y registra `block` o pasa a `hcd-clarify`; continúa solo con trabajo independiente autorizado.
