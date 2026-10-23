// Package atomicfs reemplaza un archivo por un temporal ya escrito.
package atomicfs

// Replace renombra tmp sobre dst. Los lectores ven el contenido anterior o el
// nuevo, nunca uno a medias. En Windows, rename falla mientras otro proceso
// tiene abierto dst o lo está reemplazando a la vez (los hooks corren en
// paralelo); Replace reintenta durante un intervalo corto, como hace cmd/go
// con internal/robustio.
func Replace(tmp, dst string) error {
	return replace(tmp, dst)
}

// ReadFile lee un archivo que se reemplaza con Replace. En Windows lo abre
// compartiendo también el borrado: os.ReadFile no lo hace, y mientras un
// lector lo tiene abierto ningún Replace puede sustituirlo. Además reintenta
// los fallos transitorios de abrirlo justo mientras se reemplaza.
func ReadFile(path string) ([]byte, error) {
	return readFile(path)
}
