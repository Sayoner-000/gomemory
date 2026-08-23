package persistence

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"mem/domain"
)

// TestListMemories_DevuelveTopicKey cubre el defecto latente que la feature 021
// corrige (research.md §R2, FR-030): ListMemories no proyectaba topic_key, así
// que TopicKey llegaba SIEMPRE vacío a todo consumidor de esa vía —el
// constructor de contexto, la tool MCP list_memories, la TUI—, sin error y sin
// aviso. Su hermana ListAllMemories sí lo proyecta: era una divergencia entre
// dos consultas que deben devolver lo mismo.
func TestListMemories_DevuelveTopicKey(t *testing.T) {
	db := openTestDB(t)

	m := &domain.Memory{
		Project:  "proj",
		Type:     domain.Preference,
		Title:    "Reglas de trabajo del proyecto",
		Content:  "contenido de las reglas",
		TopicKey: domain.TopicWorkRules,
	}
	if _, err := InsertMemory(db, m); err != nil {
		t.Fatalf("insert: %v", err)
	}

	mems, err := ListMemories(db, "proj", 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(mems) != 1 {
		t.Fatalf("esperaba 1 memoria, hay %d", len(mems))
	}
	if mems[0].TopicKey != domain.TopicWorkRules {
		t.Errorf("TopicKey = %q, esperaba %q — la columna no viaja en el SELECT de ListMemories",
			mems[0].TopicKey, domain.TopicWorkRules)
	}
}

// TestListMemories_TopicKeyVacioSigueVacio: una memoria sin clave de tópico
// debe seguir devolviendo cadena vacía, no NULL ni basura. Es el otro lado del
// COALESCE.
func TestListMemories_TopicKeyVacioSigueVacio(t *testing.T) {
	db := openTestDB(t)

	if _, err := InsertMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Learning, Title: "suelta", Content: "sin tópico",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	mems, err := ListMemories(db, "proj", 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if mems[0].TopicKey != "" {
		t.Errorf("TopicKey = %q, esperaba vacío", mems[0].TopicKey)
	}
}

// TestGetMemoryByTopicKey cubre FR-006/FR-031: la presencia de una memoria
// fijada no puede depender de la ventana de recencia de ListMemories, así que
// se resuelve por su clave.
func TestGetMemoryByTopicKey(t *testing.T) {
	db := openTestDB(t)

	if _, err := InsertMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Preference, Title: "Reglas",
		Content: "texto de las reglas", TopicKey: domain.TopicWorkRules,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := GetMemoryByTopicKey(db, "proj", domain.TopicWorkRules)
	if err != nil {
		t.Fatalf("por clave: %v", err)
	}
	if got == nil {
		t.Fatal("esperaba encontrar la memoria por su clave de tópico")
	}
	if got.Content != "texto de las reglas" || got.TopicKey != domain.TopicWorkRules {
		t.Errorf("memoria inesperada: %+v", got)
	}
}

// TestGetMemoryByTopicKey_AusenteNoEsError fija la regla explícita de la
// constitución: nil para "no encontrado", nunca un error de not-found.
func TestGetMemoryByTopicKey_AusenteNoEsError(t *testing.T) {
	db := openTestDB(t)

	got, err := GetMemoryByTopicKey(db, "proj", domain.TopicConstitution)
	if err != nil {
		t.Fatalf("una clave ausente no debe producir error, produjo: %v", err)
	}
	if got != nil {
		t.Errorf("esperaba nil, obtuve %+v", got)
	}
}

// TestGetMemoryByTopicKey_NoCruzaProyectos: el store es global y está indexado
// por proyecto; la consulta no puede filtrarse solo por clave.
func TestGetMemoryByTopicKey_NoCruzaProyectos(t *testing.T) {
	db := openTestDB(t)

	if _, err := InsertMemory(db, &domain.Memory{
		Project: "otro", Type: domain.Preference, Title: "Reglas ajenas",
		Content: "no deberían verse", TopicKey: domain.TopicWorkRules,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := GetMemoryByTopicKey(db, "proj", domain.TopicWorkRules)
	if err != nil {
		t.Fatalf("por clave: %v", err)
	}
	if got != nil {
		t.Errorf("la consulta cruzó proyectos: %+v", got)
	}
}

// ─── A1: vía de inserción inerte ────────────────────────────────────────────

// TestInsertSeedMemory_NoFormaSinapsis cubre G2 (FR-033). Antes de la vía
// inerte, la inercia dependía de un accidente: formSynapse retorna pronto si
// SessionID va vacío. Quien asociara la siembra a la sesión activa —un cambio
// razonable en apariencia— habría reactivado el enlace sin darse cuenta. Este
// test fuerza justamente ese escenario: sinapsis activa Y sesión presente.
func TestInsertSeedMemory_NoFormaSinapsis(t *testing.T) {
	db := openTestDB(t)
	SetSynapseEnabled(true)
	t.Cleanup(func() { SetSynapseEnabled(true) })

	sess, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("iniciar sesión: %v", err)
	}

	// Ancla: una memoria normal en la misma sesión, para que haya con qué enlazar.
	if _, err := InsertMemory(db, &domain.Memory{
		Project: "proj", SessionID: sess.ID, Type: domain.Learning,
		Title: "ancla", Content: "memoria previa de la sesión",
	}); err != nil {
		t.Fatalf("insert ancla: %v", err)
	}

	antes := contarRelaciones(t, db, "proj")

	if _, err := InsertSeedMemory(db, &domain.Memory{
		Project: "proj", SessionID: sess.ID, Type: domain.Preference,
		Title: "Reglas", Content: "texto de las reglas", TopicKey: domain.TopicWorkRules,
	}); err != nil {
		t.Fatalf("insert seed: %v", err)
	}

	if despues := contarRelaciones(t, db, "proj"); despues != antes {
		t.Errorf("la siembra creó %d relación(es); la vía inerte no debe enlazar nada", despues-antes)
	}
}

// TestInsertSeedMemory_NoExportaADR cubre G3 (FR-034) — la brecha ACTIVA que la
// primera auditoría dio por inofensiva. adrSectionForType mapea Architecture a
// una sección exportable, y la semilla de constitución es de ese tipo: con
// adr_sync_enabled=true, instalar habría publicado las 635 líneas en el
// documento ADR externo de la persona, síncrono y sin que nadie lo pidiera.
func TestInsertSeedMemory_NoExportaADR(t *testing.T) {
	db := openTestDB(t)

	prov := &adrProviderEspia{}
	SetADRSync(prov, NewADRSyncRepository(db))
	SetAdrSyncEnabled(true)
	t.Cleanup(func() {
		SetAdrSyncEnabled(false)
		SetADRSync(nil, nil)
	})

	// Control: por la vía normal, una memoria architecture SÍ intenta exportar.
	if _, err := InsertMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Architecture,
		Title: "decisión normal", Content: "contenido de la decisión",
	}); err != nil {
		t.Fatalf("insert normal: %v", err)
	}
	if prov.llamadas == 0 {
		t.Fatal("precondición rota: con adr_sync_enabled=true la vía normal debería intentar exportar")
	}

	prov.llamadas = 0

	if _, err := InsertSeedMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Architecture,
		Title: "Constitución del proyecto", Content: "texto de la constitución",
		TopicKey: domain.TopicConstitution,
	}); err != nil {
		t.Fatalf("insert seed: %v", err)
	}
	if prov.llamadas != 0 {
		t.Errorf("la siembra intentó exportar al ADR externo %d vez/veces; debe ser 0 incluso con la sincronización activada", prov.llamadas)
	}
}

// TestInsertSeedMemory_ConservaLaDepuracionDeSecretos: la vía inerte omite los
// canales laterales, NO la defensa de seguridad. Un token pegado por error en
// un documento importado no puede persistirse (data-model.md §3bis).
func TestInsertSeedMemory_ConservaLaDepuracionDeSecretos(t *testing.T) {
	db := openTestDB(t)

	id, err := InsertSeedMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Preference, Title: "Reglas",
		Content:  "reglas del equipo\nclave: ghp_" + strings.Repeat("a", 36) + "\nfin",
		TopicKey: domain.TopicWorkRules,
	})
	if err != nil {
		t.Fatalf("insert seed: %v", err)
	}

	got, err := GetMemoryByID(db, "proj", id)
	if err != nil {
		t.Fatalf("leer: %v", err)
	}
	if strings.Contains(got.Content, "ghp_aaa") {
		t.Error("la vía inerte persistió un token: la depuración de secretos no puede desactivarse")
	}
	if !strings.Contains(got.Content, "[REDACTED:github-token]") {
		t.Errorf("esperaba el marcador de redacción, contenido: %q", got.Content)
	}
}

// TestInsertSeedMemory_UpsertPorTopicKey: la vía inerte conserva la red de
// deduplicación por clave de tópico. Es lo que hace idempotente a la
// importación repetida y lo que permite que reset reemplace en su sitio.
func TestInsertSeedMemory_UpsertPorTopicKey(t *testing.T) {
	db := openTestDB(t)

	id1, err := InsertSeedMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Preference, Title: "Reglas",
		Content: "versión uno", TopicKey: domain.TopicWorkRules,
	})
	if err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	id2, err := InsertSeedMemory(db, &domain.Memory{
		Project: "proj", Type: domain.Preference, Title: "Reglas",
		Content: "versión dos", TopicKey: domain.TopicWorkRules,
	})
	if err != nil {
		t.Fatalf("insert 2: %v", err)
	}
	if id1 != id2 {
		t.Errorf("esperaba upsert sobre la misma fila (%d), se creó otra (%d)", id1, id2)
	}

	got, _ := GetMemoryByID(db, "proj", id1)
	if got.Content != "versión dos" {
		t.Errorf("contenido = %q, esperaba la versión nueva", got.Content)
	}
}

// ─── auxiliares ─────────────────────────────────────────────────────────────

func contarRelaciones(t *testing.T, db *sql.DB, project string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_relations WHERE project = ?`, project).Scan(&n); err != nil {
		t.Fatalf("contar relaciones: %v", err)
	}
	return n
}

// adrProviderEspia implementa ports.ADRSyncProvider contando los intentos de
// exportación, sin tocar nada externo.
type adrProviderEspia struct{ llamadas int }

func (p *adrProviderEspia) Name() string { return "espia" }

func (p *adrProviderEspia) GetDocument(_ context.Context) (string, error) {
	p.llamadas++
	return "", nil
}

func (p *adrProviderEspia) UpdateDocument(_ context.Context, _ string) error {
	p.llamadas++
	return nil
}
