# Descomposición Atómica

Convierte el objetivo en un árbol de subtareas atómicas ordenadas por dependencia.
Aplica este método al planificar, apoyándote en el historial del proyecto que viene
más abajo.

## 1. Apóyate en lo que el proyecto ya sabe

Antes de descomponer, revisa el contexto histórico adjunto y úsalo:

- **Decisiones y arquitectura** ya tomadas → no las replantees; construye sobre ellas.
- **Bugfixes con causa raíz** → no reintroduzcas un problema ya resuelto.
- **Patrones y convenciones** establecidos → una tarea que los contradiga está mal
  planteada, aunque suene razonable.
- **Preferencias del usuario** → condicionan cómo se entrega, no solo qué se hace.

Si el plan se aparta de algo ya registrado, dilo explícitamente y explica por qué.
Si no hay historial, sigue con el resto del método sin más.

## 2. Test de atomicidad

Una subtarea es ATÓMICA solo si cumple TODO:

- [ ] Un verbo de acción + un objeto concreto.
- [ ] No queda ningún "cómo" por decidir; es ejecutable tal cual.
- [ ] Produce **un** resultado verificable, que otra persona pueda comprobar leyendo
      el plan (un archivo que existe, una prueba que pasa, un valor calculado).
- [ ] Cabe en un solo paso enfocado, sin cortes internos.
- [ ] No necesita el resultado de una hermana para empezar (solo de niveles superiores).

Si falla alguno → divídela. Si dividir más solo añade ruido → detente.

- Atómica: `Calcular el 15% de 2.400` · `Redactar intro de 50 palabras`
- No atómica: `Hacer el informe` · `Configurar el entorno`

## 3. Procedimiento

1. **Objetivo** — enúncialo en una frase con condición de "hecho" verificable.
2. **Nivel 1** — lista los procesos mínimos necesarios, en orden de dependencia.
3. **Recursión** — divide cada subtarea hasta que pase el test. Máximo 6 niveles.
4. **Nomenclatura** — `[id] verbo + objeto → resultado esperado`.
5. **Dependencias** — marca lo que exige el resultado de otra (`dep: 1.2`); lo demás
   va en paralelo (`∥`).

## 4. Autoverificación (antes de entregar)

Este paso no es opcional y ocurre **antes de** mostrar el plan:

1. Recorre cada hoja del árbol y contrástala contra el test de atomicidad.
2. La que no lo pase, divídela y vuelve a contrastar.
3. Si una hoja **no puede** hacerse atómica —falta información, o depende de una
   decisión que le corresponde a la persona—, entrégala marcada como `⚠ no atómica`
   con el motivo declarado en una línea. No bloquees el plan por ella y no la
   disfraces de atómica.
4. Comprueba que ninguna hoja contradice el historial del punto 1.

## 5. Salida

```
🎯 [objetivo]
├─ [1] subtarea
│  ├─ [1.1] ✓ verbo + objeto → resultado verificable
│  └─ [1.2] subtarea                    (dep: 1.1)
│     └─ [1.2.1] ✓ verbo + objeto → resultado verificable
└─ [2] ⚠ no atómica → motivo por el que no pudo dividirse   (∥)
```

`✓` = hoja atómica ejecutable · `⚠` = hoja no atómica, con motivo · `dep:` =
depende de · `∥` = paralelizable.

## 6. Cuándo detenerte a preguntar

Solo si: el objetivo admite varias lecturas válidas · hay más de 25 hojas atómicas
(propón priorización) · la persona pidió revisar el plan. En otro caso, continúa.

## 7. Modo plan: entrega y detente

En modo plan **entrega el árbol y detente ahí. No ejecutes** ninguna tarea, no
edites archivos y no marques avance. La ejecución es un paso posterior y separado,
que la persona decide cuándo iniciar.

## Reglas

1. Nunca ejecutes una tarea grande sin descomponerla antes.
2. Una hoja atómica se ejecuta de una vez, sin cortes internos.
3. Solicitud nueva a mitad de camino → termina la hoja en curso, descompón la nueva,
   decide qué plan sigue.
4. Dependencia circular → rómpela con un orden de prioridad explícito.
5. Estricto con la atomicidad, pero pragmático: menos ruido, más precisión. Una
   solicitud trivial de un solo paso no necesita árbol.
