package domain

import (
	"errors"
	"testing"
)

// Una unidad sin objetivo es inenrutable: INV-AAR-003 exige objetivo acotado,
// y un objetivo vacío no puede acotarse. Se valida en el borde, antes de decidir.
func TestWorkUnit_Validate(t *testing.T) {
	casos := []struct {
		nombre  string
		unidad  WorkUnit
		wantErr bool
	}{
		{"completa", WorkUnit{ID: "T001", Objective: "Implementar el modelo"}, false},
		{"sin identificador", WorkUnit{Objective: "Implementar el modelo"}, true},
		{"sin objetivo", WorkUnit{ID: "T001"}, true},
		{"objetivo en blanco", WorkUnit{ID: "T001", Objective: "   "}, true},
		{"identificador con espacios", WorkUnit{ID: "T 001", Objective: "algo"}, true},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			err := c.unidad.Validate()
			if c.wantErr && err == nil {
				t.Fatal("esperaba error de validación")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("no esperaba error: %v", err)
			}
			if c.wantErr && !errors.Is(err, ErrValidation) {
				t.Errorf("el error debería envolver ErrValidation, es %v", err)
			}
		})
	}
}

// La clasificación es extensible (FR-012): un valor desconocido es válido y no
// convierte la unidad en errónea. Enrutar nunca depende solo de la clase.
func TestTaskClass_ValorDesconocidoEsValido(t *testing.T) {
	u := WorkUnit{ID: "T001", Objective: "algo", Class: TaskClass("clase-que-nadie-previó")}

	if err := u.Validate(); err != nil {
		t.Fatalf("una clase desconocida no debería invalidar la unidad: %v", err)
	}
	if u.Class.Known() {
		t.Error("una clase fuera del catálogo no debería reportarse como conocida")
	}
	if !ClassInvestigation.Known() {
		t.Error("investigation sí pertenece al catálogo")
	}
}

// Dos unidades que escriben el mismo artefacto no pueden compartir grupo
// paralelo aunque no exista dependencia declarada entre ellas.
func TestScope_WritesIntersect(t *testing.T) {
	casos := []struct {
		nombre string
		a, b   Scope
		want   bool
	}{
		{
			"mismo archivo, ambas escriben",
			Scope{Files: []string{"a.go", "b.go"}},
			Scope{Files: []string{"b.go"}},
			true,
		},
		{
			"mismo archivo pero una es de solo lectura",
			Scope{Files: []string{"b.go"}, ReadOnly: true},
			Scope{Files: []string{"b.go"}},
			false,
		},
		{
			"ambas de solo lectura sobre el mismo archivo",
			Scope{Files: []string{"b.go"}, ReadOnly: true},
			Scope{Files: []string{"b.go"}, ReadOnly: true},
			false,
		},
		{
			"archivos distintos",
			Scope{Files: []string{"a.go"}},
			Scope{Files: []string{"b.go"}},
			false,
		},
		{"alcances vacíos", Scope{}, Scope{}, false},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.a.WritesIntersect(c.b); got != c.want {
				t.Errorf("WritesIntersect = %v, esperaba %v", got, c.want)
			}
			if got := c.b.WritesIntersect(c.a); got != c.want {
				t.Errorf("WritesIntersect no es simétrica: %v vs %v", got, c.want)
			}
		})
	}
}

func TestLevel_String(t *testing.T) {
	casos := map[Level]string{
		LevelTrivial: "trivial",
		LevelLow:     "low",
		LevelMedium:  "medium",
		LevelHigh:    "high",
	}
	for nivel, want := range casos {
		if got := nivel.String(); got != want {
			t.Errorf("Level(%d).String() = %q, esperaba %q", nivel, got, want)
		}
	}
}
