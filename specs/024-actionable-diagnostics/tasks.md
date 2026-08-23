# Tareas — Diagnóstico accionable y vitalidad

**Estado**: completa. Las tres historias implementadas y verificadas contra el binario.

## Fase 1 — Informe accionable (Historia 1) ✅

- [X] **T001** Pruebas en rojo: todo canal declara su efecto, en términos de comportamiento y no de mecanismo.
- [X] **T002** Pruebas en rojo: todo agente declara comando de corrección, y advierte cuando el efecto sale del proyecto.
- [X] **T003** `EfectoDelCanal` y `CorreccionPara` en el dominio, junto al canal y no en quien imprime (FR-008).
- [X] **T004** Agrupar las correcciones que comparten comando y listar los efectos sin repetir (FR-004).
- [X] **T005** Declarar que las degradaciones no requieren acción, y afirmarlo explícitamente cuando no hay problemas (FR-003, FR-005).
- [X] **T006** Exponer efecto, corrección y advertencia en la salida legible por máquina (FR-007).
- [X] **T007** Verificar contra el binario con un canal realmente caído.

## Fase 2 — Verificación previa a publicar (Historia 3) ✅

- [X] **T008** Contrato que extrae los hooks que el complemento registra y los contrasta con la interfaz publicada por el agente.
- [X] **T009** Omitir el contrato sin error cuando esa interfaz no está en el entorno (FR-018).
- [X] **T010** Verificar en rojo: simular un renombre y comprobar que la batería falla nombrando la operación.

## Fase 3 — Vitalidad en ejecución (Historia 2) ✅

- [X] **T011** Registrar cuándo se ejerció por última vez cada canal de inyección (FR-009).
- [X] **T012** Reportar como inactivo el canal cuya última actividad supere un umbral (FR-010).
- [X] **T013** Distinguir un canal sin actividad por falta de sesiones de uno que no responde habiéndolas (FR-011).
- [X] **T014** Que las rutas de error del complemento dejen rastro legible por el informe (FR-012).

## Cierre parcial

**Antes**, un canal caído se reportaba así:

```
❌ gomemory opencode user plan_entry  no encontrado: <ruta>
```

**Ahora**:

```
Qué hacer:

  Afecta a 2 canal(es): opencode · user · instructions, opencode · user · plan_entry
    • el agente no lee el protocolo de memoria al arrancar y usa las herramientas solo si se lo pides
    • al entrar en modo plan, el agente no recibe el método de descomposición ni el historial
    ⚠️  afecta a todos los proyectos de esta máquina, no solo a este
    → mem setup-mcp --scope global --agents opencode
```

**Historia 2, cerrada**: tabla `channel_activity`, puerto `ports.ChannelActivityLog`,
subcomandos `channel-fired` y `channel-error` que el complemento invoca, y una sección del
informe que solo aparece cuando hay algo que decir.

**Dos defectos encontrados al validar contra el binario, no en los tests:**

1. *Los argumentos se leían corridos una posición.* `args[0]` es el propio subcomando, así que
   el rastro se anotaba con `channel-error` como nombre de agente y nunca llegaba al informe.
   La batería estaba en verde porque ninguna prueba ejercía el despacho completo. Corregido, con
   `TestHookChannelActivity_LeeLosArgumentosEnSuPosicion` fijando la regresión.

2. *Alerta falsa.* La primera versión acusaba a un canal que nunca se había ejercido si había
   habido sesiones de trabajo. Pero las sesiones no se atribuyen a un agente: trabajar con Claude
   Code hacía parecer muerto el canal de OpenCode, que simplemente no se estaba usando. Ahora solo
   se reporta el canal que **demostró funcionar y dejó de hacerlo**, que es donde hay evidencia de
   deterioro y no de desuso. Un informe que alarma sin motivo pierde la credibilidad justo donde
   debe ganarla.

**Verificado end-to-end**: un canal que funcionó y se calló se reporta con la fecha del último
uso, el fallo registrado, el efecto y el comando; en cuanto vuelve a ejercerse, la sección
desaparece.
