package hostbind

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
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

// TerminalID identifica el terminal del proceso («terminal-ttys003»), o ""
// si no tiene: la sesión propia de una persona en su terminal (Q2).
func TerminalID() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	out, err := exec.Command("ps", "-o", "tty=", "-p", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return ""
	}
	tty := strings.TrimSpace(string(out))
	if tty == "" || strings.Trim(tty, "?") == "" {
		return ""
	}
	return "terminal-" + tty
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
