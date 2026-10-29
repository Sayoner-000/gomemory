# Contrato: formato parseable de artefactos HCD

**Cubre**: FR-015, FR-020, FR-021, FR-052, FR-053; research R5.

## Reglas generales

- Markdown UTF-8 con finales de línea LF.
- Un elemento empieza con un encabezado que lleva su ID seguido de punto: `## US1.`, `### C-01.`, `## VS-01.`, `### T-001.`.
- Justo debajo van líneas `Clave: valor`. Una clave admite una lista separada por comas (`Historias: US1, US2`).
- Las subsecciones `#### Nombre` son texto libre y pertenecen al elemento.
- Una clave desconocida produce un aviso; una clave obligatoria ausente produce un `pending_check`.
- Las revisiones son SHA-256 del contenido normalizado (LF, sin espacio al final de línea), abreviado a 12 hexadecimales.

## `definition.md`

```markdown
# {Título}

ID: HCD-001
Tipo: fix
Enfoque: hcd
Implementador: {persona}
Playbook: {revisión}

## Intención
Problema: …
Actor: …
Resultado esperado: …

## Alcance
Incluido: …
Excluido: …
Restricciones: …

## Particular del tipo
Fallo observado: …                 # fix
Reproducción: …                    # fix
Causa raíz: pendiente | …          # fix

## US1. {Título de la historia}
Actor: …
Texto: Como …, quiero …, para …

### C-01. {Condición}
Historias: US1
Verificación: automated
Condición: Dado …, cuando …, entonces …

## Decisiones pendientes
- …

## Referencias
- …
```

**Marcadores de pendiente** (FR-016a): un valor `pendiente`, `[pendiente]`, `TODO` o `[NEEDS CLARIFICATION: …]` (sin distinguir mayúsculas) cuenta como vacío. `validate` lo informa como `pending_check` `DEF-PENDING` con el campo afectado. La única excepción es `Causa raíz: pendiente` en una unidad `fix`, que es válida hasta el cierre.

**Obligatorios para `validate`** (FR-015): ID, Tipo, Título, Implementador, Problema, Actor, Resultado esperado, Incluido, Excluido, Restricciones, al menos una historia, al menos un criterio por historia, Verificación de cada criterio, Playbook y los campos particulares del tipo (FR-014).

## `delivery.md`

```markdown
# Entrega HCD-001

## VS-01. {Comportamiento entregable}
Historias: US1
Criterios: C-01
Dependencias: ninguna
#### Comportamiento observable
#### Límites
#### Impacto arquitectónico conocido
#### Verificación esperada
#### Incertidumbres

### T-001. Rechazar una tarea con referencias inexistentes
Slice: VS-01
Historias: US1
Criterios: C-01
Tipo: implementation
Dependencias: ninguna
Reglas: HCD-TRACE-01
Resultado: …
Alcance: …
Excluido: …
Verificación: …
Evidencia esperada: …
```

Los estados **no** se escriben en `delivery.md`: viven en el estado de control y se muestran en `verification.md`. Así una edición manual no puede marcar trabajo como verificado.

## Evidencia (`report --file`)

```markdown
Cubre: T-001, C-01
Resultado: pass
Procedimiento: go test ./domain -run TestHCDAtomicity
Entorno: darwin/arm64, go1.x
Sujeto: commit abc1234 | tree:9f8e7d6c5b4a (archivos: …)
Inspeccionable: salida adjunta abajo
Ejecutor: claude
---
{salida}
```

## `verification.md` (generado)

Cabecera `<!-- GENERADO por mem hcd verify; no editar -->`, resultado global PASS, FAIL o BLOCKED, secciones Protocolo, Trazabilidad, Criterios, Evidencias (vigentes y desactualizadas), Aprobaciones (origen y cita), Excepciones, Bloqueos y Acción requerida. Si alguien la edita, la edición se descarta en la siguiente regeneración y el motor la avisa.
