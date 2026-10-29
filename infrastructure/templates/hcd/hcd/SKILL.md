---
name: hcd
description: Conduce el paso hcd del protocolo HCD mediante el motor mem hcd.
---

# HCD — entrada

## Reglas comunes

1. **Motor primero.** Al empezar, la skill ejecuta `mem hcd resume <unit> --json --client <agente>` (o `./mem` si `mem` no está en PATH) y actúa solo dentro de `allowed_actions`.
2. **Sin motor no hay avance.** Si ni `mem` ni `./mem` existen, o si el motor responde `internal`, la skill informa la limitación y se detiene. No edita artefactos para simular un estado.
3. **Las mutaciones van por el motor.** La skill redacta el contenido de `definition.md`, `delivery.md` o la evidencia y lo pasa al motor por stdin con `--file -` (por ejemplo, `mem hcd define HCD-001 --file - <<'EOF'`), registrándolo con `define`, `slice`, `tasks` o `report`. Si necesita un archivo, lo escribe fuera del repositorio. Nunca escribe estados, aprobaciones, evidencias ni `verification.md` a mano.
4. **Preguntar antes que suponer** (FR-016a). Ante una ambigüedad que cambia comportamiento, contrato, datos, seguridad o alcance, la skill abre `hcd-clarify`: hace una pregunta a la vez, con 2 a 4 opciones y una recomendación justificada, y registra la respuesta. No elige la interpretación más probable.
5. **Aprobación citada** (FR-017a). Para aprobar, la skill muestra el objeto y su revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal de la persona en `--quote`. Nunca redacta la cita ni aprueba sin una respuesta en la conversación.
6. **Detención localizada** (US8). Ante una ampliación de alcance, una excepción no aprobada, una revisión incompatible, un permiso insuficiente o un resultado incierto, la skill registra un bloqueo o una aclaración sobre la parte afectada (que genera una decisión al resolverse) y continúa solo con trabajo independiente autorizado.
7. **Siguiente paso.** Toda respuesta termina con el `next_action` del motor: nombra la skill siguiente y por qué.
8. **Privacidad.** No pone secretos en artefactos, evidencias ni citas.

## Uso

- **Sin argumentos:** intenta `resume` de la unidad activa; si no hay unidad activa, ejecuta `list` y ofrece retomar una unidad no terminal o pregunta qué se desea iniciar.
- **Con id:** ejecuta `resume <id>` y presenta etapa, pendientes y siguiente acción.
- **Con solicitud nueva:** propone tipo y título y confirma con la persona antes de ejecutar `start --kind <fix|feature|evolution|technical> --title "…" [--implementer "…"]`; después continúa con `hcd-define`. `fix` corrige un fallo observado; `feature` añade un comportamiento nuevo; `evolution` cambia uno existente; `technical` mejora una propiedad técnica medible.
- No guardes el foco en archivos de contexto del agente. El motor es la fuente de verdad.
- Para cambiar la unidad activa, ejecuta `focus <id>` y confirma el resultado del motor; no escribas archivos de contexto.
- Ofrece `classify` y `memory sync` solo si aparecen en `allowed_actions`.
