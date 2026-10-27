package usecases

import (
	"fmt"
	"strings"
	"testing"

	"mem/domain"
)

// memoriasFijas devuelve siempre las mismas memorias: el filtro que se prueba
// es el de trabajoCubierto, no el ranking de la búsqueda.
type memoriasFijas struct {
	fakeCaptureMemories
	mems []domain.Memory
}

func (f *memoriasFijas) Search(project, query string, limit int) ([]domain.Memory, error) {
	return f.mems, nil
}

// relleno genera n palabras distintas sin relación con ningún objetivo.
func relleno(n int) string {
	palabras := make([]string, n)
	for i := range palabras {
		palabras[i] = fmt.Sprintf("relleno%d", i)
	}
	return strings.Join(palabras, " ")
}

const objetivoRevision = "Revisión independiente de seguridad de rutas y adaptadores: autenticación, validación de entrada y fugas en errores"

func cubierto(t *testing.T, mems ...domain.Memory) bool {
	t.Helper()
	uc := NewRouteTaskUseCase(nil).WithMemoryRepository(&memoriasFijas{mems: mems})
	return uc.trabajoCubierto("p", objetivoRevision)
}

// TestTrabajoCubierto_DocumentoLargoConTerminosDispersosNoEsDuplicado: una
// memoria extensa (la constitución de un proyecto real, 29 000 caracteres)
// contenía casi todas las palabras de cualquier objetivo repartidas por el
// texto, y Octopus devolvía INLINE por "trabajo duplicado" para todo. Tener
// las palabras sueltas no es haber hecho el trabajo.
func TestTrabajoCubierto_DocumentoLargoConTerminosDispersosNoEsDuplicado(t *testing.T) {
	var partes []string
	for _, termino := range strings.Fields("revisión independiente seguridad rutas adaptadores autenticación validación entrada fugas errores") {
		partes = append(partes, relleno(80), termino)
	}
	constitucion := domain.Memory{Type: domain.Architecture, Title: "Constitución del proyecto", Content: strings.Join(partes, " ")}

	if cubierto(t, constitucion) {
		t.Error("un documento largo con los términos dispersos no debe marcar el objetivo como trabajo duplicado")
	}
}

// TestTrabajoCubierto_DocumentoLargoQueContieneElObjetivoSiEsDuplicado
// conserva la decisión original: una memoria extensa que contiene el objetivo
// completo en un pasaje no debe perder por su longitud.
func TestTrabajoCubierto_DocumentoLargoQueContieneElObjetivoSiEsDuplicado(t *testing.T) {
	informe := domain.Memory{
		Type:    domain.Discovery,
		Title:   "Informe de auditoría",
		Content: relleno(2000) + " Se completó la " + objetivoRevision + ". " + relleno(2000),
	}

	if !cubierto(t, informe) {
		t.Error("una memoria larga que contiene el objetivo completo debe marcarlo como trabajo duplicado")
	}
}

func TestTrabajoCubierto_MemoriaBreveEquivalenteEsDuplicado(t *testing.T) {
	breve := domain.Memory{Type: domain.Discovery, Title: "ACR de seguridad", Content: objetivoRevision + ": dos hallazgos confirmados."}

	if !cubierto(t, breve) {
		t.Error("una memoria breve que describe el mismo trabajo debe marcarlo como trabajo duplicado")
	}
}

func TestTrabajoCubierto_CheckpointNoCuenta(t *testing.T) {
	checkpoint := domain.Memory{Type: domain.Checkpoint, Title: "checkpoint", Content: objetivoRevision}

	if cubierto(t, checkpoint) {
		t.Error("un checkpoint automático no es trabajo curado y no debe contar como duplicado")
	}
}
