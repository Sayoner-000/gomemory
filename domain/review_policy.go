package domain

// ReviewPolicy son las reglas con las que una revisión nace y que ya no cambian
// aunque el proyecto se reconfigure después.
//
// Existe porque la política del proyecto ya estaba en Settings desde la
// funcionalidad 027 —ReviewMaxFixRounds, ReviewAutoFixSeverities, con sus defectos
// normalizados— pero NADIE la leía: StartReview reimplantaba `maxRounds = 2` y
// `{CRITICAL, HIGH}` a mano. Configurar el proyecto no tenía ningún efecto. Este tipo
// no añade configuración nueva; conecta la que ya existía y le suma la autorización
// de corrección (FR-017, FR-018).
type ReviewPolicy struct {
	FixAuthorized     bool
	MaxFixRounds      int
	AutoFixSeverities []Severity
}

// DefaultReviewPolicy es lo que rige cuando ni la revisión ni el proyecto declaran
// nada. Autoriza corregir porque es el comportamiento que tenían todas las revisiones
// antes de que existiera la distinción: cambiarlo escalaría revisiones que nadie pidió
// escalar.
func DefaultReviewPolicy() ReviewPolicy {
	return ReviewPolicy{
		FixAuthorized:     true,
		MaxFixRounds:      DefaultMaxFixRounds,
		AutoFixSeverities: []Severity{SeverityCritical, SeverityHigh},
	}
}

// Resolve aplica la precedencia declarada en la especificación: los valores
// explícitos de la revisión ganan a la política del proyecto, y esta a los defectos
// del dominio. Un valor "no declarado" es el cero de su tipo, así que fijar cero
// rondas no es una forma de pedir cero: para eso está FixAuthorized.
func (p ReviewPolicy) Resolve(explicita ReviewPolicy) ReviewPolicy {
	resuelta := p
	if resuelta.MaxFixRounds <= 0 {
		resuelta.MaxFixRounds = DefaultMaxFixRounds
	}
	if len(resuelta.AutoFixSeverities) == 0 {
		resuelta.AutoFixSeverities = DefaultReviewPolicy().AutoFixSeverities
	}
	if explicita.MaxFixRounds > 0 {
		resuelta.MaxFixRounds = explicita.MaxFixRounds
	}
	if len(explicita.AutoFixSeverities) > 0 {
		resuelta.AutoFixSeverities = append([]Severity(nil), explicita.AutoFixSeverities...)
	}
	return resuelta
}
