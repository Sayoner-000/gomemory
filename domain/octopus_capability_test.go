package domain

import "testing"

// El default conservador es la pieza que hace segura la degradación (FR-035):
// un runtime que no declara nada NO obtiene delegación. Lo contrario — asumir
// capacidades ausentes — produciría recomendaciones que nadie puede ejecutar.
func TestRuntimeCapabilities_NormalizeConservador(t *testing.T) {
	var vacias RuntimeCapabilities
	n := vacias.Normalize()

	if n.Subagents {
		t.Error("sin declaración explícita no debe asumirse soporte de subagentes")
	}
	if n.Parallel {
		t.Error("sin declaración explícita no debe asumirse paralelismo")
	}
	if n.IsolatedContext {
		t.Error("sin declaración explícita no debe asumirse aislamiento de contexto")
	}
	if n.MaxParallel != 1 {
		t.Errorf("MaxParallel = %d, esperaba 1 sin paralelismo declarado", n.MaxParallel)
	}
}

func TestRuntimeCapabilities_NormalizeMaxParallel(t *testing.T) {
	casos := []struct {
		nombre   string
		entrada  RuntimeCapabilities
		wantMax  int
		wantPara bool
	}{
		{
			"paralelo declarado sin tope usa el default de Octopus",
			RuntimeCapabilities{Subagents: true, Parallel: true, MaxParallel: 0},
			DefaultMaxParallel, true,
		},
		{
			"tope negativo usa el default de Octopus",
			RuntimeCapabilities{Subagents: true, Parallel: true, MaxParallel: -5},
			DefaultMaxParallel, true,
		},
		{
			"tope declarado se respeta",
			RuntimeCapabilities{Subagents: true, Parallel: true, MaxParallel: 4},
			4, true,
		},
		{
			"sin paralelismo el tope efectivo es 1 aunque se declare mayor",
			RuntimeCapabilities{Subagents: true, Parallel: false, MaxParallel: 8},
			1, false,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			n := c.entrada.Normalize()
			if n.MaxParallel != c.wantMax {
				t.Errorf("MaxParallel = %d, esperaba %d", n.MaxParallel, c.wantMax)
			}
			if n.Parallel != c.wantPara {
				t.Errorf("Parallel = %v, esperaba %v", n.Parallel, c.wantPara)
			}
		})
	}
}

// Normalizar debe ser idempotente: normalizar dos veces da lo mismo que una.
func TestRuntimeCapabilities_NormalizeIdempotente(t *testing.T) {
	c := RuntimeCapabilities{Subagents: true, Parallel: true, MaxParallel: 0}
	una := c.Normalize()
	dos := una.Normalize()
	if una != dos {
		t.Errorf("Normalize no es idempotente: %+v vs %+v", una, dos)
	}
}
