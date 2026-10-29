---
name: hcd-tasks
description: Conduce el paso hcd-tasks del protocolo HCD mediante el motor mem hcd.
---

# HCD — tareas atómicas

## Reglas comunes

1. **Motor primero.** Al empezar, la skill ejecuta `mem hcd resume <unit> --json --client <agente>` (o `./mem` si `mem` no está en PATH) y actúa solo dentro de `allowed_actions`.
2. **Sin motor no hay avance.** Si ni `mem` ni `./mem` existen, o si el motor responde `internal`, la skill informa la limitación y se detiene. No edita artefactos para simular un estado.
3. **Las mutaciones van por el motor.** La skill redacta el contenido de `definition.md`, `delivery.md` o la evidencia y lo pasa al motor por stdin con `--file -` (por ejemplo, `mem hcd define HCD-001 --file - <<'EOF'`), registrándolo con `define`, `slice`, `tasks` o `report`. Si necesita un archivo, lo escribe fuera del repositorio. Nunca escribe estados, aprobaciones, evidencias ni `verification.md` a mano.
4. **Preguntar antes que suponer** (FR-016a). Ante una ambigüedad que cambia comportamiento, contrato, datos, seguridad o alcance, la skill abre `hcd-clarify`: hace una pregunta a la vez, con 2 a 4 opciones y una recomendación justificada, y registra la respuesta. No elige la interpretación más probable.
5. **Aprobación citada** (FR-017a). Para aprobar, la skill muestra el objeto y su revisión de 12 hexadecimales, pide una elección explícita y pasa la respuesta literal de la persona en `--quote`. Nunca redacta la cita ni aprueba sin una respuesta en la conversación.
6. **Detención localizada** (US8). Ante una ampliación de alcance, una excepción no aprobada, una revisión incompatible, un permiso insuficiente o un resultado incierto, la skill registra un bloqueo o una aclaración sobre la parte afectada (que genera una decisión al resolverse) y continúa solo con trabajo independiente autorizado.
7. **Siguiente paso.** Toda respuesta termina con el `next_action` del motor: nombra la skill siguiente y por qué.
8. **Privacidad.** No pone secretos en artefactos, evidencias ni citas.

Descompón el slice en tareas de un resultado, con verbo y objeto, alcance, exclusiones, criterios enlazados, reglas, dependencias existentes y verificación. Registra con `tasks --slice <id> --file -` (heredoc). Ejecuta `check --task <id>` para cada tarea; corrige todo `ATOM-*` antes de presentarla como lista admitida.

Formato que el motor reconoce (otro encabezado no se registra y `tasks` responde `TASK-EMPTY`; los estados no se escriben aquí):

```markdown
### T-001. {Verbo y objeto}
Slice: VS-01
Historias: US1
Criterios: C-01
Tipo: implementation
Dependencias: ninguna
Reglas: HCD-TRACE-01
Resultado: {un único resultado observable}
Alcance: {archivos o símbolos que toca}
Excluido: {lo que no hace}
Verificación: {comando o inspección}
Evidencia esperada: {qué quedará en el report}
```

Para retirar una tarea mal planteada (`tasks` solo añade o actualiza, y `block` no la saca del cálculo), propón una decisión y acéptala con la respuesta literal de la persona:

```markdown
Tipo: task_cancelled
Resumen: Retirar T-002: no corresponde a ningún criterio aprobado
Referencias: T-002
```

Registra con `decision --file -` (heredoc) y, tras la respuesta, `decision --accept D-NN --quote "<respuesta literal>"`. Al pedir la aceptación muestra la revisión de la definición: el motor solo admite una cita que la contenga (por ejemplo «acepto D-06 3633b5f1c2fb») o un «apruebo» explícito. La tarea deja de contar en la admisión, la verificación y el cierre; si ya tenía un bloqueo, resuélvelo con `block --resolve B-NN`.
