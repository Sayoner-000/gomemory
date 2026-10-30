package cli

import (
	"fmt"
	"strings"
	"testing"

	"mem/adapters/secondary/clock"
	"mem/adapters/secondary/compression/native"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/domain"
)

// fixedSearchRepo devuelve siempre los mismos resultados, en el orden dado:
// así la coincidencia exacta cae en las filas intermedias que comprime la tabla.
type fixedSearchRepo struct {
	ports.MemoryRepository
	mems []domain.Memory
}

func (r fixedSearchRepo) Search(string, string, int) ([]domain.Memory, error) { return r.mems, nil }

// TestHCD004SearchKeepsExactTitleRow fija la reproducción de HCD-004
// (recuperacion-03 de HCD-002): con el nivel max, la fila cuyo título es igual
// a la consulta sigue visible con su ID aunque el resto de la tabla se
// comprima.
func TestHCD004SearchKeepsExactTitleRow(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	const query = "representativo decision 0146: sobre observación omisión delta arr"
	var mems []domain.Memory
	for i := 0; i < 20; i++ {
		title := fmt.Sprintf("checkpoint representativo %04d", 146+i)
		typ := domain.Checkpoint
		if i == 10 {
			title, typ = query, domain.Decision
		}
		mems = append(mems, domain.Memory{ID: int64(500 + i), Project: "demo", Type: typ, Title: title,
			Content: fmt.Sprintf("CHECKPOINT-%04d motor delta huella contrato aislamiento sesión entrega capa", 146+i)})
	}
	engine := native.NewEngine(persistence.NewOriginalStoreRepository(db, clock.SystemClock{}, 0, 0), nil, nil, "demo")
	deps := &Deps{
		Root: root, Project: "demo", ProjectRepo: &fakeProjectRepo{root: root},
		MemoryRepo: fixedSearchRepo{persistence.NewMemoryRepository(db), mems},
		Compressor: engine, CompressionLevel: ports.CompressionMax,
	}

	out := captureStdout(t, func() { CmdSearch(deps, []string{query}) })
	if !strings.Contains(out, domain.MarkerTag) {
		t.Fatalf("control: la tabla de 20 filas debía comprimirse con max:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "510 ") && strings.Contains(line, query) {
			return
		}
	}
	t.Fatalf("la fila 510, cuyo título es igual a la consulta, quedó oculta:\n%s", out)
}
