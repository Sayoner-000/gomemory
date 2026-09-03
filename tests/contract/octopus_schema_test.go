package main

import (
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// La privacidad de la telemetría de Octopus es una propiedad del ESQUEMA, no una
// promesa de quien escribe el código: si la tabla no tiene ningún sitio donde
// quepa texto libre venido de contenido, no hay forma de que se cuele una
// transcripción, una credencial o razonamiento privado (INV-AAR-013, FR-047,
// SC-011).
//
// Esta prueba falla en cuanto alguien añada una columna de contenido. Ese es
// exactamente su propósito: la revisión humana se olvida, el esquema no.
func TestOctopusExecutions_EsquemaSinTextoLibreDeContenido(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT name, type FROM pragma_table_info('octopus_executions')`)
	if err != nil {
		t.Fatalf("leer esquema: %v", err)
	}
	defer rows.Close()

	// Lista blanca EXPLÍCITA de columnas. Añadir una obliga a declararla aquí y,
	// con ello, a justificar que no transporta contenido.
	permitidas := map[string]bool{
		"id": true, "project": true, "plan_id": true, "task_id": true,
		"task_class": true, "route": true, "reason_code": true,
		"parallel_group": true, "context_budget": true, "output_budget": true,
		"estimated_tokens": true, "decided_at": true, "status": true,
		"context_tokens": true, "output_tokens": true, "duration_ms": true,
		"quality": true, "reported_at": true,
	}

	// La garantía real se juega en las columnas TEXT: una columna INTEGER no
	// puede transportar una transcripción. Estas son las ÚNICAS de tipo texto
	// admitidas, y cada una es un identificador, un enum del dominio o una marca
	// de tiempo — ninguna acepta texto libre venido de contenido.
	textoPermitido := map[string]string{
		"project":        "identificador de proyecto",
		"plan_id":        "identificador de plan",
		"task_id":        "identificador de tarea",
		"task_class":     "catálogo extensible de clases",
		"route":          "enum domain.Route",
		"reason_code":    "catálogo CERRADO domain.Reason",
		"parallel_group": "identificador de grupo",
		"status":         "enum domain.ResultStatus",
		"quality":        "enum domain.Quality",
		"decided_at":     "marca de tiempo",
		"reported_at":    "marca de tiempo",
	}

	var vistas int
	for rows.Next() {
		var nombre, tipo string
		if err := rows.Scan(&nombre, &tipo); err != nil {
			t.Fatalf("scan: %v", err)
		}
		vistas++

		if !permitidas[nombre] {
			t.Errorf("columna %q (%s) no declarada: si transporta contenido rompe INV-AAR-013; "+
				"si no, decláralas en la lista blanca de esta prueba", nombre, tipo)
			continue
		}
		if strings.EqualFold(tipo, "TEXT") {
			if _, ok := textoPermitido[nombre]; !ok {
				t.Errorf("la columna de texto %q no está justificada: la telemetría solo admite "+
					"identificadores, enums y marcas de tiempo, nunca contenido libre", nombre)
			}
		}
	}

	// Control: si la tabla no existiera, el bucle no vería columnas y la prueba
	// pasaría sin comprobar nada.
	if vistas != len(permitidas) {
		t.Fatalf("la tabla tiene %d columnas y la lista blanca declara %d: la prueba no está "+
			"midiendo el esquema real", vistas, len(permitidas))
	}
}

// La migración es aditiva e idempotente: aplicarla dos veces no falla ni duplica.
func TestOctopusExecutions_MigracionIdempotente(t *testing.T) {
	root := t.TempDir()

	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("primera init: %v", err)
	}
	db.Close()

	db2, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("segunda init sobre la misma base: %v", err)
	}
	defer db2.Close()

	var n int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM octopus_executions`).Scan(&n); err != nil {
		t.Fatalf("la tabla debe existir tras reabrir: %v", err)
	}
	if n != 0 {
		t.Errorf("la tabla debería nacer vacía, tiene %d filas", n)
	}
}
