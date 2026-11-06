package usecases

import (
	"mem/application/ports"
	"mem/domain"
)

// ReviewSinceSavings mide el ahorro de una revisión enlazada (HCD-035): los
// tokens del alcance en la revisión del objetivo frente a los del delta,
// descontando inheritedPayload, que es lo que el revisor recibe de más. Nunca
// falla: si no se puede medir, lo dice con el motivo.
func ReviewSinceSavings(files ports.RevisionFiles, counter ports.TokenCounter, review *domain.Review, inheritedPayload string) domain.SinceSavings {
	switch {
	case review == nil || review.SinceReviewID == "":
		return domain.SinceSavings{Reason: "la revisión no está enlazada con --since"}
	case review.DeltaSource != domain.DeltaSourceGit:
		return domain.SinceSavings{Reason: "el delta no salió de git: el objetivo no es un commit y no hay alcance comparable"}
	case files == nil || counter == nil:
		return domain.SinceSavings{Reason: "no hay lector de revisiones o contador de tokens configurado"}
	}
	scope := review.Target.Scope
	if len(scope) == 0 {
		scope = []string{"."}
	}
	contents, err := files.TextFilesAt(review.Target.Revision, scope)
	if err != nil {
		return domain.SinceSavings{Reason: "no se pudo leer el alcance en " + review.Target.Revision + ": " + err.Error()}
	}
	tokens := make(map[string]int, len(contents))
	for name, text := range contents {
		tokens[name] = counter.Count(text)
	}
	return domain.MeasureSinceSavings(tokens, review.DeltaScope, counter.Count(inheritedPayload))
}
