# Contrato de agentes, distribución y consulta local

## Entradas

| Agente | Entrada |
| --- | --- |
| Claude Code | `/socratica-hu Quiero permitir que…` |
| OpenCode | `/socratica-hu Quiero permitir que…` |
| Codex | `$socratica prepara para speckit-specify: Quiero permitir que…` |
| Los tres | «Usa Socrática para preparar esta idea como entrada de Spec Kit…» |

`socratica-hu` es un wrapper de la skill `socratica` en modo `specify`, no una habilidad con reglas independientes. Acepta ideas y mejoras técnicas aunque no estén redactadas como historias.

## Procedimiento de la skill

1. Conservar la solicitud y consultar el contexto relevante. Iniciar o recuperar el flujo con su modo real; no convertir un pendiente de otro modo.
2. Seguir `next`, `stage`, `allowed_actions` y `pending_checks`. Proponer secciones mediante `prepare` y comprobar cada etapa; nunca simular un resultado válido ni sustituir los datos por una propuesta libre.
3. Formular como máximo tres preguntas acumuladas. Mostrar respuesta libre, omisión y omitir el resto. Una omisión deja el bloqueo; después del límite mostrar qué falta y permitir editar o aportar información voluntaria.
4. Presentar el texto final devuelto por gomemory, con original, supuestos y pendientes no bloqueantes. Aclarar que la persona valida el sentido de negocio. Esperar «Aprobar y generar especificación» antes de aprobar.
5. Comprobar que `speckit-specify` está disponible en el catálogo activo o puede cargarse desde la instalación del proyecto/anfitrión y que existe la estructura de proyecto requerida. Leer su contrato cuando sea necesario. Una ruta inventada o un binario disponible no demuestra que el agente pueda invocarla.
6. Si falta Spec Kit, conservar el estado y mostrar la entrada preparada; no consumir autorización ni instalar dependencias automáticamente.
7. Después de aprobar, confirmar con `ack` y ejecutar únicamente si la respuesta concede `execution_granted=true`. Mantener el nonce para reintentar una respuesta perdida; un replay sin concesión no autoriza ejecutar. Seguir las instrucciones de reconciliación cuando corresponda.
8. Invocar la skill `speckit-specify` con exactamente `approved_prompt` como argumento de usuario, sin prefijos añadidos, sin reescritura, sin construir un shell con ese contenido y sin implementar la necesidad descrita. La procedencia aprobada es metadato aparte.
9. Inspeccionar la especificación y su checklist realmente generados. Verificar alcance y trazar cada criterio preparado a sus historias de origen, conservar los identificadores de origen en el documento o una tabla de correspondencia y comunicar rutas y estado con `report`. Un fallo se comunica como fallo; no se infiere éxito del acuse.

Sin `socratica_flow` disponible, la skill puede ayudar en texto y declarar la limitación, pero no afirmar que existe preparación validada ni realizar automáticamente el traspaso gestionado. La persona puede usar Spec Kit directamente por separado.

## Distribución y desinstalación

- Fuente canónica: `infrastructure/templates/socratica/SKILL.md`; copia publicada: `skills/socratica/SKILL.md`. Deben ser idénticas tras la actualización.
- Nuevo wrapper: `infrastructure/templates/socratica/commands/socratica-hu.md`. Los destinos Claude/OpenCode se añaden al inventario existente `socraticaWrapperTargets` y al mapa de cuerpos instalado desde `cmd_mcp_setup.go`.
- Usar los instaladores existentes: solo raíces de agentes presentes, escribir únicamente si cambia el contenido y conservar los comandos previos. Codex no recibe un directorio de comandos cuya capacidad no esté verificada.
- La desinstalación retira los artefactos concretos gestionados y conserva contenedores compartidos y archivos ajenos conforme a sus reglas de propiedad. La nueva entrada debe aparecer en el inventario y comprobarse en una prueba real de eliminación, además de comprobar la lista.
- Las instrucciones dinámicas, solicitudes, respuestas y preparación nunca se escriben en el directorio de la skill.

## CLI y TUI

La consulta de pendientes, historial y detalle reconoce `specify`, muestra etapa, revisión, pendientes y revisión disponible, y ofrece una instrucción explícita para retomarlo desde el agente correspondiente. No se añade un generador de preparación fuera del agente.

Interceptar el modo antes de `drive`, `Execute`, edición textual o asistencia genérica. En `specify`, la consulta local y el dibujado no lanzan generadores ni consumen autorizaciones. La cancelación explícita sigue disponible cuando es válida. El resto de modos mantiene sus acciones actuales.

La vista de una reserva privada en otra instancia explica que el contenido solo existe en la instancia de origen; no promete mostrarlo ni recuperarlo. Si la instancia se perdió, se puede cancelar la reserva y empezar de nuevo explícitamente.

## Evidencia exigida

Para cada agente registrar versión del cliente, método de invocación, modo persistente/privado, preparación y revisión usadas, confirmación humana, resultado del `ack`, invocación de Spec Kit, rutas reales y correspondencia de criterios. Registrar los casos no ejecutados como pendientes; no sustituirlos por presencia de binario o pruebas de generación de `refine`.

La evidencia compartida debe usar datos sintéticos y omitir secretos y texto protegido. Las ejecuciones reales son opt-in y se separan de pruebas automáticas sin modelos externos.
