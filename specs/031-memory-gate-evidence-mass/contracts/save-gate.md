# Contrato: aviso del gate de pre-escritura

Aplica a la tool MCP `save_memory` y a `mem save`. Los parámetros de entrada NO
cambian: el contrato solo añade líneas a la salida. El guardado ocurre siempre.

## `save_memory` (MCP): texto de la respuesta

Primera línea, igual que hoy:

```
✓ Memoria guardada (id=<id>)
```

Si hay candidatos (1 a 3), una línea por candidato:

```
⚠ Posible duplicado de #<N> «<título>» (similitud título <t.tt> · contenido <c.cc>) — si es el mismo tema, repite con topic_key o revisa get_memory <N>
```

Cuando el candidato tiene `topic_key`, el final dice
`— si es el mismo tema, repite con topic_key="<clave>" o revisa get_memory <N>`.

Tras las líneas de candidatos, una sola vez:

```
  (similitud léxica de palabras: no afirma que sea el mismo tema)
```

Si la comprobación no se pudo realizar:

```
ℹ Comprobación de duplicados no realizada (<motivo>): la memoria se guardó igual
```

Sin candidatos y con la comprobación hecha, o cuando el guardado actualizó una
memoria existente, no se añade nada.

## `mem save` (CLI)

- **stdout**: igual que hoy (`✓ Memoria guardada (id=…)` y la sesión activa).
- **stderr**: las mismas líneas `⚠`/`ℹ` definidas arriba. El código de salida no cambia (0).

## Invariantes comprobables

1. La memoria existe tras la llamada en todos los casos (con aviso, sin aviso o sin comprobación).
2. `#<N>` nunca es el id de la primera línea.
3. Hay como mucho 3 líneas `⚠`.
4. Dos llamadas idénticas sobre el mismo almacén producen los mismos candidatos en el mismo orden.
