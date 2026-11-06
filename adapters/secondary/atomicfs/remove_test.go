package atomicfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var errEnUso = errors.New("en uso por otro proceso")

func enUso(err error) bool { return errors.Is(err, errEnUso) }

// HCD-039: en Windows un ejecutable recién ejecutado sigue bloqueado un
// instante; el borrado reintenta el fallo transitorio hasta lograrlo.
func TestRemove_ReintentaTransitoriosHastaLograrlo(t *testing.T) {
	intentos := 0
	err := retry(func() error {
		intentos++
		if intentos < 3 {
			return errEnUso
		}
		return nil
	}, enUso, time.Second)
	if err != nil || intentos != 3 {
		t.Fatalf("err=%v intentos=%d; quiero nil y 3", err, intentos)
	}
}

// Un fallo que no es transitorio se devuelve al primer intento.
func TestRemove_NoTransitorioNoReintenta(t *testing.T) {
	intentos := 0
	errFijo := errors.New("no existe")
	err := retry(func() error { intentos++; return errFijo }, enUso, time.Second)
	if !errors.Is(err, errFijo) || intentos != 1 {
		t.Fatalf("err=%v intentos=%d; quiero errFijo y 1", err, intentos)
	}
}

// Un bloqueo que no se libera devuelve el error al agotar el plazo.
func TestRemove_AgotaElPlazo(t *testing.T) {
	err := retry(func() error { return errEnUso }, enUso, 20*time.Millisecond)
	if !errors.Is(err, errEnUso) {
		t.Fatalf("err=%v; quiero errEnUso", err)
	}
}

func TestRemove_BorraElArchivo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.exe")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Remove(path); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("el archivo sigue: %v", err)
	}
	if err := Remove(path); !os.IsNotExist(err) {
		t.Fatalf("borrar lo inexistente: %v; quiero IsNotExist", err)
	}
}
