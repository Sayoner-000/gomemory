package usecases_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// hcd001Store es un almacén de originales en memoria que falla a partir de
// la escritura failFrom (0 = nunca).
type hcd001Store struct {
	originals map[string]string
	puts      int
	failFrom  int
}

func (s *hcd001Store) Put(_ context.Context, _, content, _, _ string) (string, error) {
	s.puts++
	if s.failFrom > 0 && s.puts >= s.failFrom {
		return "", errors.New("disco lleno")
	}
	ref := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))[:12]
	s.originals[ref] = content
	return ref, nil
}

func (s *hcd001Store) Get(_ context.Context, ref string) (string, ports.OriginalMeta, bool, error) {
	c, ok := s.originals[ref]
	return c, ports.OriginalMeta{}, ok, nil
}

func (s *hcd001Store) Purge(context.Context) (int, int64, error) { return 0, 0, nil }
func (s *hcd001Store) Usage(context.Context) (int64, int, error) {
	return 0, len(s.originals), nil
}

func TestHCD001ExactRecovery(t *testing.T) {
	repo := newContextPackTestDeps(t)
	for i, words := range [][]string{{"á漢字", "tejado", "naranja"}, {"ñandú", "puente", "violeta"}, {"über", "molino", "turquesa"}} {
		var b strings.Builder
		for j := 0; j < 60; j++ {
			fmt.Fprintf(&b, "recuperar %s %s %d ", words[j%3], words[(j+1)%3], j*(i+7))
		}
		m := domain.Memory{Project: "recovery", Type: domain.Learning, Title: fmt.Sprintf("recuperar %d", i), Content: b.String()}
		if _, err := repo.Insert(&m); err != nil {
			t.Fatal(err)
		}
	}
	for _, failFrom := range []int{0, 2, 1} {
		store := &hcd001Store{originals: map[string]string{}, failFrom: failFrom}
		pack, err := usecases.BuildContextPack(repo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil, usecases.ContextRequest{
			Project: "recovery", Task: "recuperar", MaxTokens: 900, Compression: ports.CompressionNone,
			Renderer: hcd001JSONRenderer{}, Originals: store, Receipt: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		omitted, withRef := 0, 0
		for _, r := range pack.Receipts {
			if r.Status != "omitted" {
				continue
			}
			omitted++
			for _, ref := range r.Refs {
				withRef++
				got, _, found, _ := store.Get(context.Background(), ref)
				if !found || fmt.Sprintf("%x", sha256.Sum256([]byte(got))) != r.OriginalSHA256 {
					t.Fatalf("failFrom=%d: ref %s no resuelve al original por huella", failFrom, ref)
				}
			}
		}
		if omitted < 2 {
			t.Fatalf("failFrom=%d: se esperaban varias omisiones, hubo %d", failFrom, omitted)
		}
		want := omitted
		if failFrom > 0 {
			want = failFrom - 1
		}
		if withRef != want {
			t.Fatalf("failFrom=%d: refs entregadas=%d, originales disponibles=%d", failFrom, withRef, want)
		}
	}
}
