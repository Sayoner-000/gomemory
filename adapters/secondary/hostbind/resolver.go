package hostbind

import (
	"os"
	"strings"
	"sync"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// lookupTTL acota cada cuánto se vuelve a recorrer la cadena de procesos: un
// MCP de larga vida ve en segundos la conversación nueva tras un /clear.
// missTTL es más corto: al arrancar, el MCP suele consultar antes de que el
// hook SessionStart publique el enlace, y no debe quedarse con ese fallo.
const (
	lookupTTL = 2 * time.Second
	missTTL   = 250 * time.Millisecond
)

// Resolver implementa ports.ConversationResolver para un proceso: el id que
// adopta de un hook, el enlace de su host o su id local, en ese orden.
type Resolver struct {
	mu      sync.Mutex
	hook    string
	binding ports.HostBinding
	local   func() string
	now     func() time.Time

	bound     string
	checkedAt time.Time
	checked   bool
}

var _ ports.ConversationResolver = (*Resolver)(nil)

// TerminalEnv es la variable de entorno que da a la CLI y a la TUI una
// identidad de terminal propia (HCD-048, Q1).
const TerminalEnv = "GOMEMORY_TERMINAL"

// TerminalID es la identidad de terminal fijada con GOMEMORY_TERMINAL
// («terminal-<valor>»), o "" sin ella. No se deduce del TTY (HCD-048): el MCP
// y los hooks no tienen terminal, y una CLI con TTY abría una sesión que ellos
// no veían.
func TerminalID() string {
	if v := strings.TrimSpace(os.Getenv(TerminalEnv)); v != "" {
		return "terminal-" + v
	}
	return ""
}

// NewResolver crea el resolver. binding y local pueden ser nil.
func NewResolver(binding ports.HostBinding, local func() string) *Resolver {
	return &Resolver{binding: binding, local: local, now: time.Now}
}

func (r *Resolver) Adopt(hookID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id := strings.TrimSpace(hookID); id != "" {
		r.hook = id
	}
}

func (r *Resolver) Current() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hook != "" {
		return r.hook
	}
	ttl := lookupTTL
	if r.bound == "" {
		ttl = missTTL
	}
	if r.binding != nil && (!r.checked || r.now().Sub(r.checkedAt) >= ttl) {
		r.bound, _ = r.binding.Lookup()
		r.checked, r.checkedAt = true, r.now()
	}
	local := ""
	if r.local != nil {
		local = r.local()
	}
	return domain.ResolveConversationID(r.hook, r.bound, local)
}
