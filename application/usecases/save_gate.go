package usecases

import (
	"sort"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

const (
	// Medición del 2026-09-13: 4/82 avisos (4.9 %) y recall de 207/209.
	gateTitleThreshold = 0.70
	gateBodyThreshold  = 0.25
	gateMaxCandidates  = 3
)

type NearDuplicate struct {
	ID                int64
	Title, TopicKey   string
	TitleSim, BodySim float64
}

type GateResult struct {
	Checked, Updated bool
	Reason           string
	Candidates       []NearDuplicate
}

type gateStore interface {
	Insert(*domain.Memory) (int64, error)
	ports.MemoryFullLister
}

// GateSimilarity compara el título y el texto completo con la misma
// tokenización usada por la detección de duplicados.
func GateSimilarity(a, b domain.Memory) (title, body float64) {
	title = jaccardSimilarity(tokenize(a.Title), tokenize(b.Title))
	body = jaccardSimilarity(tokenize(a.Title+" "+a.Content), tokenize(b.Title+" "+b.Content))
	return title, body
}

// GateSizeCompatible descarta pares cuyo máximo Jaccard posible no alcanza t.
func GateSizeCompatible(na, nb int, t float64) bool {
	if na <= 0 || nb <= 0 {
		return false
	}
	if na > nb {
		na, nb = nb, na
	}
	return float64(na)/float64(nb) >= t
}

func CheckNearDuplicates(snapshot []domain.Memory, saved domain.Memory, savedID int64) GateResult {
	for _, m := range snapshot {
		if m.ID == savedID {
			return GateResult{Checked: true, Updated: true}
		}
	}
	result := GateResult{Checked: true}
	if saved.Type == domain.Checkpoint {
		return result
	}
	// La instantánea guarda el texto ya redactado (insertMemory); la memoria
	// nueva se compara igual, o un bloque privado movería el Jaccard. La nota
	// de impacto se descuenta en los dos lados, también si viene pegada aquí.
	title := domain.RedactSecrets(domain.RedactPrivate(saved.Title))
	content := domain.RedactSecrets(domain.RedactPrivate(domain.StripImpactAnnotation(saved.Content)))
	titleTokens := tokenize(title)
	bodyTokens := tokenize(title + " " + content)
	savedKey := strings.TrimSpace(saved.TopicKey)
	for _, m := range snapshot {
		if m.ID == savedID || m.Type == domain.Checkpoint || m.Type != saved.Type {
			continue
		}
		// La misma topic_key ya resuelve la identidad: no hay duplicado que
		// sugerir, con o sin upsert de por medio (S-001).
		if savedKey != "" && strings.TrimSpace(m.TopicKey) == savedKey {
			continue
		}
		candidateTitle := tokenize(m.Title)
		// La instantánea lleva la nota de impacto que insertMemory añade a las
		// memorias de hotspots; la memoria nueva todavía no. Se descuenta para
		// que no diluya el Jaccard de cuerpos cortos.
		candidateBody := tokenize(m.Title + " " + domain.StripImpactAnnotation(m.Content))
		titlePossible := GateSizeCompatible(len(titleTokens), len(candidateTitle), gateTitleThreshold)
		bodyPossible := GateSizeCompatible(len(bodyTokens), len(candidateBody), gateBodyThreshold)
		if !titlePossible && !bodyPossible {
			continue
		}
		title, body := 0.0, 0.0
		if titlePossible {
			title = jaccardSimilarity(titleTokens, candidateTitle)
		}
		if bodyPossible {
			body = jaccardSimilarity(bodyTokens, candidateBody)
		}
		if title >= gateTitleThreshold || body >= gateBodyThreshold {
			result.Candidates = append(result.Candidates, NearDuplicate{ID: m.ID, Title: m.Title, TopicKey: m.TopicKey, TitleSim: title, BodySim: body})
		}
	}
	sort.Slice(result.Candidates, func(i, j int) bool {
		a, b := result.Candidates[i], result.Candidates[j]
		if a.BodySim != b.BodySim {
			return a.BodySim > b.BodySim
		}
		if a.TitleSim != b.TitleSim {
			return a.TitleSim > b.TitleSim
		}
		return a.ID < b.ID
	})
	if len(result.Candidates) > gateMaxCandidates {
		result.Candidates = result.Candidates[:gateMaxCandidates]
	}
	return result
}

func SaveWithGate(store gateStore, m *domain.Memory) (int64, GateResult, error) {
	snapshot, listErr := store.ListAll(m.Project)
	id, err := store.Insert(m)
	if err != nil {
		return 0, GateResult{}, err
	}
	if listErr != nil {
		return id, GateResult{Reason: "no se pudieron leer las memorias: " + listErr.Error()}, nil
	}
	return id, CheckNearDuplicates(snapshot, *m, id), nil
}
