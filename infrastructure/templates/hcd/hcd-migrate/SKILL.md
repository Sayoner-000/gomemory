---
name: hcd-migrate
description: Conduce la migración voluntaria de una unidad Spec Kit a HCD mediante el motor mem hcd.
---

# HCD — migración desde Spec Kit

## Reglas comunes

1. **Motor primero.** Al empezar, la skill ejecuta `mem hcd resume <unit> --json --client <agente>` (o `./mem` si `mem` no está en PATH) y actúa solo dentro de `allowed_actions`.
2. **Sin motor no hay avance.** Si ni `mem` ni `./mem` existen, o si el motor responde `internal`, la skill informa la limitación y se detiene. No edita artefactos para simular un estado.
3. **Las mutaciones van por el motor.** La skill redacta el contenido de `definition.md`, `delivery.md` o la evidencia y lo pasa al motor por stdin con `--file -` (por ejemplo, `mem hcd define HCD-001 --file - <<'EOF'`), registrándolo con `define`, `slice`, `tasks` o `report`. Si necesita un archivo, lo escribe fuera del repositorio. Nunca escribe estados, aprobaciones, evidencias ni `verification.md` a mano.
4. **Preguntar antes que suponer** (FR-016a). Ante una ambigüedad que cambia comportamiento, contrato, datos, seguridad o alcance, la skill abre `hcd-clarify`: hace una pregunta a la vez, con 2 a 4 opciones y una recomendación justificada, y registra la respuesta. No elige la interpretación más probable.
5. **Aprobación citada** (FR-017a). Para aprobar, la skill muestra el objeto y su revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal de la persona en `--quote`. Nunca redacta la cita ni aprueba sin una respuesta en la conversación.
6. **Detención localizada** (US8). Ante una ampliación de alcance, una excepción no aprobada, una revisión incompatible, un permiso insuficiente o un resultado incierto, la skill registra un bloqueo o una aclaración sobre la parte afectada (que genera una decisión al resolverse) y continúa solo con trabajo independiente autorizado.
7. **Siguiente paso.** Toda respuesta termina con el `next_action` del motor: nombra la skill siguiente y por qué.
8. **Privacidad.** No pone secretos en artefactos, evidencias ni citas.

La migración es opcional y nunca se inicia sola: la unidad sigue en Spec Kit hasta que se activa. Esta skill no usa `resume` (aún no hay unidad HCD): trabaja con la carpeta de `specs/` y con el identificador `M-NN` de la propuesta.

1. **Propuesta.** Ejecuta `mem hcd migrate --preview <carpeta bajo specs/> --json --client <agente>`. No activa HCD ni escribe bajo `specs/`. Una propuesta nueva sobre la misma carpeta deja obsoleta la anterior.
2. **Revisión con la persona.** Presenta el recuento por clase (`migrated`, `consolidated`, `reference`, `discarded` con su justificación, `pending`) y recorre los elementos `pending`: son los que requieren una decisión. Ninguna clase es una decisión aprobada ni verificada. Una tarea `[x]` de Spec Kit queda como `reference`: no se promueve a verificada.
3. **Aprobación citada.** Muestra el identificador `M-NN` y su revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal en `--quote`. Nunca redacta la cita.
4. **Activación.** `mem hcd migrate --activate <M-NN> --revision <rev> --quote <respuesta literal> --implementer <Implementador> [--kind feature|evolution|technical|fix] --client <agente> --json`. Crea la unidad HCD (borrador), conserva las fuentes como `legacy-reference` y registra la decisión. Si las fuentes cambiaron desde la propuesta, el motor responde `revision_conflict`: vuelve al paso 1.
5. **Fallo.** Si la activación falla, la unidad sigue siendo Spec Kit; informa el error y repite desde el paso 1. No edites artefactos a mano para repararla.
6. **Después.** Continúa con `hcd-define` sobre la unidad nueva: el `next_action` del motor la nombra.
