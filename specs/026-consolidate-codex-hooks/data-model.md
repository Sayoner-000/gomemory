# Modelo de configuración: Consolidación de hooks de Codex

## Fuente de configuración de hooks

Representa un archivo que Codex inspecciona para cargar automatizaciones.

| Campo | Descripción | Regla |
|---|---|---|
| Ruta | Ubicación absoluta reconocida por Codex | Al final solo `config.toml` puede estar activo |
| Formato | Representación de la configuración | El estado final usa TOML válido |
| Estado | Activa, respaldada o retirada | No puede haber dos fuentes activas |
| Hooks | Colección de eventos, grupos y acciones | Debe contener cada identidad normalizada exactamente una vez |

## Hook de sesión

Representa una acción que Codex ejecuta ante un evento de hook.

| Campo | Descripción | Regla |
|---|---|---|
| Posición | Índices de grupo y acción dentro de la fuente | Determina la clave de estado asociada |
| Evento | Clase de evento que contiene el grupo | Se conserva exactamente |
| Filtro | Fuentes del evento que activan el hook | Ausencia y valor explícito son identidades distintas |
| Tipo | Clase de acción | Se conserva sin listas específicas por proveedor |
| Comando | Acción exacta que se ejecuta | Participa en la identidad sin inspeccionar su contenido |
| Opciones | Límite y campos adicionales | Se conservan si son compatibles con la representación destino |

Relaciones:

- Una fuente activa contiene el conjunto deduplicado de hooks originales.
- Cada hook activo puede tener un estado de confianza asociado a su posición y fuente.
- Los destinos referenciados por los hooks se consumen, pero no se modifican.

## Identidad normalizada

Representa la clave estructural usada para detectar equivalencias. Incluye evento, filtro efectivo y todos los campos
de cada acción. No incluye estado de confianza ni procedencia física.

## Estado de confianza

Representa la autorización persistida para ejecutar un comando de hook.

| Campo | Descripción | Regla |
|---|---|---|
| Identificador | Fuente, evento e índices del hook | Debe apuntar a la fuente y posición vigentes |
| Hash confiable | Huella del comando autorizado | No se reutiliza para un comando diferente |
| Habilitado | Permiso efectivo de ejecución | Debe quedar activo tras la autorización |

## Transiciones de estado

```text
Estado actual
  ├─ config.toml: definiciones principales, incluidas equivalencias duplicadas
  └─ hooks.json: definiciones heredadas
        ↓ respaldo completo
Estado preparado
  ├─ candidato TOML validado: una aparición por identidad normalizada
  └─ originales recuperables
        ↓ retiro del JSON + nueva autorización
Estado consolidado
  ├─ config.toml: única fuente activa
  ├─ hooks no reubicados: confianza conservada
  └─ hooks migrados o reubicados: confianza renovada
        ↓ si falla cualquier verificación
Rollback
  └─ restauración de ambos originales y sus estados
```

## Reglas de validación

- Cada identidad normalizada del inventario original existe exactamente una vez en el resultado.
- Cada campo compatible del inventario original se conserva en la identidad resultante.
- Ninguna regla de migración inspecciona nombres de proveedores ni contenido de comandos.
- `[features] hooks = true` permanece sin cambios.
- No quedan claves de estado que contengan `/root/.codex/hooks.json`.
- No se conserva confianza cuando una posición cambia de identidad.
- Modelo, esfuerzo, MCP, confianza del proyecto y demás configuración permanecen semánticamente equivalentes fuera de
  las secciones de hooks y su estado; el diff debe limitar los cambios textuales a esas secciones.
