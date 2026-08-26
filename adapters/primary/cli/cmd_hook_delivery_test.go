package cli

import (
	"strings"
	"testing"

	"mem/application/ports"
	"mem/application/usecases"
)

// contextBuilderFalso devuelve un contexto fijo, sin tocar base de datos.
type contextBuilderFalso struct{ salida string }

func (c contextBuilderFalso) Build() (string, error) { return c.salida, nil }
func (c contextBuilderFalso) WriteFile() error       { return nil }

// registroDeEntregasFalso guarda en memoria lo que cada canal entregó.
type registroDeEntregasFalso struct{ porCanal map[string]string }

func (r *registroDeEntregasFalso) Last(kind string) (string, bool) {
	h, ok := r.porCanal[kind]
	return h, ok
}

func (r *registroDeEntregasFalso) Record(kind, hash string) error {
	if r.porCanal == nil {
		r.porCanal = map[string]string{}
	}
	r.porCanal[kind] = hash
	return nil
}

// TestEntregaContextoDeArranque_AnotaLaEntrega cubre la causa raíz de la doble
// entrega: el hook de arranque emitía el contexto sin anotarlo, así que la
// guarda de get_plan_context nunca tenía con qué comparar y el historial
// completo viajaba dos veces en la misma sesión.
func TestEntregaContextoDeArranque_AnotaLaEntrega(t *testing.T) {
	historial := "# Memoria del Proyecto\n\ncontenido histórico"
	registro := &registroDeEntregasFalso{}
	deps := &Deps{ContextBuilder: contextBuilderFalso{salida: historial}, DeliveryLog: registro}

	got := entregaContextoDeArranque(deps)

	if got != historial {
		t.Fatalf("el hook debe seguir emitiendo el contexto íntegro, got %q", got)
	}
	hash, ok := registro.Last(ports.DeliveryContext)
	if !ok {
		t.Fatal("la entrega del canal context no quedó anotada")
	}
	if hash != usecases.HashDeContenido(historial) {
		t.Fatalf("el hash anotado no corresponde al contexto emitido: %q", hash)
	}
}

// TestEntregaContextoDeArranque_SuprimeElReenvioEnModoPlan es la verificación de
// punta a punta del ahorro: tras el arranque, entrar en modo plan ya no repite
// el historial, lo sustituye por el aviso.
func TestEntregaContextoDeArranque_SuprimeElReenvioEnModoPlan(t *testing.T) {
	historial := "# Memoria del Proyecto\n\ncontenido histórico que no debe repetirse"
	registro := &registroDeEntregasFalso{}
	builder := contextBuilderFalso{salida: historial}

	entregaContextoDeArranque(&Deps{ContextBuilder: builder, DeliveryLog: registro})

	plan, err := usecases.NewPlanContext("método de descomposición", builder, registro).Build(false)
	if err != nil {
		t.Fatalf("plan context: %v", err)
	}

	if strings.Contains(plan, "contenido histórico que no debe repetirse") {
		t.Fatalf("el historial no debe reenviarse tras haberse entregado al arrancar:\n%s", plan)
	}
	if !strings.Contains(plan, "método de descomposición") {
		t.Fatalf("el método de planificación sí debe viajar siempre:\n%s", plan)
	}
	if !strings.Contains(plan, "ya está disponible en esta sesión") {
		t.Fatalf("la supresión debe declararse, nunca ser silenciosa:\n%s", plan)
	}
}

// TestEntregaContextoDeArranque_SinRegistroNoRompe: el puerto es opcional, igual
// que el resto de colaboradores de los hooks.
func TestEntregaContextoDeArranque_SinRegistroNoRompe(t *testing.T) {
	deps := &Deps{ContextBuilder: contextBuilderFalso{salida: "algo"}}
	if got := entregaContextoDeArranque(deps); got != "algo" {
		t.Fatalf("sin DeliveryLog el contexto debe emitirse igual, got %q", got)
	}
}
