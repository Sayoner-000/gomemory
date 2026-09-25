package main

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/compression/native"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/speckit"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

const secretoFalso = "ghp_FAKE000000000000000000000000000000000"

// originalesGuardados devuelve el contenido DESCOMPRIMIDO de todos los
// originales: content_gz está en gzip, así que buscar el secreto en la columna
// cruda no probaría nada.
func originalesGuardados(t *testing.T, db *sql.DB, store *persistence.OriginalStoreRepository) []string {
	t.Helper()
	rows, err := db.Query(`SELECT ref FROM compression_originals`)
	if err != nil {
		t.Fatal(err)
	}
	var refs []string
	for rows.Next() {
		var r string
		_ = rows.Scan(&r)
		refs = append(refs, r)
	}
	_ = rows.Close()
	var out []string
	for _, r := range refs {
		c, _, ok, err := store.Get(context.Background(), r)
		if err != nil || !ok {
			t.Fatalf("original %s ilegible: %v", r, err)
		}
		out = append(out, c)
	}
	return out
}

// T024 — SC-009 por todas las vías que comprimen: la compresión directa, el
// paquete de contexto (pack_build) y el paquete delegado de Octopus.
//
// En pack_build y Octopus el motor solo recibe memorias (los fragmentos de
// spec-kit son críticos y nunca se comprimen), y las memorias ya se redactan al
// guardarse. La prueba lo verifica igualmente: ningún original guardado
// contiene el secreto.
func TestCompressionPrivacy_TodasLasVias(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	store := persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0)
	engine := native.NewEngine(store, nil, nil, "p")

	privado := strings.Repeat("2026-09-25 10:00:00 INFO token <private>"+secretoFalso+"</private> usado en la petición\n", 300)

	// 1. Compresión directa (pack_compress / mem pack compress).
	r, _ := engine.Compress(privado, ports.CompressionOptions{Level: ports.CompressionMax})
	if len(r.Refs) != 0 || r.FallbackReason != "private" {
		t.Errorf("directa: refs=%v fallback=%q", r.Refs, r.FallbackReason)
	}

	// 2 y 3. Una memoria grande con el secreto sin marcar (lo redacta el
	// guardado) y otra con <private>.
	memRepo := persistence.NewMemoryRepository(db)
	for i, content := range []string{
		strings.Repeat("2026-09-25 10:00:00 INFO usando "+secretoFalso+" contra la API de lotes\n", 300),
		privado,
	} {
		if _, err := memRepo.Insert(&domain.Memory{Project: "p", Type: domain.Learning, Title: "log de prueba " + string(rune('A'+i)), Content: content}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := usecases.BuildContextPack(memRepo, engine, tokens.ApproximateTokenCounter{}, speckit.Reader{}, usecases.ContextRequest{
		Task: "log de prueba API lotes", Project: "p", MaxTokens: 50000, MinRelevance: -1, Root: root, Compression: ports.CompressionMax,
	}); err != nil {
		t.Fatal(err)
	}
	uc := usecases.NewPackContractUseCase(memRepo, engine, tokens.ApproximateTokenCounter{}, speckit.Reader{})
	uc.Compression = ports.CompressionMax
	_, _ = uc.Build(usecases.PackContractRequest{
		Unit:     domain.WorkUnit{ID: "u1", Objective: "log de prueba API lotes"},
		Decision: domain.RouteDecision{Route: domain.RouteDelegate},
		Project:  "p", Root: root,
	})

	originales := originalesGuardados(t, db, store)
	if len(originales) == 0 {
		t.Fatal("control: pack_build debería haber comprimido la memoria redactada y guardado su original")
	}
	for _, o := range originales {
		if strings.Contains(o, secretoFalso) || strings.Contains(o, "<private>") {
			t.Errorf("un original guardado contiene contenido privado:\n%.200s", o)
		}
	}
}
