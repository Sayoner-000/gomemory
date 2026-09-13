package usecases_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

func buildPack031(t *testing.T, memRepo ports.MemoryRepository, req usecases.ContextRequest) domain.ContextPack {
	t.Helper()
	pack, err := usecases.BuildContextPack(memRepo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil, req)
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}
	return pack
}

func packIndex031(p domain.ContextPack, id int64) int {
	want := fmt.Sprintf("memory:%d", id)
	for i, it := range p.Items {
		if it.ID == want {
			return i
		}
	}
	return -1
}

func assertStatsInvariant031(t *testing.T, s domain.ContextStats) {
	t.Helper()
	sum := s.ItemsDuplicate + s.ItemsCritical + s.ItemsRelevant + s.ItemsOptional + s.ItemsDiscarded
	if s.ItemsRetrieved != sum {
		t.Fatalf("ItemsRetrieved (%d) != suma de categorías (%d): %+v", s.ItemsRetrieved, sum, s)
	}
}

func TestBuildContextPack_SinRelacionesEsIdentico(t *testing.T) {
	_, memRepo, _, _, _ := newCtx031Fixture(t)
	mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "redis como caché", Content: "usamos redis como caché compartida"})
	mustInsert031(t, memRepo, domain.Memory{Type: domain.Pattern, Title: "patrón de caché redis", Content: "invalidar la caché redis al escribir"})
	req := usecases.ContextRequest{Task: "redis", Project: "proj", MaxTokens: 1000}
	a := buildPack031(t, memRepo, req)
	req.Relations = nil
	b := buildPack031(t, memRepo, req)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("sin relaciones el pack debe ser idéntico:\n a=%+v\n b=%+v", a, b)
	}
}

func TestBuildContextPack_AnadeVecinosPorMasa(t *testing.T) {
	_, memRepo, _, relRepo, _ := newCtx031Fixture(t)
	a := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "redis como caché", Content: "usamos redis como caché compartida"})
	nb := mustInsert031(t, memRepo, domain.Memory{Type: domain.Learning, Title: "latencia del servicio de pagos", Content: "medimos el p95 del servicio de pagos"})
	cp := mustInsert031(t, memRepo, domain.Memory{Type: domain.Checkpoint, Title: "Checkpoint automático", Content: "Editó: pagos.go"})
	relate031(t, relRepo, a, nb, domain.Related, 1)
	relate031(t, relRepo, a, cp, domain.Related, 1)

	req := usecases.ContextRequest{Task: "redis", Project: "proj", MaxTokens: 2000}
	if without := buildPack031(t, memRepo, req); packIndex031(without, nb) >= 0 {
		t.Fatalf("precondición: la búsqueda sola no debe devolver la vecina")
	}
	req.Relations = relRepo
	pack := buildPack031(t, memRepo, req)
	at := packIndex031(pack, nb)
	if at < 0 {
		t.Fatalf("falta la vecina memory:%d: %+v", nb, pack.Items)
	}
	if pack.Items[at].Priority != domain.PriorityOptional {
		t.Fatalf("la vecina debe ser opcional, es %v", pack.Items[at].Priority)
	}
	if at != len(pack.Items)-1 {
		t.Fatalf("la vecina debe ir al final (índice %d de %d)", at, len(pack.Items))
	}
	if packIndex031(pack, cp) >= 0 {
		t.Fatalf("un checkpoint nunca es vecino")
	}
	assertStatsInvariant031(t, pack.Stats)
}

func TestBuildContextPack_MaximoCincoVecinos(t *testing.T) {
	_, memRepo, _, relRepo, _ := newCtx031Fixture(t)
	a := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "redis como caché", Content: "usamos redis como caché compartida"})
	neighbors := make([]int64, 7)
	for i := range neighbors {
		neighbors[i] = mustInsert031(t, memRepo, domain.Memory{Type: domain.Learning, Title: fmt.Sprintf("vecina %d", i), Content: fmt.Sprintf("texto sin relación %d", i)})
		relate031(t, relRepo, a, neighbors[i], domain.Related, 1)
	}
	pack := buildPack031(t, memRepo, usecases.ContextRequest{Task: "redis", Project: "proj", MaxTokens: 4000, Relations: relRepo})
	added := 0
	for _, id := range neighbors {
		if packIndex031(pack, id) >= 0 {
			added++
		}
	}
	if added != 5 {
		t.Fatalf("vecinas añadidas = %d, want 5", added)
	}
}

func TestBuildContextPack_VecinosCedenAntePresupuesto(t *testing.T) {
	_, memRepo, _, relRepo, _ := newCtx031Fixture(t)
	a := mustInsert031(t, memRepo, domain.Memory{Type: domain.Decision, Title: "redis como caché", Content: "usamos redis como caché compartida"})
	nb := mustInsert031(t, memRepo, domain.Memory{Type: domain.Learning, Title: "latencia del servicio de pagos", Content: strings.Repeat("medimos el p95 del servicio de pagos. ", 20)})
	relate031(t, relRepo, a, nb, domain.Related, 1)

	base := buildPack031(t, memRepo, usecases.ContextRequest{Task: "redis", Project: "proj", MaxTokens: 2000})
	pack := buildPack031(t, memRepo, usecases.ContextRequest{Task: "redis", Project: "proj", MaxTokens: base.TokenCount, Relations: relRepo})
	if packIndex031(pack, nb) >= 0 {
		t.Fatalf("sin presupuesto sobrante la vecina debe descartarse")
	}
	if pack.Stats.ItemsDiscarded < 1 {
		t.Fatalf("la vecina descartada debe contar en ItemsDiscarded: %+v", pack.Stats)
	}
	assertStatsInvariant031(t, pack.Stats)
}
