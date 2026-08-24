# Contrato de interacción: texto en la TUI

## Copia

- La acción de copia está disponible de forma consistente desde las vistas textuales.
- El detalle de memoria copiado contiene título, tipo, fecha, contenido completo y metadatos opcionales
  presentables.
- Copiar no depende de la posición de desplazamiento ni de colores o recortes de la pantalla.
- La interfaz confirma la acción; si el terminal no ofrece portapapeles, permanece utilizable.

## Pegado

- El campo editable que tiene foco recibe el texto que entrega el terminal.
- El texto se conserva sin pérdida ni alteración de caracteres.
- El comportamiento cubre filtros, formularios, rutas, confirmaciones y configuración.

## Desplazamiento del detalle

- Las acciones permiten ir una línea, una página, al inicio y al final.
- El desplazamiento se calcula sobre líneas visuales ajustadas al ancho actual.
- La posición nunca queda fuera de los límites y la vista señala contenido superior o inferior cuando
  existe.
