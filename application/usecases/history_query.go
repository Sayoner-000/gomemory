package usecases

import (
	"sort"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// HistoryQuery lista las fichas del historial de decisiones (HCD-046) en
// orden cronológico, filtradas por texto o por archivo enlazado.
type HistoryQuery struct {
	Reader  ports.HistoryReader
	Project string
}

// HistoryItem es una ficha en el listado.
type HistoryItem struct {
	ID       int64  `json:"id"`
	Topic    string `json:"topic_key"`
	Date     string `json:"date"`
	Title    string `json:"title"`
	Filepath string `json:"filepath"`
}

// List devuelve las fichas; text filtra sin distinguir mayúsculas sobre título
// y contenido, y file por archivo enlazado. Ambos vacíos devuelven todo.
func (q *HistoryQuery) List(text, file string) ([]HistoryItem, error) {
	var mems []domain.Memory
	var err error
	if file = strings.TrimSpace(file); file != "" {
		mems, err = q.Reader.DigestsByFile(q.Project, file)
	} else {
		mems, err = q.Reader.Digests(q.Project)
	}
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(text))
	items := []HistoryItem{}
	for _, m := range mems {
		if needle != "" && !strings.Contains(strings.ToLower(m.Title+"\n"+m.Content), needle) {
			continue
		}
		items = append(items, HistoryItem{ID: m.ID, Topic: m.TopicKey, Date: domain.HistoryDigestDate(m), Title: m.Title, Filepath: m.Filepath})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Date != items[j].Date {
			return items[i].Date < items[j].Date
		}
		return items[i].Topic < items[j].Topic
	})
	return items, nil
}
