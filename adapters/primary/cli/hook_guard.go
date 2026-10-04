package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mem/adapters/primary/setup"
	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/domain"
)

// Guarda de reentrada (feature 035, research.md R4). Con los hooks de gomemory
// registrados a la vez en el ámbito de usuario y en el de proyecto, Claude Code
// ejecuta cada evento dos veces EN PARALELO con el mismo stdin: recordatorios
// duplicados, checkpoints dobles y carreras en los marcadores. La segunda
// invocación se reconoce porque la primera conserva un lock del SO. Las
// invocaciones secuenciales sin identidad de prompt —dos prompts iguales
// seguidos— no son duplicados. UserPromptSubmit también usa prompt_id, cuando
// Claude lo proporciona, para reconocer gemelos que llegan después del cierre.

func hookLockPath(root, event string, payload []byte) string {
	sum := sha256.Sum256(payload)
	return filepath.Join(root, persistence.MemDir, ".hook-lock-"+event+"-"+hex.EncodeToString(sum[:])[:12])
}

func hookDonePath(root, event string, payload []byte) string {
	sum := sha256.Sum256(payload)
	return filepath.Join(root, persistence.MemDir, ".hook-done-"+event+"-"+hex.EncodeToString(sum[:])[:12])
}

func hookPromptID(payload []byte) string {
	var input struct {
		PromptID string `json:"prompt_id"`
	}
	if json.Unmarshal(payload, &input) != nil {
		return ""
	}
	return input.PromptID
}

func completedHookIdentity(event string, payload []byte) bool {
	// prompt_id identifica un prompt de usuario, pero varios eventos Stop o de
	// subagente legítimos pueden pertenecer a ese mismo prompt. El recibo solo
	// se aplica al evento que tiene correspondencia uno a uno con ese ID.
	return event == "user-prompt-submit" && hookPromptID(payload) != ""
}

var hookLockFiles sync.Map // ruta -> *os.File; el proceso conserva el lock hasta salir

// acquireHookLock devuelve true si otro proceso todavía atiende este evento y
// stdin. El lock del SO hace atómica la adquisición incluso con archivos viejos.
// Ante un error responde false: la guarda nunca impide un turno legítimo.
func acquireHookLock(root, event string, payload []byte) bool {
	p := hookLockPath(root, event, payload)
	mutex, err := hookGuardMutex(root)
	if err != nil {
		return false
	}
	defer closeHookGuardMutex(mutex)
	purgeHookLocks(root)
	if completedHookIdentity(event, payload) {
		if info, err := os.Stat(hookDonePath(root, event, payload)); err == nil &&
			time.Since(info.ModTime()) < domain.HookCompletedReceiptSecs*time.Second {
			return true
		}
	}
	if _, held := hookLockFiles.Load(p); held {
		return true
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return false
	}
	locked, err := tryLockHookFile(f)
	if err != nil || !locked {
		_ = f.Close()
		return err == nil && !locked
	}
	// Un archivo de una ejecución anterior no indica propiedad: solo el lock
	// retenido por un proceso vivo la indica. No se reemplaza ningún sello.
	hookLockFiles.Store(p, f)
	return false
}

func purgeHookLocks(root string) {
	completed, _ := filepath.Glob(filepath.Join(root, persistence.MemDir, ".hook-done-*"))
	completedLimit := time.Now().Add(-domain.HookCompletedReceiptSecs * time.Second)
	for _, p := range completed {
		if info, err := os.Stat(p); err == nil && info.ModTime().Before(completedLimit) {
			_ = os.Remove(p)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(root, persistence.MemDir, ".hook-lock-*"))
	limit := time.Now().Add(-domain.HookLockPurgeSecs * time.Second)
	for _, m := range matches {
		if strings.HasSuffix(m, ".hook-lock-mutex") {
			continue
		}
		if info, err := os.Stat(m); err == nil && info.ModTime().Before(limit) {
			f, err := os.OpenFile(m, os.O_RDWR, 0o600)
			if err != nil {
				continue
			}
			if locked, err := tryLockHookFile(f); err == nil && locked {
				_ = unlockHookFile(f)
				_ = f.Close()
				_ = os.Remove(m)
				continue
			}
			_ = f.Close()
		}
	}
}

// markHookCompleted conserva un recibo solo si hay un prompt_id del host y
// esta invocación poseía el lock. Se llama tras retornar el handler, nunca al
// adquirir: un proceso que muere antes de completar deja al gemelo continuar.
func markHookCompleted(root, event string, payload []byte) {
	if !completedHookIdentity(event, payload) {
		return
	}
	if _, held := hookLockFiles.Load(hookLockPath(root, event, payload)); !held {
		return
	}
	mutex, err := hookGuardMutex(root)
	if err != nil {
		return
	}
	defer closeHookGuardMutex(mutex)
	_ = writeFileAtomic(hookDonePath(root, event, payload), nil, 0o600)
}

func markHookCompletedForEvent(deps *Deps, event string) {
	root, err := deps.ProjectRepo.FindRoot()
	if err == nil {
		markHookCompleted(root, event, readHookStdinRaw())
	}
}

func hookGuardMutex(root string) (*os.File, error) {
	p := filepath.Join(root, persistence.MemDir, ".hook-lock-mutex")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockHookFile(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func closeHookGuardMutex(f *os.File) {
	_ = unlockHookFile(f)
	_ = f.Close()
}

// recordGuard registra un evento de las protecciones (FR-028). Fire-and-forget:
// sin registro disponible o ante un fallo, no altera la salida del hook.
func recordGuard(deps *Deps, agent, kind, detail string) {
	if deps == nil || deps.ChannelActivity == nil {
		return
	}
	if rec, ok := deps.ChannelActivity.(ports.HookGuardRecorder); ok {
		_ = rec.RecordGuard(agent, kind, detail)
	}
}

// agentOfDialect nombra al agente detrás de un dialecto de salida, para
// atribuir los eventos de guarda.
func agentOfDialect(d hookDialect) string {
	switch d {
	case dialectJSON:
		return "codex"
	case dialectText:
		return "opencode"
	case dialectClaude:
		return "claude"
	default:
		return "otro"
	}
}

// hookLockHold es la vida mínima del dueño de un bloqueo cuando hay hooks
// duplicados. Claude Code lanza los dos en paralelo, pero el segundo puede
// arrancar unos milisegundos tarde; sin esta espera, un primer proceso rápido
// terminaba antes y el duplicado pasaba por legítimo. Solo la pagan las
// instalaciones con la duplicación todavía sin corregir (mem update la quita).
const hookLockHold = 250 * time.Millisecond

var holdHookLock = func() { time.Sleep(hookLockHold) }

// releaseHookLock se usa cuando el handler retorna sin salir del proceso. La
// salida del proceso también libera el lock del SO, aunque no pase por defer.
func releaseHookLock(root, event string, payload []byte) {
	p := hookLockPath(root, event, payload)
	mutex, err := hookGuardMutex(root)
	if err != nil {
		return
	}
	defer closeHookGuardMutex(mutex)
	if held, ok := hookLockFiles.LoadAndDelete(p); ok {
		f := held.(*os.File)
		_ = unlockHookFile(f)
		_ = f.Close()
		_ = os.Remove(p)
	}
}

func releaseHookLockForEvent(deps *Deps, event string) {
	root, err := deps.ProjectRepo.FindRoot()
	if err == nil {
		releaseHookLock(root, event, readHookStdinRaw())
	}
}

// hookRegisteredTwice dice si event está registrado como hook de gomemory en el
// settings.json global y en el del proyecto a la vez.
func hookRegisteredTwice(root, event string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	for _, sub := range setup.DuplicateClaudeHookSubs(home, root) {
		if sub == event {
			return true
		}
	}
	return false
}
