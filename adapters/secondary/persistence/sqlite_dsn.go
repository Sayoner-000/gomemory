package persistence

import "fmt"

// sqliteDSN configura cada conexión del pool, no solo la primera.
// busy_timeout va primero para esperar bloqueos al activar WAL.
// WAL permite lecturas concurrentes con el escritor.
// NORMAL reduce sincronizaciones: conserva commits ante una caída del proceso,
// pero un corte de energía puede perder la última escritura sin corromper la base.
// cache_size reserva hasta unos 16 MB bajo demanda.
// temp_store evita archivos temporales para ordenar y agregar.
// mmap_size permite lectura mapeada hasta 256 MB cuando el sistema la admite.
// journal_size_limit acota el WAL retenido tras un checkpoint a 64 MB.
// immediate evita carreras al promover una transacción lectora a escritora.
func sqliteDSN(path string, busyMs int) string {
	return fmt.Sprintf("%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-16000)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(268435456)&_pragma=journal_size_limit(67108864)&_txlock=immediate", path, busyMs)
}
