package persistence

import (
	"testing"
)

var tablasCompresion = []string{"compression_originals", "delivered_blocks", "compression_stats", "compression_tuning"}

// T013 — La migración crea las 4 tablas de la feature 033 desde una base vacía
// y desde una base con el esquema de la v2.25.0 (sin ellas), y es idempotente.
func TestMigrateCompression_BaseVaciaYAnterior(t *testing.T) {
	db := openTestDB(t) // base vacía → Open → migrate
	comprobar := func(etapa string) {
		for _, tabla := range tablasCompresion {
			var n int
			if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tabla).Scan(&n); err != nil || n != 1 {
				t.Fatalf("%s: falta la tabla %s (n=%d, err=%v)", etapa, tabla, n, err)
			}
		}
	}
	comprobar("base vacía")

	// Base de la v2.25.0: el mismo esquema sin las tablas nuevas, con una
	// memoria dentro que no debe perderse.
	if _, err := db.Exec(`INSERT INTO memories (project, type, title, content) VALUES ('p','learning','t','c')`); err != nil {
		t.Fatal(err)
	}
	for _, tabla := range tablasCompresion {
		if _, err := db.Exec(`DROP TABLE ` + tabla); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ { // dos veces: idempotente
		if err := migrate(db); err != nil {
			t.Fatalf("migrate %d: %v", i, err)
		}
	}
	comprobar("base v2.25.0")
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE title='t'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("la migración perdió datos previos: n=%d err=%v", n, err)
	}
}
