package usecases

import (
	"strings"
	"testing"

	"mem/application/ports"
)

type ctxFijo struct{ texto string }

func (c ctxFijo) Build() (string, error) { return c.texto, nil }
func (c ctxFijo) WriteFile() error       { return nil }

type logFalso struct {
	entregas map[string]string
	grabado  map[string]string
}

func nuevoLog(previas map[string]string) *logFalso {
	if previas == nil {
		previas = map[string]string{}
	}
	return &logFalso{entregas: previas, grabado: map[string]string{}}
}
func (l *logFalso) Last(kind string) (string, bool) { h, ok := l.entregas[kind]; return h, ok }
func (l *logFalso) Record(kind, hash string) error  { l.grabado[kind] = hash; return nil }

const historial = "## Memoria del Proyecto\n\nDecisiones y aprendizajes."

// TestPlanContext_NoRepiteLoYaEntregado cubre FR-006 y FR-007: si el contexto
// general ya entregó el historial en esta sesión, el contexto de planificación
// no lo reenvía, e indica dónde está.
func TestPlanContext_NoRepiteLoYaEntregado(t *testing.T) {
	log := nuevoLog(map[string]string{ports.DeliveryContext: HashDeContenido(historial)})
	p := NewPlanContext("MÉTODO", ctxFijo{historial}, log)

	out, err := p.Build(false)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if strings.Contains(out, "Decisiones y aprendizajes") {
		t.Error("reenvió el historial que ya se había entregado en esta sesión")
	}
	if !strings.Contains(out, "MÉTODO") {
		t.Error("no entregó el método, que es lo que sí aporta")
	}
	if !strings.Contains(strings.ToLower(out), "ya está disponible en esta sesión") {
		t.Errorf("no indica que se suprimió ni dónde está el material:\n%s", out)
	}
}

// TestPlanContext_SinEntregaPreviaEntregaTodo cubre FR-009: la reducción nunca
// puede dejar al agente sin contexto.
func TestPlanContext_SinEntregaPreviaEntregaTodo(t *testing.T) {
	p := NewPlanContext("MÉTODO", ctxFijo{historial}, nuevoLog(nil))

	out, _ := p.Build(false)
	if !strings.Contains(out, "Decisiones y aprendizajes") {
		t.Error("sin entrega previa debe entregar el historial completo")
	}
}

// TestPlanContext_SiCambioEntregaDeNuevo cubre FR-008: si el material cambió
// desde la entrega anterior, se entrega.
func TestPlanContext_SiCambioEntregaDeNuevo(t *testing.T) {
	log := nuevoLog(map[string]string{ports.DeliveryContext: HashDeContenido("otro contenido anterior")})
	p := NewPlanContext("MÉTODO", ctxFijo{historial}, log)

	out, _ := p.Build(false)
	if !strings.Contains(out, "Decisiones y aprendizajes") {
		t.Error("el historial cambió desde la entrega anterior y debe entregarse")
	}
}

// TestPlanContext_RegistraLoQueEntrega: lo entregado queda anotado, para que la
// siguiente operación de la sesión pueda decidir.
func TestPlanContext_RegistraLoQueEntrega(t *testing.T) {
	log := nuevoLog(nil)
	NewPlanContext("MÉTODO", ctxFijo{historial}, log).Build(false)

	if log.grabado[ports.DeliveryPlanContext] == "" {
		t.Error("no anotó lo que entregó")
	}
}

// TestPlanContext_SinRegistroFuncionaIgual: el registro es opcional. Un
// proyecto sin sesión activa recibe el documento completo.
func TestPlanContext_SinRegistroFuncionaIgual(t *testing.T) {
	out, err := NewPlanContext("MÉTODO", ctxFijo{historial}, nil).Build(false)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(out, "Decisiones y aprendizajes") || !strings.Contains(out, "MÉTODO") {
		t.Error("sin registro debe entregarse el documento completo")
	}
}
