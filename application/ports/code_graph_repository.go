package ports

import "mem/domain"

// CodeGraphRepository persiste el grafo de código indexado de un proyecto:
// archivos con su hash (para reindexado incremental), símbolos (CodeNode) y
// relaciones entre ellos (CodeEdge).
type CodeGraphRepository interface {
	// FileHashes devuelve el hash conocido de cada archivo ya indexado,
	// usado por el indexador para diffear qué archivos cambiaron.
	FileHashes(project string) (map[string]string, error)

	// ReplaceFile reemplaza atómicamente todos los nodos que un archivo
	// declara: borra los nodos/aristas viejos de ese archivo, inserta los
	// nuevos, y devuelve los nodos insertados con sus IDs ya asignados (para
	// que el indexador pueda resolver aristas de CALLS en una segunda
	// pasada). Crea o actualiza la fila de code_files con el hash dado.
	ReplaceFile(project, path, hash string, nodes []domain.CodeNode) ([]domain.CodeNode, error)

	// DeleteFile borra un archivo y todo lo que declaró (nodos y aristas
	// originadas en él), usado cuando el indexador detecta que el archivo ya
	// no existe en disco.
	DeleteFile(project, path string) error

	// InsertEdges reemplaza las aristas originadas en un archivo (borra las
	// viejas de ese origen e inserta las nuevas), usado en la segunda pasada
	// de resolución de imports/calls tras persistir los nodos.
	InsertEdges(project, srcPath string, edges []domain.CodeEdge) error

	// SearchNodes busca símbolos por nombre/firma/paquete (FTS5 si está
	// disponible, LIKE como fallback).
	SearchNodes(project, query string, limit int) ([]domain.CodeNode, error)

	// NodesByName devuelve los nodos cuyo nombre coincide exactamente,
	// usado para resolver el destino de una arista y por get_symbol.
	NodesByName(project, name string) ([]domain.CodeNode, error)

	// UpsertPackageNode devuelve (creando si hace falta) el nodo paquete
	// para una ruta de import dada. Los nodos de paquete no pertenecen a un
	// archivo específico (son el destino externo de un import), así que
	// sobreviven al reindexado de cualquier archivo individual.
	UpsertPackageNode(project, importPath string) (domain.CodeNode, error)

	// Neighbors hace un BFS acotado por profundidad sobre code_edges desde
	// un nodo, filtrando por tipo de arista y dirección ("in"|"out"|"both").
	Neighbors(project string, nodeID int64, kind domain.CodeEdgeKind, direction string, depth int) ([]domain.CodeNode, []domain.CodeEdge, error)

	// Status resume el tamaño del grafo (conteos + paquetes top) para
	// graph_status y el resumen en get_context.
	Status(project string) (domain.GraphStatus, error)
}
