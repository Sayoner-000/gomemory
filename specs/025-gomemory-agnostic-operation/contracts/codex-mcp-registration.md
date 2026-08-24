# Contrato de integración: registro MCP de Codex

## Postcondición de instalación o actualización

La configuración personal de Codex contiene exactamente un registro de gomemory que inicia el modo
MCP sin un directorio de trabajo asociado a un proyecto.

## Conservación y seguridad

- Todos los registros heredados de gomemory asociados a proyectos se retiran, aun si la ruta ya no
  existe.
- Los servidores, comentarios y secciones ajenos a gomemory se conservan.
- Si se retira al menos un registro heredado, se crea una copia exacta recuperable antes del reemplazo.
- La configuración activa se reemplaza de forma que un error no publique contenido parcial.
- Los permisos del archivo existente se preservan.
- Repetir la operación o solicitarla de forma concurrente deja un solo registro compartido.

## Comportamiento de desinstalación

La desinstalación de un proyecto no elimina el registro compartido y comunica que pertenece al ámbito
personal, ya que otros proyectos pueden seguir utilizándolo.
