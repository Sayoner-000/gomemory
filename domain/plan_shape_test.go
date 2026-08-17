package domain

import "testing"

// longProse repite una frase sin ninguna señal de estructura hasta superar
// el umbral de tamaño (data-model.md §1), para que las pruebas de ShapeMissing
// no dependan de un número mágico de caracteres.
func longProse(sentence string, times int) string {
	out := ""
	for i := 0; i < times; i++ {
		out += sentence + " "
	}
	return out
}

func TestEvaluatePlanShape(t *testing.T) {
	cases := []struct {
		name string
		plan string
		want PlanShapeVerdict
	}{
		{
			name: "árbol con glifos → ShapeOK",
			plan: "🎯 objetivo\n├─ [1] subtarea\n│  └─ [1.1] verbo + objeto → resultado\n" +
				longProse("relleno para superar el umbral de tamaño del plan.", 5),
			want: ShapeOK,
		},
		{
			name: "identificadores jerárquicos de dos niveles → ShapeOK",
			plan: "Paso 1.1: hacer una cosa. Paso 1.2: hacer otra cosa relacionada con la anterior. " +
				longProse("más contexto sobre el paso y sus dependencias internas.", 4),
			want: ShapeOK,
		},
		{
			name: "marcadores del método (✓ ⚠ dep: ∥) → ShapeOK",
			plan: "Lista de tareas: primera tarea ✓ completada, segunda ⚠ pendiente de revisión, " +
				"tercera dep: primera, cuarta ∥ en paralelo con la tercera. " +
				longProse("contexto adicional sobre el plan y sus pasos.", 4),
			want: ShapeOK,
		},
		{
			name: "prosa larga sin ninguna señal de estructura → ShapeMissing",
			plan: longProse("Voy a implementar la integración completa revisando todo el código.", 10),
			want: ShapeMissing,
		},
		{
			name: "plan corto (trivial) → ShapeNotApplicable",
			plan: "Cambiar el título del README.",
			want: ShapeNotApplicable,
		},
		{
			name: "vacío → ShapeNotApplicable",
			plan: "",
			want: ShapeNotApplicable,
		},
		{
			name: "solo espacios → ShapeNotApplicable",
			plan: "   \n\t  \n  ",
			want: ShapeNotApplicable,
		},
		{
			name: "árbol en inglés (mismos glifos) → ShapeOK",
			plan: "Goal: ship the feature\n├─ [1] write the code\n│  └─ [1.1] verb + object → verifiable result\n" +
				longProse("additional context padding out the plan text.", 5),
			want: ShapeOK,
		},
		{
			name: "prosa larga con una sola flecha y sin estructura → ShapeMissing",
			plan: longProse("Primero reviso el código y luego avanzo hacia la solución final.", 8) +
				" el resultado esperado → que todo funcione bien al final del proceso.",
			want: ShapeMissing,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EvaluatePlanShape(c.plan)
			if got != c.want {
				t.Errorf("EvaluatePlanShape(%q...) = %v, se esperaba %v", truncate(c.plan, 60), got, c.want)
			}
		})
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
