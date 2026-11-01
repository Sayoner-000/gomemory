package domain

import "errors"

// ErrLimpiezaPendiente indica que las memorias ya se borraron y solo quedó
// pendiente una limpieza posterior que otro proceso impide terminar. Las
// superficies lo informan como éxito con advertencia. Cualquier otro error tras
// confirmar el borrado, como corrupción o disco lleno, sigue siendo un fallo.
var ErrLimpiezaPendiente = errors.New("limpieza pendiente")

// MinContenidoIntegro evita que un contenido breve, como «ok», cuente como vivo por
// aparecer dentro de otro texto: por debajo de este tamaño se exige igualdad.
const MinContenidoIntegro = 32
