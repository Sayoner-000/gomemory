# Referencia: ADS optimizado (línea base aportada por el usuario)

**Origen**: aportado por el usuario el 2026-08-06 durante `/speckit-specify`, como versión
ya optimizada del documento original `SKILL_ATOMIC_DESCOMPOSITION_SYSTEMS_ADS.md`.

**Estatus**: es la **línea base** del método que esta feature distribuye. No es el
entregable final: le falta la parte de memoria (uso del historial del proyecto al
descomponer, FR-016) y el paso de autoverificación previo a la entrega (FR-018, FR-019).
La fase de `/speckit-plan` debe partir de este texto, no del documento original.

**Nota sobre el alcance de ejecución**: este texto incluye una rama "Modo ejecución". No
contradice FR-020/FR-021 —que dejan la ejecución fuera del alcance— porque la propia
rama de modo plan ordena entregar el árbol y detenerse. La rama de ejecución solo aplica
cuando el método se invoca fuera de modo plan, y nunca desplaza a `/speckit-implement`
dentro del flujo SDD del proyecto.

---

```markdown
---
name: atomic-decomposition
description: Úsala cuando una solicitud sea grande, multi-paso, vaga o de resultado incierto y deba dividirse en subtareas atómicas (indivisibles, verificables, ejecutables una a una) antes de planificar o ejecutar. Ideal en modo /plan.
---

# Descomposición Atómica

Convierte cualquier objetivo en un árbol de subtareas atómicas ordenadas por
dependencia y, si no estás en modo plan, las ejecuta una a una.

## Test de atomicidad

Una subtarea es ATÓMICA solo si cumple TODO:

- [ ] Un verbo de acción + un objeto concreto.
- [ ] No queda ningún "cómo" por decidir; es ejecutable tal cual.
- [ ] Produce **un** resultado verificable.
- [ ] Cabe en una sola unidad de trabajo (≈ una llamada de herramienta / un paso enfocado).
- [ ] No necesita el resultado de una hermana para empezar (solo de niveles superiores).

Si falla alguno → divídela. Si dividir más solo añade ruido → detente.

- Atómica: `Calcular el 15% de 2.400` · `Redactar intro de 50 palabras` · `Buscar la definición de X`
- No atómica: `Hacer el informe` · `Configurar el entorno`

## Procedimiento

1. **Objetivo** — enúncialo en una frase con condición de "hecho" verificable.
2. **Nivel 1** — lista los procesos mínimos necesarios, en orden de dependencia.
3. **Recursión** — divide cada subtarea hasta que pase el test. Máx. 6 niveles.
4. **Nomenclatura** — `[id] verbo + objeto → resultado esperado`.
5. **Dependencias** — marca qué exige el resultado de otra (`dep: 1.2`); lo demás va en paralelo (`∥`).

## Salida: el plan

```
🎯 [objetivo]
├─ [1] subtarea
│  ├─ [1.1] ✓ atómica
│  └─ [1.2] subtarea        (dep: 1.1)
│     └─ [1.2.1] ✓ atómica
└─ [2] ✓ atómica            (∥ paralela)
```

`✓` = hoja atómica ejecutable. Anota dependencias y paralelismo.

## Confirmar antes de seguir SOLO si

- El objetivo admite varias lecturas válidas.
- Hay > 25 hojas atómicas → propón priorización.
- El usuario pidió revisar el plan.

En otro caso, continúa.

## Ejecución

- **Modo plan** (`/plan`): entrega el árbol y **detente**. No ejecutes.
- **Modo ejecución**: recorre las hojas en orden de dependencia.
  - Por cada una: `▶ [id] nombre` → resultado → `[✓]` o `[✗ motivo]`.
  - Guarda resultados intermedios como insumo de las siguientes.
  - Al cerrar cada nivel, verifica que el parcial sigue alineado al objetivo; corrige si se desvía.
  - Al final: integra en un entregable único + resumen `N/N` + verificación contra cada requisito.

## Reglas

1. Nunca ejecutes una tarea grande sin descomponerla.
2. Una hoja atómica se ejecuta de una vez, sin cortes internos.
3. Nueva solicitud a mitad de camino → termina la hoja en curso, descompón la nueva, decide qué plan sigue.
4. Dependencia circular → rómpela con un orden de prioridad explícito.
5. Estricto con la atomicidad, pero pragmático: menos ruido, más precisión.
```

---

## Brecha respecto de la especificación

Lo que este texto **ya cubre**: FR-009 a FR-015, FR-017, FR-020 y FR-021 (la rama de modo
plan ordena entregar y detenerse), además del formato de salida del árbol.

Lo que **falta añadir** en `/speckit-plan`:

| Requisito | Qué falta |
|-----------|-----------|
| FR-001 | Invocar gomemory de forma autónoma al entrar en modo plan |
| FR-002 | Expresar la instrucción en el protocolo común, no en un formato propio de un agente |
| FR-003 | Usar servidor de herramientas o línea de comandos, según lo disponible |
| FR-016 | Usar el historial del proyecto (decisiones, convenciones, causas raíz) al descomponer |
| FR-018 | Paso de autoverificación de cada hoja contra el test de atomicidad antes de entregar |
| FR-019 | Marcar como no atómica, con motivo, la hoja que no pueda descomponerse más |

**Observación sobre el test de atomicidad**: la condición "cabe en una sola unidad de
trabajo (≈ una llamada de herramienta / un paso enfocado)" está formulada en términos del
agente que ejecuta. Al integrarla, conviene revisar que siga siendo comprobable por una
persona que lee el plan, no solo por el agente que lo escribió (SC-003).
