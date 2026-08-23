package usecases_test

import (
	"errors"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

func semillasDePrueba() []usecases.Seed {
	return []usecases.Seed{
		{TopicKey: domain.TopicWorkRules, Type: domain.Preference, Title: "Reglas de trabajo del proyecto", Content: "REGLAS"},
		{TopicKey: domain.TopicConstitution, Type: domain.Architecture, Title: "Constitución del proyecto (spec-kit)", Content: "CONSTITUCIÓN"},
	}
}

// C3: sobre un proyecto vacío se siembran ambas.
func TestSeedDefaults_SiembraLoQueFalta(t *testing.T) {
	_, topics, seeder := repoDocs(t)

	created, err := usecases.SeedDefaults(seeder, topics, "proj", semillasDePrueba())
	if err != nil {
		t.Fatalf("sembrar: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("esperaba 2 semillas creadas, hubo %d: %v", len(created), created)
	}

	for _, s := range semillasDePrueba() {
		got, _ := topics.ByTopicKey("proj", s.TopicKey)
		if got == nil {
			t.Fatalf("no se sembró %s", s.TopicKey)
		}
		if got.Type != s.Type || got.Title != s.Title {
			t.Errorf("%s guardada con metadatos incorrectos: %+v", s.TopicKey, got)
		}
		if got.SessionID != "" {
			t.Errorf("%s: una semilla no pertenece a ninguna sesión, tiene %q", s.TopicKey, got.SessionID)
		}
		if got.Filepath != "" {
			t.Errorf("%s: una semilla no describe un archivo, tiene %q", s.TopicKey, got.Filepath)
		}
	}
}

// C2 + C6: la segunda llamada no toca nada. Es lo que hace idempotente a
// `mem install` y al arranque del servidor MCP.
func TestSeedDefaults_EsIdempotente(t *testing.T) {
	_, topics, seeder := repoDocs(t)

	if _, err := usecases.SeedDefaults(seeder, topics, "proj", semillasDePrueba()); err != nil {
		t.Fatalf("sembrar 1: %v", err)
	}
	created, err := usecases.SeedDefaults(seeder, topics, "proj", semillasDePrueba())
	if err != nil {
		t.Fatalf("sembrar 2: %v", err)
	}
	if len(created) != 0 {
		t.Errorf("la segunda siembra creó %v; no debía crear nada", created)
	}
}

// C2, el caso que más importa: la edición de la persona gana SIEMPRE, incluso
// frente a una plantilla nueva del binario. Sin esto, cada actualización
// pisaría las reglas que el equipo escribió (research.md §R5).
func TestSeedDefaults_NoPisaLaEdicionDelUsuario(t *testing.T) {
	_, topics, seeder := repoDocs(t)

	if _, err := usecases.SeedDefaults(seeder, topics, "proj", semillasDePrueba()); err != nil {
		t.Fatalf("sembrar: %v", err)
	}

	doc, _ := domain.PinnedDocByAlias("rules")
	if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, "REGLAS DEL EQUIPO"); err != nil {
		t.Fatalf("importar: %v", err)
	}

	// Cinco reinstalaciones con una plantilla distinta a la original.
	nuevas := []usecases.Seed{{
		TopicKey: domain.TopicWorkRules, Type: domain.Preference,
		Title: "Reglas de trabajo del proyecto", Content: "PLANTILLA NUEVA DEL BINARIO",
	}}
	for i := 0; i < 5; i++ {
		if _, err := usecases.SeedDefaults(seeder, topics, "proj", nuevas); err != nil {
			t.Fatalf("resiembra %d: %v", i, err)
		}
	}

	got, _ := topics.ByTopicKey("proj", domain.TopicWorkRules)
	if got.Content != "REGLAS DEL EQUIPO" {
		t.Errorf("la siembra pisó la edición del equipo: %q", got.Content)
	}
}

// C1: contenido vacío se omite en silencio en vez de crear una memoria vacía
// que el agente tomaría por buena.
func TestSeedDefaults_ContenidoVacioSeOmite(t *testing.T) {
	_, topics, seeder := repoDocs(t)

	created, err := usecases.SeedDefaults(seeder, topics, "proj", []usecases.Seed{
		{TopicKey: domain.TopicWorkRules, Type: domain.Preference, Title: "Reglas", Content: ""},
		{TopicKey: domain.TopicConstitution, Type: domain.Architecture, Title: "Constitución", Content: "   \n  "},
	})
	if err != nil {
		t.Fatalf("no debe fallar, solo omitir: %v", err)
	}
	if len(created) != 0 {
		t.Errorf("creó %v con contenido vacío", created)
	}
}

// C4: degradación limpia, mismo criterio nil-safe del resto del proyecto.
func TestSeedDefaults_TopicsNil(t *testing.T) {
	created, err := usecases.SeedDefaults(nil, nil, "proj", semillasDePrueba())
	if err != nil {
		t.Fatalf("con dependencias nil debe degradar sin error: %v", err)
	}
	if len(created) != 0 {
		t.Errorf("creó %v sin repositorio", created)
	}
}

// C5: un error de consulta en una semilla no impide intentar la otra.
func TestSeedDefaults_ErrorEnUnaNoAbortaLaOtra(t *testing.T) {
	_, topics, seeder := repoDocs(t)
	roto := &topicsQueFalla{real: topics, falla: domain.TopicWorkRules}

	created, err := usecases.SeedDefaults(seeder, roto, "proj", semillasDePrueba())
	if err == nil {
		t.Error("el error de lectura debe propagarse")
	}
	if len(created) != 1 || created[0] != domain.TopicConstitution {
		t.Errorf("la otra semilla debió sembrarse igual, created = %v", created)
	}
}

// TestSeedDefaults_ContenidoIdenticoAlOrigen es el guardián G1 (FR-032). Hoy
// las plantillas no contienen nada que active la depuración de secretos —se
// comprobaron los 6 patrones, 0 coincidencias—, pero una edición futura podría
// introducir una cadena que matchee y mutilaría la semilla EN SILENCIO. Este
// test lo convierte en un fallo de CI en vez de un `[REDACTED:...]` perdido
// entre 635 líneas.
func TestSeedDefaults_ContenidoIdenticoAlOrigen(t *testing.T) {
	_, topics, seeder := repoDocs(t)

	origen := strings.Join([]string{
		"# Reglas",
		"",
		"1. Una regla con `código`, acentos áéíóú y símbolos: <>&\"'",
		"2. Otra con una tabla | y | pipes |",
	}, "\n")

	semillas := []usecases.Seed{{
		TopicKey: domain.TopicWorkRules, Type: domain.Preference, Title: "Reglas", Content: origen,
	}}
	if _, err := usecases.SeedDefaults(seeder, topics, "proj", semillas); err != nil {
		t.Fatalf("sembrar: %v", err)
	}

	got, _ := topics.ByTopicKey("proj", domain.TopicWorkRules)
	if got.Content != origen {
		t.Errorf("el contenido persistido difiere del origen.\nesperado: %q\nobtenido: %q", origen, got.Content)
	}
}

// topicsQueFalla simula un fallo de lectura en una clave concreta.
type topicsQueFalla struct {
	real  ports.MemoryTopicQuerier
	falla string
}

func (t *topicsQueFalla) ByTopicKey(project, topicKey string) (*domain.Memory, error) {
	if topicKey == t.falla {
		return nil, errors.New("fallo simulado de lectura")
	}
	return t.real.ByTopicKey(project, topicKey)
}
