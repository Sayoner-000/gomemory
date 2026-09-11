package usecases

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// captureLearningsMaxItems acota cuántos ítems de una sola respuesta se
// persisten: un subagente verboso no debe inundar la memoria (research.md R9).
const captureLearningsMaxItems = 10

// captureLearningTitleMaxLen es el largo del título derivado del ítem.
const captureLearningTitleMaxLen = 80

// CaptureLearnings extrae los ítems de la sección de aprendizajes del mensaje
// final de un subagente (feature 030, US3) y los guarda como memorias tipo
// learning, sin acción del agente. Devuelve cuántos ítems se procesaron (no
// necesariamente cuántas filas NUEVAS resultaron: InsertMemory ya consolida
// por topic_key — spec 008, FR-013 — así que un ítem repetido actualiza la
// entrada existente en vez de duplicarla).
func CaptureLearnings(mems ports.MemoryRepository, sessions ports.SessionQuerier, project, text string) (int, error) {
	items := domain.ExtractLearnings(text)
	if len(items) == 0 {
		return 0, nil
	}
	if len(items) > captureLearningsMaxItems {
		items = items[:captureLearningsMaxItems]
	}

	sessionID := ""
	if sessions != nil {
		active, err := sessions.Active(project)
		if err != nil {
			return 0, err
		}
		if active != nil {
			sessionID = active.ID
		}
	}

	for _, item := range items {
		title := item
		if r := []rune(title); len(r) > captureLearningTitleMaxLen {
			title = string(r[:captureLearningTitleMaxLen])
		}
		m := &domain.Memory{
			Project:   project,
			SessionID: sessionID,
			Type:      domain.Learning,
			Title:     title,
			Content:   item + "\n\nProcedencia: subagente",
			TopicKey:  passiveTopicKey(item),
		}
		if _, err := mems.Insert(m); err != nil {
			return 0, err
		}
	}
	return len(items), nil
}

// passiveTopicKey deriva la clave de tópico de un ítem de aprendizaje
// capturado pasivamente: normaliza (minúsculas, espacios colapsados, sin
// puntuación final) antes de hashear, para que el mismo aprendizaje con
// distinto formato caiga en el mismo tópico y se consolide en InsertMemory en
// vez de duplicarse.
func passiveTopicKey(item string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(item), " "))
	normalized = strings.TrimRight(normalized, ".,;: ")
	sum := sha256.Sum256([]byte(normalized))
	return "passive:" + hex.EncodeToString(sum[:])[:16]
}
