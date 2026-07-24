package usecases_test

import (
	"testing"
	"time"

	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

func snapProvider(name string, available bool) *fakeCodeProvider {
	return &fakeCodeProvider{snap: domain.CodeProviderSnapshot{
		Provider: name, Available: available, CheckedAt: time.Now(),
	}}
}

func TestFirstAvailable_ReturnsFirstWithAvailableSnapshot(t *testing.T) {
	unavailable := snapProvider("no-instalado", false)
	available := snapProvider("codebase-memory-mcp", true)

	got := usecases.FirstAvailable([]ports.CodeGraphProvider{unavailable, available})
	if got == nil || got.Name() != "codebase-memory-mcp" {
		t.Fatalf("esperaba el proveedor disponible, hubo %v", got)
	}
}

func TestFirstAvailable_PrefersEarlierOverLater(t *testing.T) {
	first := snapProvider("primero", true)
	second := snapProvider("segundo", true)

	got := usecases.FirstAvailable([]ports.CodeGraphProvider{first, second})
	if got == nil || got.Name() != "primero" {
		t.Fatalf("esperaba el primero de la lista (orden de prioridad), hubo %v", got)
	}
}

func TestFirstAvailable_NilWhenNoneAvailable(t *testing.T) {
	a := snapProvider("a", false)
	b := snapProvider("b", false)

	if got := usecases.FirstAvailable([]ports.CodeGraphProvider{a, b}); got != nil {
		t.Fatalf("esperaba nil cuando ninguno está disponible, hubo %v", got)
	}
}

func TestFirstAvailable_EmptyList(t *testing.T) {
	if got := usecases.FirstAvailable(nil); got != nil {
		t.Fatalf("esperaba nil para una lista vacía, hubo %v", got)
	}
}

func TestFirstAvailable_SkipsNilEntries(t *testing.T) {
	available := snapProvider("ok", true)
	got := usecases.FirstAvailable([]ports.CodeGraphProvider{nil, available})
	if got == nil || got.Name() != "ok" {
		t.Fatalf("debería saltar entradas nil y encontrar la disponible, hubo %v", got)
	}
}
