package usecases

import (
	"errors"
	"strings"
	"testing"
)

// fakeContextBuilder es el doble de prueba del constructor de contexto. Permite
// ejercer las tres ramas de BuildPlanContext sin tocar disco ni base de datos:
// la aplicación no debe depender de infraestructura para ser comprobable
// (constitución, principio I).
type fakeContextBuilder struct {
	out    string
	err    error
	builds int
}

func (f *fakeContextBuilder) Build() (string, error) {
	f.builds++
	return f.out, f.err
}

func (f *fakeContextBuilder) WriteFile() error { return nil }

const metodoDePrueba = "# Método de prueba\n\nContenido del método."

func TestBuildPlanContext_RamaCompleta(t *testing.T) {
	ctxb := &fakeContextBuilder{out: "# Memoria del Proyecto\n\nDecisiones previas."}
	pc := NewPlanContext(metodoDePrueba, ctxb)

	got, err := pc.Build(false)
	if err != nil {
		t.Fatalf("Build devolvió error: %v", err)
	}
	if !strings.Contains(got, "Contenido del método.") {
		t.Error("la rama completa debe incluir el método")
	}
	if !strings.Contains(got, "Decisiones previas.") {
		t.Error("la rama completa debe incluir el contexto del proyecto")
	}
}

// TestBuildPlanContext_RamaDegradada cubre FR-034: la ausencia de historial es
// una circunstancia, no una preferencia. Sin contexto, el método debe seguir
// llegando — es lo que mantiene la Historia 2 independiente de la Historia 1.
func TestBuildPlanContext_RamaDegradada_SoloMetodo(t *testing.T) {
	ctxb := &fakeContextBuilder{err: errors.New("memoria no inicializada")}
	pc := NewPlanContext(metodoDePrueba, ctxb)

	got, err := pc.Build(false)
	if err != nil {
		t.Fatalf("un fallo al construir el contexto NO debe propagarse como error: %v", err)
	}
	if !strings.Contains(got, "Contenido del método.") {
		t.Error("la rama degradada debe seguir emitiendo el método")
	}
	if strings.Contains(got, "Memoria del Proyecto") {
		t.Error("la rama degradada no debe emitir sección de contexto")
	}
}

// TestBuildPlanContext_RamaSilenciada cubre FR-032: el apagado explícito SÍ es
// una preferencia, y debe silenciarlo todo — método incluido.
func TestBuildPlanContext_RamaSilenciada_SalidaVacia(t *testing.T) {
	ctxb := &fakeContextBuilder{out: "# Memoria del Proyecto\n\nDecisiones previas."}
	pc := NewPlanContext(metodoDePrueba, ctxb)

	got, err := pc.Build(true)
	if err != nil {
		t.Fatalf("Build devolvió error: %v", err)
	}
	if got != "" {
		t.Errorf("con la funcionalidad apagada la salida debe ser vacía, se obtuvo %q", got)
	}
	if ctxb.builds != 0 {
		t.Error("con la funcionalidad apagada no debe construirse el contexto (sin efectos secundarios)")
	}
}

// TestBuildPlanContext_MetodoPrecedeAlContexto cubre el invariante 5 del
// contrato: si el contexto se trunca por presupuesto, lo que se pierde es la
// cola del historial y nunca el método. Además el agente debe conocer las
// reglas antes que el material.
func TestBuildPlanContext_MetodoPrecedeAlContexto(t *testing.T) {
	ctxb := &fakeContextBuilder{out: "# Memoria del Proyecto\n\nDecisiones previas."}
	pc := NewPlanContext(metodoDePrueba, ctxb)

	got, err := pc.Build(false)
	if err != nil {
		t.Fatalf("Build devolvió error: %v", err)
	}
	iMetodo := strings.Index(got, "Contenido del método.")
	iCtx := strings.Index(got, "Memoria del Proyecto")
	if iMetodo < 0 || iCtx < 0 {
		t.Fatalf("faltan secciones en la salida: método=%d contexto=%d", iMetodo, iCtx)
	}
	if iMetodo > iCtx {
		t.Error("el método debe preceder al contexto en la salida")
	}
}

// TestBuildPlanContext_ContextoVacioNoDejaSeccionHuerfana evita emitir un
// separador y un encabezado sin cuerpo cuando el proyecto no tiene nada que
// contar todavía.
func TestBuildPlanContext_ContextoVacioNoDejaSeccionHuerfana(t *testing.T) {
	ctxb := &fakeContextBuilder{out: "   \n\n  "}
	pc := NewPlanContext(metodoDePrueba, ctxb)

	got, err := pc.Build(false)
	if err != nil {
		t.Fatalf("Build devolvió error: %v", err)
	}
	if strings.Contains(got, "---") {
		t.Errorf("con contexto vacío no debe emitirse separador, se obtuvo:\n%s", got)
	}
}

// TestBuildPlanContext_SinMetodo degrada con gracia si la plantilla embebida no
// pudo cargarse: se emite el contexto en vez de una salida rota.
func TestBuildPlanContext_SinMetodo_EmiteSoloContexto(t *testing.T) {
	ctxb := &fakeContextBuilder{out: "# Memoria del Proyecto\n\nDecisiones previas."}
	pc := NewPlanContext("", ctxb)

	got, err := pc.Build(false)
	if err != nil {
		t.Fatalf("Build devolvió error: %v", err)
	}
	if !strings.Contains(got, "Decisiones previas.") {
		t.Error("sin método debe emitirse al menos el contexto")
	}
	if strings.HasPrefix(got, "---") {
		t.Error("sin método no debe abrirse la salida con un separador huérfano")
	}
}
