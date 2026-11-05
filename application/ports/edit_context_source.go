package ports

import "mem/domain"

// EditContextSource da, para un archivo, lo que el proyecto ya sabe de él
// (HCD-028): las memorias ancladas a su ruta y las pruebas que llaman a sus
// símbolos según el grafo propio. Solo lecturas locales: corre dentro del
// plazo del hook de edición.
type EditContextSource interface {
	// MemoriesByFilepath devuelve las memorias cuyo filepath es alguna de
	// paths, de la más reciente a la más antigua.
	MemoriesByFilepath(project string, paths []string, limit int) ([]domain.Memory, error)
	// TestsCovering devuelve las funciones Test* que llaman a algún símbolo
	// definido en path.
	TestsCovering(project, path string, limit int) ([]domain.EditContextTest, error)
}
