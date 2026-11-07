package cli

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"mem/application/ports"
	"mem/domain"
)

type toolOutputCompressionStub func(string, ports.CompressionOptions) (ports.CompressionResult, error)

func (f toolOutputCompressionStub) Compress(s string, opts ports.CompressionOptions) (ports.CompressionResult, error) {
	return f(s, opts)
}

// Los métodos no usados quedan sin implementar: invocarlos hace fallar la
// prueba, en lugar de simular un éxito que oculte un camino inesperado.
type toolOutputGrantSpy struct {
	ports.OriginalStoreRepository
	ports.AutomaticOriginalGrantsRepository
	getCalls   int
	grantCalls int
	scope      ports.AutomaticOriginalScope
	refs       []string
}

func (s *toolOutputGrantSpy) Get(_ context.Context, ref string) (string, ports.OriginalMeta, bool, error) {
	s.getCalls++
	return "original", ports.OriginalMeta{Ref: ref, Project: "project"}, true, nil
}

func (s *toolOutputGrantSpy) Grant(_ context.Context, scope ports.AutomaticOriginalScope, refs []string) error {
	s.grantCalls++
	s.scope = scope
	s.refs = append([]string(nil), refs...)
	return nil
}

func TestToolOutputExpiredCompressionSkipsGrant(t *testing.T) {
	testToolOutputCompressionGrant(t, true)
}

func TestToolOutputWithinBudgetGrants(t *testing.T) {
	testToolOutputCompressionGrant(t, false)
}

func testToolOutputCompressionGrant(t *testing.T, expireDuringCompression bool) {
	t.Helper()
	started := make(chan struct{})
	release := make(chan struct{})
	releaseCompression := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseCompression)
	expired := make(chan struct{})
	refs := []string{"abcdefabcdef"}
	spy := &toolOutputGrantSpy{}
	deps := &Deps{
		Project:                 "project",
		OriginalStore:           spy,
		AutomaticOriginalGrants: spy,
		Compressor: toolOutputCompressionStub(func(_ string, _ ports.CompressionOptions) (ports.CompressionResult, error) {
			close(started)
			<-release
			return ports.CompressionResult{Content: "reducido", Refs: refs}, nil
		}),
	}
	scope := ports.AutomaticOriginalScope{Project: "project", Agent: "claude", SessionID: "host-session"}
	hintCalls := 0
	compress := toolOutputCompress(deps, scope, func() string {
		hintCalls++
		return RetrieveHint
	}, func() bool {
		select {
		case <-expired:
			return true
		default:
			return false
		}
	})
	type result struct {
		content string
		ok      bool
	}
	done := make(chan result, 1)
	go func() {
		content, ok := compress(strings.Repeat("x", domain.ToolOutputMinTokens*4))
		done <- result{content, ok}
	}()
	// El reloj lógico vence después de entrar en Compress y antes de que
	// retorne. Los temporizadores solo detectan bloqueos de la propia prueba.
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("el callback no inició la compresión")
	}
	if expireDuringCompression {
		close(expired)
	}
	releaseCompression()
	var got result
	select {
	case got = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("el callback no terminó tras liberar la compresión")
	}
	// Recibir done sincroniza también las lecturas del espía y de hintCalls.
	if expireDuringCompression {
		if spy.grantCalls != 0 {
			t.Errorf("Grant se invocó %d veces tras el plazo; se esperaban cero concesiones", spy.grantCalls)
		}
		if spy.getCalls != 0 {
			t.Errorf("se inició GrantAutomaticOriginals tras el plazo: %d lecturas", spy.getCalls)
		}
		if got.ok || got.content != "" || hintCalls != 0 {
			t.Errorf("el callback vencido publicó salida o consumió la pista: %+v, pistas=%d", got, hintCalls)
		}
		return
	}
	if spy.getCalls != 1 || spy.grantCalls != 1 || spy.scope != scope || !reflect.DeepEqual(spy.refs, refs) {
		t.Errorf("concesión de control inválida: lecturas=%d concesiones=%d scope=%+v refs=%v", spy.getCalls, spy.grantCalls, spy.scope, spy.refs)
	}
	if !got.ok || got.content != "reducido"+RetrieveHint || hintCalls != 1 {
		t.Errorf("el callback dentro del plazo debe entregar la reducción y la pista: %+v, pistas=%d", got, hintCalls)
	}
}
