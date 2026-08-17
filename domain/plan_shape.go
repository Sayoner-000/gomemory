package domain

import (
	"regexp"
	"strings"
)

// PlanShapeVerdict es el veredicto sobre la forma de un plan (data-model.md
// §1). Lo produce una función pura, sin I/O ni dependencias: el hook del
// adaptador solo traduce payload → veredicto → dialecto (plan.md, Structure
// Decision).
type PlanShapeVerdict int

const (
	// ShapeOK: el plan presenta estructura de árbol reconocible. Permitir.
	ShapeOK PlanShapeVerdict = iota
	// ShapeMissing: el plan supera el umbral de tamaño y no presenta
	// ninguna señal de estructura. Devolver con motivo (si el episodio de
	// plan lo permite).
	ShapeMissing
	// ShapeNotApplicable: el plan es demasiado corto para exigir
	// descomposición, o no hay texto que evaluar. Permitir.
	ShapeNotApplicable
)

// minPlanLengthForShapeCheck es el umbral de tamaño (caracteres, tras
// recortar espacios) por debajo del cual un plan se considera trivial y no
// se le exige forma de árbol — regla 5 del método de descomposición atómica
// ("una solicitud trivial de un solo paso no necesita árbol"). Valor
// conservador y deliberado: cambiarlo es un cambio de dominio con su propia
// prueba, no un ajuste de configuración expuesto al usuario — aflojarlo por
// descuido bajaría la guardia de la Historia 1 en producción.
const minPlanLengthForShapeCheck = 200

// treeGlyphs y methodMarkers son señales de estructura independientes del
// idioma (data-model.md §1): cualquiera basta para ShapeOK.
var treeGlyphs = []string{"├─", "└─", "│"}
var methodMarkers = []string{"✓", "⚠", "dep:", "∥"}

// hierarchicalIDPattern reconoce un identificador jerárquico de dos o más
// niveles ("1.2", "1.2.3", o esa misma forma dentro de "[1.2]"): basta con
// que aparezca la secuencia dígito-punto-dígito en cualquier parte.
var hierarchicalIDPattern = regexp.MustCompile(`\d+\.\d+`)

// EvaluatePlanShape decide si plan presenta estructura de árbol reconocible.
// Sesgo estructural a permitir (data-model.md §1): ante la duda, ShapeOK o
// ShapeNotApplicable, nunca ShapeMissing — el coste de un falso bloqueo es
// mucho mayor que el de dejar pasar un plan mediocre.
//
// La flecha de resultado verificable ("→") NO participa de esta decisión:
// exigirla produciría falsos bloqueos en planes válidos con otro formato.
// Se usa únicamente para redactar el motivo de la devolución, fuera de esta
// función (contracts/hook-plan-guard.md).
func EvaluatePlanShape(plan string) PlanShapeVerdict {
	trimmed := strings.TrimSpace(plan)
	if len(trimmed) < minPlanLengthForShapeCheck {
		return ShapeNotApplicable
	}

	for _, g := range treeGlyphs {
		if strings.Contains(trimmed, g) {
			return ShapeOK
		}
	}
	for _, m := range methodMarkers {
		if strings.Contains(trimmed, m) {
			return ShapeOK
		}
	}
	if hierarchicalIDPattern.MatchString(trimmed) {
		return ShapeOK
	}

	return ShapeMissing
}
