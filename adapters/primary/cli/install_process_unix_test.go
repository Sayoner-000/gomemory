//go:build !windows

package cli

import (
	"context"
	"os"
	"syscall"
	"testing"
	"time"
)

// S-001: solo el líder de su grupo cierra el grupo entero; cualquier otro
// proceso (p. ej. alguien que exporta a mano la variable interna) se termina
// solo a sí mismo y nunca al grupo de quien lo lanzó.
func TestOwnGroupTarget_SoloElLiderApuntaAlGrupo(t *testing.T) {
	pgid, err := syscall.Getpgid(0)
	if err != nil {
		t.Fatal(err)
	}
	want := os.Getpid()
	if pgid == os.Getpid() {
		want = -os.Getpid()
	}
	if got := ownGroupTarget(); got != want {
		t.Fatalf("objetivo = %d, esperado %d (pgid %d, pid %d)", got, want, pgid, os.Getpid())
	}
	if pgid == os.Getpid() {
		t.Skip("el proceso de prueba es líder de grupo; el caso no líder no aplica")
	}
}

// S-002 (ronda 7): el puente detecta que el cliente dejó de leer aunque haya
// arrancado ya adoptado (ppid 1, sin cambio de padre que observar).
func TestWatchOutput_DetectaQueElLectorSeCierra(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	gone := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watchOutput(ctx, int(w.Fd()), func() { close(gone) })
	select {
	case <-gone:
		t.Fatal("con el lector abierto no debe dispararse")
	case <-time.After(300 * time.Millisecond):
	}
	_ = r.Close()
	select {
	case <-gone:
	case <-time.After(2 * time.Second):
		t.Fatal("no detectó el cierre del lector")
	}
}
