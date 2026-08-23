package usecases_test

import (
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// repoDocs levanta un repositorio real sobre una base temporal. Los tests de
// este paquete ya usan la persistencia de verdad en vez de mocks: un test vale
// lo que vale su fixture, y aquí lo que se prueba es justamente la interacción
// con el upsert por clave de tópico.
func repoDocs(t *testing.T) (ports.MemoryRepository, ports.MemoryTopicQuerier, ports.MemorySeeder) {
	t.Helper()
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := persistence.NewMemoryRepository(db)
	topics, ok := repo.(ports.MemoryTopicQuerier)
	if !ok {
		t.Fatal("el repositorio debe implementar ports.MemoryTopicQuerier")
	}
	seeder, ok := repo.(ports.MemorySeeder)
	if !ok {
		t.Fatal("el repositorio debe implementar ports.MemorySeeder")
	}
	return repo, topics, seeder
}

func TestResolvePinnedDoc_DesdeLaMemoria(t *testing.T) {
	repo, topics, _ := repoDocs(t)

	if _, err := repo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Preference, Title: "Reglas",
		Content: "reglas del equipo", TopicKey: domain.TopicWorkRules,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := usecases.ResolvePinnedDoc(topics, "proj", domain.TopicWorkRules, "PLANTILLA")
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if !got.FromMemory {
		t.Error("debería reportar que el contenido viene de la memoria")
	}
	if got.Content != "reglas del equipo" {
		t.Errorf("contenido = %q", got.Content)
	}
}

func TestResolvePinnedDoc_CaeALaPlantilla(t *testing.T) {
	_, topics, _ := repoDocs(t)

	got, err := usecases.ResolvePinnedDoc(topics, "proj", domain.TopicConstitution, "PLANTILLA DE RESERVA")
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	if got.FromMemory {
		t.Error("no hay memoria: FromMemory debe ser false para poder avisar")
	}
	if got.Content != "PLANTILLA DE RESERVA" {
		t.Errorf("contenido = %q", got.Content)
	}
}

func TestResolvePinnedDoc_SinMemoriaNiPlantillaEsError(t *testing.T) {
	_, topics, _ := repoDocs(t)

	if _, err := usecases.ResolvePinnedDoc(topics, "proj", domain.TopicConstitution, ""); err == nil {
		t.Error("sin memoria ni plantilla debe ser un error: no hay nada que devolver")
	}
}

func TestResolvePinnedDoc_TopicsNil(t *testing.T) {
	got, err := usecases.ResolvePinnedDoc(nil, "proj", domain.TopicWorkRules, "PLANTILLA")
	if err != nil {
		t.Fatalf("con topics nil debe degradar a la plantilla, no fallar: %v", err)
	}
	if got.FromMemory || got.Content != "PLANTILLA" {
		t.Errorf("resultado inesperado: %+v", got)
	}
}

// ─── Importación y restauración ─────────────────────────────────────────────

func TestImportPinnedDoc_CreaCuandoFalta(t *testing.T) {
	_, topics, seeder := repoDocs(t)
	doc, _ := domain.PinnedDocByAlias("rules")

	res, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, "mis reglas")
	if err != nil {
		t.Fatalf("importar: %v", err)
	}
	if !res.Created {
		t.Error("esperaba Created=true sobre un documento inexistente")
	}

	got, _ := topics.ByTopicKey("proj", doc.TopicKey)
	if got == nil || got.Content != "mis reglas" {
		t.Errorf("no se guardó: %+v", got)
	}
}

func TestImportPinnedDoc_ReemplazaConservandoLaClave(t *testing.T) {
	_, topics, seeder := repoDocs(t)
	doc, _ := domain.PinnedDocByAlias("rules")

	if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, "versión uno"); err != nil {
		t.Fatalf("importar 1: %v", err)
	}
	res, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, "versión dos")
	if err != nil {
		t.Fatalf("importar 2: %v", err)
	}
	if res.Created {
		t.Error("la segunda importación reemplaza, no crea")
	}

	got, _ := topics.ByTopicKey("proj", doc.TopicKey)
	if got.Content != "versión dos" {
		t.Errorf("contenido = %q, esperaba la versión nueva", got.Content)
	}
	if got.TopicKey != doc.TopicKey {
		t.Errorf("se perdió la clave de tópico: %q — dejaría de ser un documento fijado", got.TopicKey)
	}
}

// TestImportPinnedDoc_VacioNoDestruyeElAnterior cubre el peor modo de fallo de
// esta capacidad (FR-040): un import fallido que borre lo que había.
func TestImportPinnedDoc_VacioNoDestruyeElAnterior(t *testing.T) {
	_, topics, seeder := repoDocs(t)
	doc, _ := domain.PinnedDocByAlias("rules")

	if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, "contenido valioso"); err != nil {
		t.Fatalf("importar: %v", err)
	}

	for _, vacio := range []string{"", "   \n\t  "} {
		if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, vacio); err == nil {
			t.Errorf("importar %q debe fallar", vacio)
		}
		got, _ := topics.ByTopicKey("proj", doc.TopicKey)
		if got == nil || got.Content != "contenido valioso" {
			t.Fatalf("un import fallido destruyó el documento anterior: %+v", got)
		}
	}
}

// TestImportPinnedDoc_SoloSecretosSeRechaza: si tras depurar no queda nada, es
// contenido vacío. Nunca puede dejar el documento en blanco.
func TestImportPinnedDoc_SoloSecretosSeRechaza(t *testing.T) {
	_, topics, seeder := repoDocs(t)
	doc, _ := domain.PinnedDocByAlias("rules")

	if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title,
		"<private>token supersecreto</private>"); err == nil {
		t.Error("contenido que queda vacío tras depurar debe rechazarse")
	}
	if got, _ := topics.ByTopicKey("proj", doc.TopicKey); got != nil {
		t.Errorf("no debió crearse nada: %+v", got)
	}
}

func TestImportPinnedDoc_IdenticoEsNoOp(t *testing.T) {
	_, topics, seeder := repoDocs(t)
	doc, _ := domain.PinnedDocByAlias("rules")

	if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, "mismo texto"); err != nil {
		t.Fatalf("importar: %v", err)
	}
	res, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, "mismo texto")
	if err != nil {
		t.Fatalf("reimportar: %v", err)
	}
	if !res.Unchanged {
		t.Error("importar contenido idéntico debe reportarse como sin cambios")
	}
}

// TestImportPinnedDoc_ClaveFueraDelCatalogo cubre FR-042: el catálogo es una
// comodidad, no un límite.
func TestImportPinnedDoc_ClaveFueraDelCatalogo(t *testing.T) {
	_, topics, seeder := repoDocs(t)

	if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj", "equipo:runbook",
		domain.Learning, "Runbook del equipo", "pasos de guardia"); err != nil {
		t.Fatalf("importar clave arbitraria: %v", err)
	}
	got, _ := topics.ByTopicKey("proj", "equipo:runbook")
	if got == nil || got.Content != "pasos de guardia" {
		t.Errorf("no se guardó bajo la clave arbitraria: %+v", got)
	}
}

// TestPinnedDocState comprueba el estado DERIVADO (data-model.md §3ter): se
// calcula comparando con la plantilla, no se almacena.
func TestPinnedDocState(t *testing.T) {
	_, topics, seeder := repoDocs(t)
	doc, _ := domain.PinnedDocByAlias("rules")

	if st := usecases.PinnedDocState(topics, "proj", doc.TopicKey, "PLANTILLA"); st.State != usecases.DocSinSembrar {
		t.Errorf("sin memoria: estado = %v, esperaba sin sembrar", st.State)
	}

	if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, "PLANTILLA"); err != nil {
		t.Fatalf("importar: %v", err)
	}
	if st := usecases.PinnedDocState(topics, "proj", doc.TopicKey, "PLANTILLA"); st.State != usecases.DocPorDefecto {
		t.Errorf("contenido igual a la plantilla: estado = %v, esperaba por defecto", st.State)
	}

	if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj", doc.TopicKey, doc.Type, doc.Title, "PLANTILLA\nmás cosas"); err != nil {
		t.Fatalf("importar: %v", err)
	}
	st := usecases.PinnedDocState(topics, "proj", doc.TopicKey, "PLANTILLA")
	if st.State != usecases.DocPersonalizado {
		t.Errorf("contenido distinto: estado = %v, esperaba personalizado", st.State)
	}
	if st.Lines != 2 {
		t.Errorf("líneas = %d, esperaba 2", st.Lines)
	}
	if strings.TrimSpace(st.UpdatedAt) == "" {
		t.Error("un documento existente debe reportar su fecha de modificación")
	}
}

// TestImportPinnedDoc_UsaLaViaInerte cubre FR-045: importar y restaurar
// comparten las garantías de la siembra. Sin esto, reemplazar la constitución
// publicaría el documento del equipo en el ADR externo de quien tenga la
// sincronización activada — el mismo agujero que la siembra ya cerró.
func TestImportPinnedDoc_UsaLaViaInerte(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := persistence.NewMemoryRepository(db)
	topics := repo.(ports.MemoryTopicQuerier)
	seeder := repo.(ports.MemorySeeder)

	// Ancla en una sesión activa: si la importación formara sinapsis, aquí
	// habría con qué enlazar.
	sess, err := persistence.StartSession(db, "proj")
	if err != nil {
		t.Fatalf("sesión: %v", err)
	}
	if _, err := repo.Insert(&domain.Memory{
		Project: "proj", SessionID: sess.ID, Type: domain.Learning,
		Title: "ancla", Content: "memoria previa",
	}); err != nil {
		t.Fatalf("insert ancla: %v", err)
	}

	var antes int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_relations WHERE project = ?`, "proj").Scan(&antes); err != nil {
		t.Fatalf("contar: %v", err)
	}

	doc, _ := domain.PinnedDocByAlias("constitution")
	if _, err := usecases.ImportPinnedDoc(seeder, topics, "proj",
		doc.TopicKey, doc.Type, doc.Title, "CONSTITUCIÓN DEL EQUIPO"); err != nil {
		t.Fatalf("importar: %v", err)
	}

	var despues int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_relations WHERE project = ?`, "proj").Scan(&despues); err != nil {
		t.Fatalf("contar: %v", err)
	}
	if despues != antes {
		t.Errorf("la importación creó %d relación(es); debe usar la vía inerte", despues-antes)
	}
}
