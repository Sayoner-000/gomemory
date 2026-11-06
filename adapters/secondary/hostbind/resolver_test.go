package hostbind

import (
	"testing"
	"time"
)

type enlaceContado struct {
	id      string
	lookups int
}

func (e *enlaceContado) Publish(id string) error { e.id = id; return nil }
func (e *enlaceContado) Lookup() (string, bool) {
	e.lookups++
	return e.id, e.id != ""
}

// HCD-036 Q1: el resolver aplica la precedencia del dominio, consulta el enlace
// con una caché corta (cada consulta recorre procesos) y solo sin hook ni
// enlace usa el id local del canal.
func TestResolver(t *testing.T) {
	enlace := &enlaceContado{}
	ahora := time.Unix(1000, 0)
	r := NewResolver(enlace, func() string { return "local-1" })
	r.now = func() time.Time { return ahora }

	if got := r.Current(); got != "local-1" {
		t.Fatalf("sin hook ni enlace, el id local: %q", got)
	}
	enlace.id = "claude-A"
	if got := r.Current(); got != "local-1" {
		t.Fatalf("dentro de la caché no se vuelve a recorrer procesos: %q", got)
	}
	// Un fallo caduca enseguida: el hook SessionStart publica el enlace un
	// instante después de que el MCP arranque.
	ahora = ahora.Add(300 * time.Millisecond)
	if got := r.Current(); got != "claude-A" {
		t.Fatalf("tras la caché corta de un fallo, el enlace: %q", got)
	}
	enlace.id = "claude-A2"
	ahora = ahora.Add(time.Second)
	if got := r.Current(); got != "claude-A" {
		t.Fatalf("un acierto se guarda más tiempo: %q", got)
	}
	ahora = ahora.Add(2 * time.Second)
	if got := r.Current(); got != "claude-A2" {
		t.Fatalf("pasada la caché, el enlace nuevo (p. ej. tras /clear): %q", got)
	}
	r.Adopt("claude-hook")
	if got := r.Current(); got != "claude-hook" {
		t.Fatalf("el id del hook manda: %q", got)
	}
	if n := enlace.lookups; n != 3 {
		t.Fatalf("consultas al enlace = %d, want 3", n)
	}
	if got := NewResolver(nil, nil).Current(); got != "" {
		t.Fatalf("sin enlace ni local, sin conversación: %q", got)
	}
}
