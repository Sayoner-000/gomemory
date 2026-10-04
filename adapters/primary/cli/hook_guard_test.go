package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireHookLock_PrimeraInvocacionAdquiere(t *testing.T) {
	root := t.TempDir()
	payload := []byte(`{"prompt":"hola"}`)
	if dup := acquireHookLock(root, "user-prompt-submit", payload); dup {
		t.Error("sin bloqueo previo no puede ser duplicado")
	}
	releaseHookLock(root, "user-prompt-submit", payload)
}

func TestAcquireHookLock_DuplicadoSiElDuenoSigueVivo(t *testing.T) {
	root := t.TempDir()
	payload := []byte(`{"prompt":"hola"}`)
	acquireHookLock(root, "user-prompt-submit", payload) // dueño: este mismo proceso, vivo
	if dup := acquireHookLock(root, "user-prompt-submit", payload); !dup {
		t.Error("misma invocación con dueño vivo y reciente debe ser duplicado")
	}
	releaseHookLock(root, "user-prompt-submit", payload)
}

func TestAcquireHookLock_SecuencialConDuenoMuertoNoEsDuplicado(t *testing.T) {
	root := t.TempDir()
	payload := []byte(`{}`)
	p := hookLockPath(root, "user-prompt-submit", payload)
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	// PID que no existe: la invocación anterior ya terminó.
	if err := os.WriteFile(p, []byte(fmt.Sprintf("%d %d", 999999999, time.Now().Unix())), 0o600); err != nil {
		t.Fatalf("sembrar: %v", err)
	}
	if dup := acquireHookLock(root, "user-prompt-submit", payload); dup {
		t.Error("con el dueño ya terminado, la invocación secuencial es legítima")
	}
	releaseHookLock(root, "user-prompt-submit", payload)
}

func TestAcquireHookLock_BloqueoViejoNoEsDuplicado(t *testing.T) {
	root := t.TempDir()
	payload := []byte(`{}`)
	p := hookLockPath(root, "turn-end", payload)
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	viejo := time.Now().Unix() - 10
	if err := os.WriteFile(p, []byte(fmt.Sprintf("%d %d", os.Getpid(), viejo)), 0o600); err != nil {
		t.Fatalf("sembrar: %v", err)
	}
	if dup := acquireHookLock(root, "turn-end", payload); dup {
		t.Error("un bloqueo de más de 5 s no cuenta como duplicado")
	}
	releaseHookLock(root, "turn-end", payload)
}

func TestAcquireHookLock_PayloadsDistintosNoChocan(t *testing.T) {
	root := t.TempDir()
	uno := []byte(`{"prompt":"uno"}`)
	dos := []byte(`{"prompt":"dos"}`)
	acquireHookLock(root, "user-prompt-submit", uno)
	if dup := acquireHookLock(root, "user-prompt-submit", dos); dup {
		t.Error("prompts distintos no pueden tratarse como duplicados")
	}
	releaseHookLock(root, "user-prompt-submit", uno)
	releaseHookLock(root, "user-prompt-submit", dos)
	if hookLockPath(root, "a", []byte("x")) == hookLockPath(root, "b", []byte("x")) {
		t.Error("eventos distintos deben tener claves distintas")
	}
}

func TestAcquireHookLock_PurgaBloqueosAntiguos(t *testing.T) {
	root := t.TempDir()
	viejo := hookLockPath(root, "session-start", []byte("viejo"))
	_ = os.MkdirAll(filepath.Dir(viejo), 0o700)
	_ = os.WriteFile(viejo, []byte("1 1"), 0o600)
	antes := time.Now().Add(-2 * time.Minute)
	_ = os.Chtimes(viejo, antes, antes)

	acquireHookLock(root, "session-start", []byte("nuevo"))
	releaseHookLock(root, "session-start", []byte("nuevo"))
	if _, err := os.Stat(viejo); !os.IsNotExist(err) {
		t.Error("los bloqueos de más de 60 s deben purgarse")
	}
}

// Un archivo vacío de un proceso que terminó no conserva propiedad sobre el
// lock: solo el bloqueo que mantiene el SO distingue al dueño actual.
func TestAcquireHookLock_ArchivoVacioSinDuenoNoEsDuplicado(t *testing.T) {
	root := t.TempDir()
	payload := []byte(`{"prompt":"x"}`)
	p := hookLockPath(root, "user-prompt-submit", payload)
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = os.WriteFile(p, nil, 0o600)
	if dup := acquireHookLock(root, "user-prompt-submit", payload); dup {
		t.Error("un archivo sin lock activo no pertenece a otra invocación")
	}
	releaseHookLock(root, "user-prompt-submit", payload)
}

func TestAcquireHookLock_ElDuenoSigueProtegidoDespuesDe250ms(t *testing.T) {
	root := t.TempDir()
	payload := []byte(`{"prompt":"tardío"}`)
	if acquireHookLock(root, "user-prompt-submit", payload) {
		t.Fatal("el primero debe adquirir")
	}
	time.Sleep(350 * time.Millisecond)
	if !acquireHookLock(root, "user-prompt-submit", payload) {
		t.Error("el duplicado tardío debe descartarse mientras el handler siga vivo")
	}
	releaseHookLock(root, "user-prompt-submit", payload)
	if acquireHookLock(root, "user-prompt-submit", payload) {
		t.Error("un turno secuencial debe ejecutarse tras liberar el lock")
	}
	releaseHookLock(root, "user-prompt-submit", payload)
}

func TestAcquireHookLock_PromptIDDescartaGemeloDespuesDelCierre(t *testing.T) {
	root := t.TempDir()
	one := []byte(`{"session_id":"s","prompt_id":"p-1","prompt":"igual"}`)
	if acquireHookLock(root, "user-prompt-submit", one) {
		t.Fatal("la primera copia debe adquirir")
	}
	markHookCompleted(root, "user-prompt-submit", one)
	releaseHookLock(root, "user-prompt-submit", one)
	time.Sleep(350 * time.Millisecond)
	if !acquireHookLock(root, "user-prompt-submit", one) {
		t.Fatal("la copia tardía del mismo prompt_id debe descartarse")
	}
	two := []byte(`{"session_id":"s","prompt_id":"p-2","prompt":"igual"}`)
	if acquireHookLock(root, "user-prompt-submit", two) {
		t.Fatal("otro prompt_id con texto idéntico es un turno legítimo")
	}
	releaseHookLock(root, "user-prompt-submit", two)
}

func TestAcquireHookLock_SinReciboTrasAbortoPermiteReintento(t *testing.T) {
	root := t.TempDir()
	payload := []byte(`{"prompt_id":"p-1","prompt":"x"}`)
	if acquireHookLock(root, "user-prompt-submit", payload) {
		t.Fatal("la primera copia debe adquirir")
	}
	releaseHookLock(root, "user-prompt-submit", payload) // simula salir antes de completar
	if acquireHookLock(root, "user-prompt-submit", payload) {
		t.Fatal("sin recibo de éxito el reintento debe poder ejecutarse")
	}
	releaseHookLock(root, "user-prompt-submit", payload)
}

func TestAcquireHookLock_ReciboExpiradoPermiteReintento(t *testing.T) {
	root := t.TempDir()
	payload := []byte(`{"prompt_id":"p-1","prompt":"x"}`)
	if acquireHookLock(root, "user-prompt-submit", payload) {
		t.Fatal("la primera copia debe adquirir")
	}
	markHookCompleted(root, "user-prompt-submit", payload)
	releaseHookLock(root, "user-prompt-submit", payload)
	done := hookDonePath(root, "user-prompt-submit", payload)
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(done, old, old); err != nil {
		t.Fatal(err)
	}
	if acquireHookLock(root, "user-prompt-submit", payload) {
		t.Fatal("un recibo vencido no debe bloquear un reintento")
	}
	releaseHookLock(root, "user-prompt-submit", payload)
	if _, err := os.Stat(done); !os.IsNotExist(err) {
		t.Fatal("el recibo vencido debe purgarse")
	}
}

func TestMarkHookCompleted_SoloPromptSubmit(t *testing.T) {
	root := t.TempDir()
	payload := []byte(`{"prompt_id":"p-1","session_id":"s"}`)
	if acquireHookLock(root, "turn-end", payload) {
		t.Fatal("el primer evento debe adquirir")
	}
	markHookCompleted(root, "turn-end", payload)
	releaseHookLock(root, "turn-end", payload)
	if _, err := os.Stat(hookDonePath(root, "turn-end", payload)); !os.IsNotExist(err) {
		t.Fatal("Stop puede repetirse dentro del mismo prompt y no debe dejar recibo")
	}
}

// C-002 (acr_715249c3): el dueño libera su bloqueo al terminar la espera; así
// un PID reciclado no puede hacer pasar por duplicado un turno legítimo.
func TestReleaseHookLock_ElDuenoLiberaSuBloqueo(t *testing.T) {
	root := t.TempDir()
	payload := []byte(`{"prompt":"y"}`)
	acquireHookLock(root, "turn-end", payload)
	releaseHookLock(root, "turn-end", payload)
	if _, err := os.Stat(hookLockPath(root, "turn-end", payload)); !os.IsNotExist(err) {
		t.Error("el dueño debe borrar su bloqueo")
	}
	ajeno := []byte(`{"prompt":"z"}`)
	p := hookLockPath(root, "turn-end", ajeno)
	_ = os.WriteFile(p, []byte(fmt.Sprintf("%d %d", 999999999, time.Now().Unix())), 0o600)
	releaseHookLock(root, "turn-end", ajeno)
	if _, err := os.Stat(p); err != nil {
		t.Error("nunca se borra un bloqueo ajeno")
	}
}
