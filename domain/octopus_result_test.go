package domain

import "strings"

import "testing"

// T058 — AC-013: un resultado que excede el presupuesto de integración se reduce
// preservando lo que las tareas posteriores necesitan y descartando el relleno.
// La transcripción completa NUNCA se integra (INV-AAR-012).
func TestDelegatedResult_Compactar(t *testing.T) {
	r := DelegatedResult{
		TaskID:  "T004",
		Status:  StatusCompleted,
		Summary: strings.Repeat("relleno conversacional que no aporta. ", 200),
		Evidence: []string{
			"expiration.go:42 toma el lock después de leer",
			"store.go:88 lo toma antes",
		},
		AffectedSymbols: []string{"Expire", "Refresh"},
		Artifacts:       []string{"internal/memory/expiration.go"},
		Unresolved:      []string{"¿el refresco puede correr sin sesión activa?"},
	}

	compacto := r.Compactar(200)

	if compacto.TokensAprox() > 200 {
		t.Errorf("el resultado compactado sigue excediendo el presupuesto: %d", compacto.TokensAprox())
	}
	// Lo que las tareas posteriores necesitan sobrevive entero.
	if len(compacto.Evidence) != len(r.Evidence) {
		t.Error("la evidencia no debe descartarse al compactar")
	}
	if len(compacto.Artifacts) != len(r.Artifacts) {
		t.Error("los artefactos modificados no deben descartarse")
	}
	if len(compacto.Unresolved) != len(r.Unresolved) {
		t.Error("las preguntas sin resolver no deben descartarse")
	}
	if compacto.Status != r.Status {
		t.Error("el estado debe conservarse")
	}
	// El relleno sí se recorta.
	if len(compacto.Summary) >= len(r.Summary) {
		t.Error("el resumen debería recortarse cuando excede el presupuesto")
	}
}

// Un resultado que ya cabe no se toca: compactar no es reescribir.
func TestDelegatedResult_CompactarNoTocaLoQueYaCabe(t *testing.T) {
	r := DelegatedResult{TaskID: "T004", Status: StatusCompleted, Summary: "conclusión breve"}

	if got := r.Compactar(1000); got.Summary != r.Summary {
		t.Errorf("un resultado que cabe no debe alterarse: %q", got.Summary)
	}
}

// Un presupuesto no declarado no compacta nada.
func TestDelegatedResult_CompactarSinPresupuesto(t *testing.T) {
	r := DelegatedResult{TaskID: "T004", Summary: strings.Repeat("x", 5000)}
	if got := r.Compactar(0); len(got.Summary) != 5000 {
		t.Error("sin presupuesto declarado no debe recortarse nada")
	}
}

// --- Historia 7: fallos acotados ---

// T086 — AC-011: con tope de un reintento, el segundo fallo NO produce otro.
func TestNextAfterFailure_ReintentosAcotados(t *testing.T) {
	casos := []struct {
		nombre string
		estado AttemptState
		want   FailurePolicy
	}{
		{"primer fallo", AttemptState{Retries: 0, ParentCanDoIt: true}, PolicyRetry},
		{"segundo fallo, el padre puede", AttemptState{Retries: 1, ParentCanDoIt: true}, PolicyFallbackInline},
		{"segundo fallo, el padre no puede", AttemptState{Retries: 1}, PolicyAbort},
		{"muchos fallos no reabren el reintento", AttemptState{Retries: 9, ParentCanDoIt: true}, PolicyFallbackInline},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := NextAfterFailure(StatusFailed, c.estado, PolicyOverrides{}); got != c.want {
				t.Errorf("NextAfterFailure = %q, esperaba %q", got, c.want)
			}
		})
	}
}

// T087 — AC-012: contexto insuficiente admite UNA ampliación acotada. La segunda
// vez, el problema no es el presupuesto: la tarea no estaba bien acotada.
func TestNextAfterFailure_UnaSolaAmpliacionDeContexto(t *testing.T) {
	casos := []struct {
		nombre string
		estado AttemptState
		want   FailurePolicy
	}{
		{"primera vez", AttemptState{Expansions: 0}, PolicyExpandContext},
		{"segunda vez, el padre puede", AttemptState{Expansions: 1, ParentCanDoIt: true}, PolicyFallbackInline},
		{"segunda vez, el padre no puede", AttemptState{Expansions: 1}, PolicyAbort},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := NextAfterFailure(StatusInsufficientContext, c.estado, PolicyOverrides{}); got != c.want {
				t.Errorf("NextAfterFailure = %q, esperaba %q", got, c.want)
			}
		})
	}
}

// El tope de reintentos del llamador puede ser MÁS restrictivo, nunca más laxo.
func TestNextAfterFailure_TopeDelLlamador(t *testing.T) {
	sinReintentos := PolicyOverrides{MaxRetries: 0}
	if got := NextAfterFailure(StatusFailed, AttemptState{ParentCanDoIt: true}, sinReintentos); got != PolicyRetry {
		t.Errorf("con MaxRetries=0 (sin opinión) debe usarse el de fábrica: %q", got)
	}

	// Un tope explícito por encima del de fábrica no lo relaja.
	laxo := PolicyOverrides{MaxRetries: 99}
	if got := NextAfterFailure(StatusFailed, AttemptState{Retries: 1, ParentCanDoIt: true}, laxo); got != PolicyFallbackInline {
		t.Errorf("un tope del llamador no puede superar el de fábrica: %q", got)
	}
}

// T088 — FR-043: el resultado parcial se entrega solo si aporta algo.
func TestDelegatedResult_ConservaResultadoParcial(t *testing.T) {
	casos := []struct {
		nombre string
		r      DelegatedResult
		want   bool
	}{
		{"con evidencia", DelegatedResult{Evidence: []string{"a.go:1"}}, true},
		{"con artefactos", DelegatedResult{Artifacts: []string{"a.go"}}, true},
		{"con pendientes", DelegatedResult{Unresolved: []string{"¿y esto?"}}, true},
		// A-R001 (re-juicio ACR 027): Missing es justo el campo que más importa
		// en insufficient_context, y quedó fuera la primera vez.
		{"con faltante", DelegatedResult{Missing: []string{"el archivo b.go"}}, true},
		{"solo prosa", DelegatedResult{Summary: "no llegué a nada"}, false},
		{"vacío", DelegatedResult{}, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := c.r.ConservaResultadoParcial(); got != c.want {
				t.Errorf("ConservaResultadoParcial = %v, esperaba %v", got, c.want)
			}
		})
	}
}
