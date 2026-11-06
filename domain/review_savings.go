package domain

// SinceSavings es el ahorro estimado de una revisión enlazada (HCD-035): un
// proxy de entrada, no el consumo real de un modelo. AvoidedTokens puede ser
// negativo si la lista de heredados pesa más que lo que se deja de releer.
type SinceSavings struct {
	Measurable      bool
	Reason          string
	ScopeFiles      int
	DeltaFiles      int
	ScopeTokens     int
	DeltaTokens     int
	InheritedTokens int
	AvoidedTokens   int
}

// MeasureSinceSavings compara el alcance completo (tokens por archivo) con la
// parte que está en el delta y descuenta lo que el revisor recibe de más.
func MeasureSinceSavings(fileTokens map[string]int, delta []string, inheritedTokens int) SinceSavings {
	changed := make(map[string]bool, len(delta))
	for _, d := range delta {
		changed[normalizeReviewPath(d)] = true
	}
	s := SinceSavings{Measurable: true, InheritedTokens: inheritedTokens}
	for file, tokens := range fileTokens {
		s.ScopeFiles++
		s.ScopeTokens += tokens
		if changed[normalizeReviewPath(file)] {
			s.DeltaFiles++
			s.DeltaTokens += tokens
		}
	}
	s.AvoidedTokens = s.ScopeTokens - s.DeltaTokens - s.InheritedTokens
	return s
}
