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
