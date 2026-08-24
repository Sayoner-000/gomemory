# Modelo de datos: Operación agnóstica de gomemory

No se introducen tablas ni migraciones. Este modelo documenta el estado que las interfaces ya
administran y las invariantes que deben preservarse.

## Registro MCP compartido

| Campo | Significado | Regla |
|---|---|---|
| Identidad | Nombre estable de la entrada gomemory | Debe existir exactamente una en el ámbito personal. |
| Comando | Proceso que inicia el servidor | Debe ejecutar el modo MCP de gomemory. |
| Argumentos | Parámetros de inicio | Deben describir el modo MCP y no incluir una ruta de proyecto. |
| Directorio de trabajo | Dependencia de ubicación | No debe estar presente. |

**Transiciones**: inexistente → compartido; registros heredados → respaldo + compartido; compartido
→ compartido ante una repetición. Una migración que no pueda respaldar el estado anterior termina sin
modificar la configuración activa.

## Registro MCP antiguo

| Campo | Significado | Regla |
|---|---|---|
| Identidad heredada | Nombre asociado a un proyecto | Se elimina durante la migración. |
| Ruta heredada | Ubicación que antes determinaba el inicio | Puede no existir; nunca debe bloquear la migración. |
| Contenido ajeno | Otras secciones de la configuración | Debe conservarse sin alteración semántica. |

## Respaldo de configuración

| Campo | Significado | Regla |
|---|---|---|
| Contenido | Copia previa a la migración | Debe ser exacta. |
| Identificador temporal | Distingue respaldos sucesivos | Debe ser único; nunca sustituye otro respaldo. |
| Permisos | Restricciones de acceso | Deben igualar las del archivo reemplazado. |

## Estado textual de la TUI

| Entidad | Atributos | Invariantes |
|---|---|---|
| Vista textual | pantalla actual y texto lógico | Puede copiarse sin formato visual. |
| Detalle de memoria | título, tipo, fecha, contenido, ruta y sesión opcionales | La copia contiene todo el contenido, no solo la ventana visible. |
| Posición de desplazamiento | línea visual actual y última línea posible | Siempre está dentro de los límites, incluso si no hay contenido. |
| Campo activo | control de entrada con foco | Es el único receptor de texto pegado. |
| Estado de aviso | mensaje y duración | Confirma una solicitud de copia sin cambiar el contenido. |

## Documento fijado de constitución

| Campo | Significado | Regla |
|---|---|---|
| Alias | Nombre que usa la persona para gestionarlo | Identifica la constitución. |
| Clave de tema | Identidad estable en la memoria del proyecto | No cambia por una actualización. |
| Contenido | Constitución vigente | Si existe, prevalece sobre la plantilla. |
| Estado | sin sembrar, por defecto o personalizado | Se deriva comparando el contenido con la plantilla. |
| Plantilla | Contenido base embebido | No contiene identidad o atribución particular. |

**Transiciones**: sin sembrar → por defecto al inicializar; por defecto o personalizado → personalizado
al importar; cualquier estado → por defecto solo ante una restauración explícita. La actualización o
siembra repetida no cambia un documento existente.
