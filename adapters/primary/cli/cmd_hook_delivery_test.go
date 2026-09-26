package cli

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
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

func (r *registroDeEntregasFalso) Reset() error { r.porCanal = nil; return nil }

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

// bloquesFalsos es un registro de bloques entregados en memoria.
type bloquesFalsos struct{ vistos map[string]bool }

func (b *bloquesFalsos) Seen(_ context.Context, hs []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, h := range hs {
		if b.vistos[h] {
			out[h] = true
		}
	}
	return out, nil
}

func (b *bloquesFalsos) Mark(_ context.Context, hs []string) error {
	if b.vistos == nil {
		b.vistos = map[string]bool{}
	}
	for _, h := range hs {
		b.vistos[h] = true
	}
	return nil
}

func (b *bloquesFalsos) Reset(context.Context) error { b.vistos = nil; return nil }

func historialConDecisiones(relleno int) string {
	return "# Memoria del Proyecto\n\n## Decisiones Técnicas\n\n- **A**: uno " + strings.Repeat("x", relleno) +
		"\n- **B**: dos\n\n## Bugfixes\n\n- **C**: tres\n"
}

// C-002 (acr_961a1676) — un contexto de arranque mayor que lo que el host
// inyecta entero llega truncado: no puede darse por entregado, o get_context
// devolvería marcadores de lo que el agente nunca leyó.
func TestEntregaContextoDeArranque_SalidaTruncadaNoSeDaPorEntregada(t *testing.T) {
	historial := historialConDecisiones(domain.HookInlineContextMaxChars)
	registro := &registroDeEntregasFalso{}
	bloques := &bloquesFalsos{}
	deps := &Deps{ContextBuilder: contextBuilderFalso{salida: historial}, DeliveryLog: registro,
		DeliveredBlocks: bloques, CompressionLevel: ports.CompressionMax}

	if got := entregaContextoDeArranque(deps); got != historial {
		t.Fatalf("el hook debe seguir emitiendo el contexto")
	}
	if _, ok := registro.Last(ports.DeliveryContext); ok {
		t.Error("una salida truncada no debe anotarse en el registro por canal")
	}
	if len(bloques.vistos) != 0 {
		t.Errorf("una salida truncada no debe marcar bloques como entregados: %d", len(bloques.vistos))
	}
	if got := deliverContextDoc(deps, historial); strings.Contains(got, "ya entregad") {
		t.Errorf("get_context debe entregar completo lo que el arranque no pudo entregar:\n%s", got)
	}
}

// C-002 (acr_961a1676) — lo ya entregado se sustituye por marcadores con una
// vía real de recuperación, y full=true la cumple: entrega todo.
func TestDeliverContextDocFull_RecuperaLoYaEntregado(t *testing.T) {
	historial := historialConDecisiones(10)
	deps := &Deps{ContextBuilder: contextBuilderFalso{salida: historial}, DeliveredBlocks: &bloquesFalsos{},
		CompressionLevel: ports.CompressionMax}

	if got := entregaContextoDeArranque(deps); got != historial {
		t.Fatalf("el arranque debe entregar el contexto completo")
	}
	delta := deliverContextDocFull(deps, historial, false)
	if !strings.Contains(delta, "2 entradas ya entregadas") || !strings.Contains(delta, "get_context(full=true)") {
		t.Errorf("el delta debe decir cómo recuperar lo sustituido:\n%s", delta)
	}
	if got := deliverContextDocFull(deps, historial, true); got != historial {
		t.Errorf("full=true debe entregar el documento completo:\n%s", got)
	}
}

// A-R2-01 (acr_df09a036) — full=true es la vía de recuperación de quien no
// tiene el contexto (un subagente, sobre todo). No anota nada en la sesión
// compartida: si lo hiciera, el agente principal recibiría marcadores de lo que
// solo vio truncado. Mismo criterio que get_plan_context(full=true).
func TestDeliverContextDocFull_NoAnotaEnLaSesionCompartida(t *testing.T) {
	historial := historialConDecisiones(domain.HookInlineContextMaxChars)
	bloques := &bloquesFalsos{}
	deps := &Deps{ContextBuilder: contextBuilderFalso{salida: historial}, DeliveredBlocks: bloques,
		CompressionLevel: ports.CompressionMax}

	entregaContextoDeArranque(deps) // truncado: no se anota
	if got := deliverContextDocFull(deps, historial, true); got != historial {
		t.Fatalf("full=true debe entregar el documento completo")
	}
	if len(bloques.vistos) != 0 {
		t.Errorf("full=true no debe marcar bloques en la sesión compartida: %d", len(bloques.vistos))
	}
	if got := deliverContextDoc(deps, historial); strings.Contains(got, "ya entregad") {
		t.Errorf("tras un full=true ajeno, el agente principal debe seguir recibiendo todo:\n%s", got)
	}
}

// C-001 (acr_6793454b) — el tope del canal se mide en caracteres: un contexto
// con acentos que cabe en 10 000 caracteres, aunque pase de 10 000 bytes, sí
// llegó entero y se anota como entregado.
func TestEntregaContextoDeArranque_TopeEnCaracteresNoEnBytes(t *testing.T) {
	historial := "# Memoria del Proyecto\n\n## Decisiones Técnicas\n\n- **A**: " + strings.Repeat("ñ", 6000) + "\n"
	if len(historial) <= domain.HookInlineContextMaxChars || utf8.RuneCountInString(historial) > domain.HookInlineContextMaxChars {
		t.Fatalf("control: se necesita un texto de más de 10 000 bytes y menos de 10 000 caracteres")
	}
	registro := &registroDeEntregasFalso{}
	entregaContextoDeArranque(&Deps{ContextBuilder: contextBuilderFalso{salida: historial}, DeliveryLog: registro})
	if _, ok := registro.Last(ports.DeliveryContext); !ok {
		t.Error("un contexto que cabe en caracteres debe anotarse como entregado")
	}
}
