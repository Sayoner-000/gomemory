package atomicfs

import (
	"os"
	"time"
)

// removeTimeout acota la espera del borrado, como replaceTimeout la del
// reemplazo: más allá, el archivo sigue en uso por algo que no es breve.
const removeTimeout = 2 * time.Second

// Remove borra path. En Windows un ejecutable que acaba de terminar sigue
// bloqueado un instante (cierre del proceso, antivirus) y os.Remove falla;
// Remove reintenta esos fallos transitorios durante un intervalo corto
// (HCD-039). En el resto de sistemas no hay fallo transitorio: es os.Remove.
func Remove(path string) error {
	return retry(func() error { return os.Remove(path) }, transient, removeTimeout)
}

// retry repite op mientras falle con un error transitorio y no venza el plazo,
// con espera exponencial hasta 64 ms.
func retry(op func() error, isTransient func(error) bool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	wait := time.Millisecond
	for {
		err := op()
		if err == nil || !isTransient(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(wait)
		if wait < 64*time.Millisecond {
			wait *= 2
		}
	}
}
