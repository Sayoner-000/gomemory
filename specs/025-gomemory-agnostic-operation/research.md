# Investigación: Operación agnóstica de gomemory

## R1. Registro MCP único de Codex

**Decisión**: mantener una única entrada global de gomemory, sin directorio de trabajo por proyecto;
migrar selectivamente todas las entradas heredadas con nombre `gomemory_*` antes de comprobar si ya
existe la entrada global.

**Racional**: una configuración personal debe servir a todos los proyectos. Las rutas de proyectos
eliminados fueron la causa de arranques fallidos. La migración previa evita que una entrada global
correcta oculte entradas obsoletas todavía activas.

**Alternativas consideradas**:

- Mantener una entrada por proyecto: descartada porque conserva rutas obsoletas y duplica clientes.
- Reescribir o reformatear toda la configuración: descartada porque puede perder comentarios y valores
  de servidores ajenos.
- Escribir directamente sobre el archivo: descartada porque una interrupción puede dejarlo truncado.

**Diseño resultante**: una transformación por secciones conserva contenido ajeno; un bloqueo dentro
del proceso, la idempotencia y un reemplazo seguro producen exactamente una entrada. Cuando se elimina
una entrada heredada, se guarda primero una copia exclusiva del archivo original y se preservan sus
permisos.

## R2. Copiar y pegar en la TUI

**Decisión**: centralizar la acción de copia en el enrutador principal de la TUI y obtener el texto de
una representación semántica de la pantalla; dirigir todos los mensajes no asociados a teclas al
campo de texto que tenga el foco.

**Racional**: una acción única evita implementaciones divergentes por pantalla. La copia del modelo
preserva el contenido de la memoria y no depende de colores o recortes de la vista. El pegado puede
llegar como evento posterior a la tecla que lo originó, por lo que debe alcanzar al control enfocado.

**Alternativas consideradas**:

- Copiar el texto ya renderizado: descartada porque incorpora formato y puede contener solo una
  ventana visible.
- Gestionar pegar en cada formulario: descartada porque omite campos y duplica rutas de eventos.
- Usar comandos de portapapeles del sistema: descartada porque no es portable ni necesaria.

**Diseño resultante**: la copia incluye el detalle íntegro y sus metadatos presentables; documentos y
otras vistas usan su contenido lógico o texto sin formato. La indisponibilidad del portapapeles no
modifica la memoria ni interrumpe la TUI.

## R3. Desplazamiento de detalles extensos

**Decisión**: recorrer líneas ajustadas al ancho actual, con un desplazamiento de línea acotado entre
el inicio y la última línea visible posible.

**Racional**: las líneas visuales, no los caracteres, se corresponden con lo que una persona ve en
una terminal de tamaño variable. El estado acotado evita índices inválidos para contenido corto o
vacío.

**Alternativas consideradas**:

- Renderizar el contenido completo: descartada porque no cabe en una terminal y no permite navegación.
- Desplazarse por caracteres: descartada porque rompe el ajuste visual y la lectura.

**Diseño resultante**: avance y retroceso por línea, página, inicio y final; la vista presenta una
ventana e indicadores de contenido fuera de ella. Copiar sigue usando la memoria completa, no esa
ventana.

## R4. Constitución predeterminada agnóstica

**Decisión**: conservar la constitución como documento fijado de cada proyecto; usar la plantilla
embebida solo para la primera siembra, la consulta de reserva y una restauración explícita.

**Racional**: la memoria del proyecto es la fuente de verdad operativa y protege el texto que un equipo
personalizó. Una plantilla sin atribuciones específicas sirve como punto de partida reutilizable sin
imponer identidad.

**Alternativas consideradas**:

- Copiar una constitución estática a cada instalación: descartada por divergencia y sobrescritura.
- Reemplazar constituciones existentes durante una actualización: descartada porque destruiría
  personalizaciones.

**Diseño resultante**: la instalación siembra solo si falta el documento; la importación es explícita y
valida contenido no vacío; la restauración es una acción explícita. La sincronización con spec-kit
solo ocurre si su directorio ya existe.

## R5. Evidencia de validación

**Decisión**: validar mediante pruebas unitarias de frontera, pruebas de compilación y escenarios
manuales de la guía rápida.

**Racional**: la migración de archivo, los eventos de terminal y el contenido de la plantilla tienen
fronteras distintas. Las pruebas deben verificar el efecto observable de cada una sin requerir acceso
a configuraciones personales reales.

**Alternativas consideradas**: una única prueba integral de instalación. Se descarta porque no cubre
con precisión fallos de respaldo, concurrencia, eventos de pegado o conservación de personalizaciones.
