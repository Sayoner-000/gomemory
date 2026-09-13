//go:build calibration

package main

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	"mem/application/usecases"
	"mem/domain"
)

func TestSaveGateCalibration(t *testing.T) {
	path := os.Getenv("GOMEMORY_CALIBRATION_DB")
	if path == "" {
		t.Skip("GOMEMORY_CALIBRATION_DB no está definido")
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.Query(`SELECT id, type, title, content, COALESCE(topic_key,'') FROM memories WHERE type <> 'checkpoint' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var mems []domain.Memory
	for rows.Next() {
		var m domain.Memory
		var typ string
		if err := rows.Scan(&m.ID, &typ, &m.Title, &m.Content, &m.TopicKey); err != nil {
			t.Fatal(err)
		}
		m.Type = domain.MemoryType(typ)
		mems = append(mems, m)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	titles := []float64{0.5, 0.6, 0.7, 0.8, 0.9, 1.0}
	bodies := []float64{0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.5}
	for _, titleT := range titles {
		for _, bodyT := range bodies {
			warnings, pair := 0, false
			for i, m := range mems {
				for _, prev := range mems[:i] {
					if m.Type != prev.Type {
						continue
					}
					title, body := usecases.GateSimilarity(m, prev)
					if title >= titleT || body >= bodyT {
						warnings++
						if (m.ID == 207 && prev.ID == 209) || (m.ID == 209 && prev.ID == 207) {
							pair = true
						}
					}
				}
			}
			t.Logf("title=%.2f body=%.2f warnings=%d (%.1f%%) pair207/209=%t", titleT, bodyT, warnings, 100*float64(warnings)/float64(len(mems)), pair)
		}
	}
	t.Logf("duplicate groups: %#v", usecases.DetectDuplicateGroups(mems, 0.09))
}
