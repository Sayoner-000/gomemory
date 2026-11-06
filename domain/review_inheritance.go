package domain

import (
	"path"
	"slices"
	"strings"
)

// Revisión acumulativa (HCD-032): una revisión iniciada con --since hereda de
// la anterior lo ya asentado, para no volver a adjudicarlo, y examina solo el
// delta. Este archivo es la parte pura: qué se hereda y cuándo sigue valiendo.

// SettledFinding es un hallazgo de consenso de una revisión anterior que la
// nueva hereda como ya asentado.
type SettledFinding struct {
	ConsensusID  int64
	FromReviewID string
	Status       ConsensusStatus
	Severity     Severity
	Claim        string
	// Files son los archivos de las ubicaciones de sus hallazgos fuente: lo que
	// se compara con el delta para saber si el asentamiento sigue valiendo.
	Files []string
}

// InheritableFindings selecciona lo que una revisión terminada deja asentado
// (Q2): CONFIRMED con re-juicio RESOLVED vigente en su última ronda, y SUSPECT
// o INFO ya clasificados. Nunca hereda un CONFIRMED sin resolver ni una
// CONTRADICTION: eso sigue abierto y se revisa de nuevo.
func InheritableFindings(prev Review, consensus []ConsensusFinding, sources map[int64]Finding) []SettledFinding {
	var out []SettledFinding
	for _, c := range consensus {
		switch c.Status {
		case ConsensusConfirmed:
			if !c.ResueltoEn(prev.Round) {
				continue
			}
		case ConsensusSuspect, ConsensusInfo:
		default:
			continue
		}
		var files []string
		for _, id := range c.SourceFindingIDs {
			if f, ok := sources[id]; ok {
				if file := FileOfLocation(f.Location); file != "" && !slices.Contains(files, file) {
					files = append(files, file)
				}
			}
		}
		slices.Sort(files)
		out = append(out, SettledFinding{
			ConsensusID: c.ID, FromReviewID: prev.ID, Status: c.Status,
			Severity: c.Severity, Claim: c.Claim, Files: files,
		})
	}
	return out
}

// SettledStillValid dice si un asentado sigue valiendo frente al delta (Q4):
// solo si ninguno de sus archivos cambió. Un asentado sin archivos no se puede
// comprobar y se reabre: ante la duda, se revisa.
func SettledStillValid(s SettledFinding, delta []string) bool {
	if len(s.Files) == 0 {
		return false
	}
	changed := make(map[string]bool, len(delta))
	for _, d := range delta {
		changed[normalizeReviewPath(d)] = true
	}
	for _, f := range s.Files {
		if changed[normalizeReviewPath(f)] {
			return false
		}
	}
	return true
}

// FileOfLocation extrae la ruta de una ubicación de hallazgo: «ruta»,
// «ruta:línea» o «ruta:inicio-fin».
func FileOfLocation(loc string) string {
	loc = strings.TrimSpace(loc)
	if i := strings.LastIndex(loc, ":"); i > 0 {
		if rest := loc[i+1:]; rest != "" && strings.Trim(rest, "0123456789-") == "" {
			loc = loc[:i]
		}
	}
	return normalizeReviewPath(loc)
}

func normalizeReviewPath(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, `\`, "/"))
	if p == "" {
		return ""
	}
	return strings.TrimPrefix(path.Clean(p), "./")
}
