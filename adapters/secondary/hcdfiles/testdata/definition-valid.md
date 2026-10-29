# Fix de ejemplo

ID: HCD-001
Tipo: fix
Enfoque: hcd
Implementador: dev
Playbook: abc123def456

## Intención
Problema: falla al guardar
Actor: persona usuaria
Resultado esperado: guardar correctamente

## Alcance
Incluido: persistir datos
Excluido: cambiar interfaz
Restricciones: mantener compatibilidad

## Particular del tipo
Fallo observado: error al guardar
Reproducción: ejecutar el guardado
Causa raíz: pendiente

## US1. Guardar datos
Actor: persona usuaria
Texto: Como persona, quiero guardar, para conservar mis datos.

### C-01. Guarda el registro
Historias: US1
Verificación: automated
Condición: cuando guarda, entonces se conserva el registro

## Decisiones pendientes
- …

## Referencias
- …
