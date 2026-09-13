# Contrato: secciones de `get_context` / `mem context`

## 🧭 Anclas sin evidencia (nueva, US2)

Posición: justo después de `## 🔥 Memoria conectada a código activo` (o donde
iría esa sección si no aparece). Se omite si no hay anclas que listar o si no
cabe en el presupuesto.

```
## 🧭 Anclas sin evidencia

> Hipótesis, no orden de borrado: verifica y usa judge_memories/forget_memory.

- [<id>] «<título>» — `<ruta>` → movida a `<ruta-candidata>`
- [<id>] «<título>» — `<ruta>` → movida (ambigua: <k> rutas con el mismo nombre)
- [<id>] «<título>» — `<ruta>` → huérfana candidata (no está en disco ni en el índice)
```

- Como máximo 8 líneas de entrada, en orden de id ascendente, cada una bajo el presupuesto.
- Sin índice de código, la leyenda añade la línea `> Sin índice de código: solo se comprobó el disco.`, y la variante de huérfana dice `(no está en disco)`.
- Nunca aparecen anclas vigentes ni no verificables (ruta vacía, absoluta fuera del proyecto, directorio).

## 🔗 Sinapsis (memorias enlazadas) (cambia, US3)

El formato de línea se mantiene:

```
- [<a>] <título a> ↔ [<b>] <título b>
- [<a>] <título a> ⇒ supera a [<b>] <título b>
```

Cambios:

1. **Orden**: por `masa(a)+masa(b)` descendente; en empate, la relación más reciente primero (id de relación ↓), que era el orden anterior. Precisado en la validación del 2026-09-13: con semillas concentradas casi todas las aristas empatan a masa 0, y desempatar por id de memoria mostraba primero las más viejas.
2. **Exclusión**: ninguna línea tiene un checkpoint ni una memoria inexistente en un extremo.
3. **Universo**: todas las relaciones del proyecto, no las 20 más recientes.
4. **Tope**: 12 líneas, cada una bajo el presupuesto.
5. **Leyenda** al final de la sección:

```
_Orden: masa = centralidad en el grafo de memorias sembrado en <semillas>; no mide importancia ni corrección._
```

`<semillas>` es uno de estos textos: `sesión activa (N)`, `anclas a hotspot (N)`,
`sesión activa (N) + anclas a hotspot (M)` o `todas las memorias (uniforme)`.

## ⚠ Conflictos sin resolver (sin cambio de formato)

El único cambio es de universo: se leen todas las relaciones del proyecto. Un
`conflicts_with` antiguo ya no desaparece por el recorte a 20.
