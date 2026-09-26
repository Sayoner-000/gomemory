package persistence

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"mem/adapters/secondary/compression/native"
	"mem/application/ports"
	"mem/domain"
)

// C-001 (acr_961a1676) — con el tope mínimo que imponen los ajustes, una
// llamada que guarda varios originales grandes no expulsa ninguno de los que
// acaba de guardar: toda ref devuelta sigue siendo recuperable (INV-C4).
func TestCompressKeepsEveryRefWithSmallConfiguredCap(t *testing.T) {
	db := openTestDB(t)
	clk := &fakeClock{now: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)}
	store := NewOriginalStoreRepository(db, clk, 7, domain.EffectiveOriginalsMaxMB(1))
	engine := native.NewEngine(store, nil, nil, "p")

	rng := rand.New(rand.NewSource(3))
	fenced := func() string { // array JSON grande e incompresible por gzip
		var sb strings.Builder
		sb.WriteString("```json\n[")
		for i := 0; i < 60; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			b := make([]byte, 11<<10)
			rng.Read(b)
			fmt.Fprintf(&sb, `{"id":%d,"blob":"%s"}`, i, base64.StdEncoding.EncodeToString(b))
		}
		sb.WriteString("]\n```\n")
		return sb.String()
	}
	in := "Primer volcado del servicio.\n\n" + fenced() + "\nSegundo volcado del servicio.\n\n" + fenced()

	res, err := engine.Compress(in, ports.CompressionOptions{Level: ports.CompressionMax})
	if err != nil || !res.Compressed || len(res.Refs) != 2 {
		t.Fatalf("se esperaban dos originales guardados: %v %+v", err, res.Refs)
	}
	for _, ref := range res.Refs {
		if _, _, ok, err := store.Get(t.Context(), ref); err != nil || !ok {
			t.Errorf("la ref %s debe seguir siendo recuperable (err=%v)", ref, err)
		}
	}
}
