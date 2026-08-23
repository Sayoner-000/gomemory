package usecases_test

import (
	"fmt"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

const seccionFijada = "## Reglas de trabajo (memoria fijada)"

// reglasLargas simula el preámbulo real: muy por encima del extracto de 200
// caracteres con el que acota() recorta cualquier otra memoria.
var reglasLargas = strings.Repeat("Regla de trabajo del equipo que ocupa espacio. ", 30) + "\nMARCA-FINAL"

func builderConSemilla(t *testing.T, extra int) *usecases.Builder {
	t.Helper()
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	memRepo := persistence.NewMemoryRepository(db)
	sessRepo := persistence.NewSessionRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	seeder := memRepo.(ports.MemorySeeder)
	topics := memRepo.(ports.MemoryTopicQuerier)

	if _, err := usecases.SeedDefaults(seeder, topics, "proj", []usecases.Seed{{
		TopicKey: domain.TopicWorkRules, Type: domain.Preference,
		Title: "Reglas de trabajo del proyecto", Content: reglasLargas,
	}}); err != nil {
		t.Fatalf("sembrar: %v", err)
	}

	for i := 0; i < extra; i++ {
		if _, err := memRepo.Insert(&domain.Memory{
			Project: "proj", Type: domain.Learning,
			Title: fmt.Sprintf("ruido %d", i), Content: fmt.Sprintf("memoria de relleno %d", i),
		}); err != nil {
			t.Fatalf("insert ruido: %v", err)
		}
	}

	b := usecases.New(memRepo, sessRepo, relRepo, t.TempDir(), "proj")
	b.Topics = topics
	b.Budget = persistence.DefaultBudget
	return b
}

// FR-008: la sección de reglas es una excepción declarada al recorte por
// presupuesto, igual que los conflictos sin resolver. Con Budget=24000 (el
// default real), cualquier otra memoria llegaría cortada a 200 caracteres con
// un puntero `get_memory`; las reglas deben llegar enteras o el agente no las
// aplicaría.
func TestBuild_ReglasFijadasSeEmitenIntegras(t *testing.T) {
	b := builderConSemilla(t, 0)

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(out, seccionFijada) {
		t.Fatalf("falta la sección fijada.\n%s", primeras(out, 20))
	}
	if !strings.Contains(out, "MARCA-FINAL") {
		t.Error("el contenido llegó truncado: falta el final de las reglas")
	}

	seccion := out[strings.Index(out, seccionFijada):]
	if idx := strings.Index(seccion, "\n## "); idx != -1 {
		seccion = seccion[:idx]
	}
	if strings.Contains(seccion, "get_memory") {
		t.Error("la sección fijada no debe llevar puntero `get_memory`: se emite íntegra")
	}
}

// FR-007: posición estable, antes de las secciones por tipo.
func TestBuild_ReglasFijadasVanAlPrincipio(t *testing.T) {
	b := builderConSemilla(t, 3)

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	posFijada := strings.Index(out, seccionFijada)
	posAprendizajes := strings.Index(out, "## Aprendizajes Recientes")
	if posFijada == -1 {
		t.Fatal("falta la sección fijada")
	}
	if posAprendizajes != -1 && posFijada > posAprendizajes {
		t.Error("la sección fijada debe ir antes de las secciones por tipo")
	}
	if cab := strings.Index(out, "# Memoria del Proyecto"); posFijada < cab {
		t.Error("la sección fijada debe ir después del encabezado del documento")
	}
}

// FR-009: sin esto, las reglas aparecerían DOS veces — íntegras arriba y
// recortadas en preferencias. Este test solo puede pasar si ListMemories
// proyecta topic_key (FR-030): con la columna ausente la comparación por clave
// daría siempre falso.
func TestBuild_ReglasFijadasNoSeDuplicanEnPreferencias(t *testing.T) {
	b := builderConSemilla(t, 0)

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if n := strings.Count(out, "Reglas de trabajo del proyecto"); n != 0 {
		t.Errorf("el título de la semilla aparece %d vez/veces en el cuerpo; debe emitirse solo bajo su propia sección", n)
	}
	if strings.Contains(out, "## Preferencias del Usuario") {
		prefs := out[strings.Index(out, "## Preferencias del Usuario"):]
		if idx := strings.Index(prefs[1:], "\n## "); idx != -1 {
			prefs = prefs[:idx]
		}
		if strings.Contains(prefs, "MARCA-FINAL") || strings.Contains(prefs, "Regla de trabajo del equipo") {
			t.Error("las reglas fijadas se repitieron dentro de Preferencias del Usuario")
		}
	}
}

// FR-010: el modo índice es un índice PURO por contrato (feature 020, FR-032).
// La excepción de emisión íntegra no lo alcanza.
func TestBuild_ModoIndiceColapsaLasReglas(t *testing.T) {
	b := builderConSemilla(t, 0)
	b.IndexMode = true

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(out, seccionFijada) {
		t.Fatal("la sección debe seguir apareciendo, colapsada")
	}
	if strings.Contains(out, "MARCA-FINAL") {
		t.Error("en modo índice el cuerpo no puede emitirse: debe colapsar a un puntero")
	}
	if !strings.Contains(out, "get_memory") {
		t.Error("en modo índice la sección debe llevar el puntero `get_memory`")
	}
}

// FR-031: el fallo silencioso que este test cierra. La semilla se crea una vez,
// al principio; Build() lista por recencia con tope. Con suficientes memorias
// nuevas —los checkpoints se generan por turno— desaparecería sin error alguno.
func TestBuild_ReglasFijadasSobrevivenALaVentanaDeRecencia(t *testing.T) {
	b := builderConSemilla(t, 200)

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.Contains(out, seccionFijada) {
		t.Error("la semilla desapareció al acumularse memorias más recientes: la sección no puede depender de la ventana de recencia")
	}
	if !strings.Contains(out, "MARCA-FINAL") {
		t.Error("la sección apareció pero recortada")
	}
}

// Degradación limpia, mismo criterio que Graph/Counter/Recorder.
func TestBuild_SinTopicsOmiteLaSeccion(t *testing.T) {
	b := builderConSemilla(t, 0)
	b.Topics = nil

	out, err := b.Build()
	if err != nil {
		t.Fatalf("build con Topics nil no debe fallar: %v", err)
	}
	if strings.Contains(out, seccionFijada) {
		t.Error("sin Topics no hay forma de resolver la semilla: la sección debe omitirse")
	}
}

func primeras(s string, n int) string {
	ls := strings.Split(s, "\n")
	if len(ls) > n {
		ls = ls[:n]
	}
	return strings.Join(ls, "\n")
}
