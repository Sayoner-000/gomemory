package domain

// CodeNodeKind clasifica los símbolos que el indexador de código extrae de
// los fuentes Go: el archivo mismo, el paquete, y sus símbolos exportados o
// no (funciones, métodos, tipos).
type CodeNodeKind string

const (
	NodeFile     CodeNodeKind = "file"
	NodePackage  CodeNodeKind = "package"
	NodeFunction CodeNodeKind = "function"
	NodeMethod   CodeNodeKind = "method"
	NodeType     CodeNodeKind = "type"
)

// CodeEdgeKind clasifica las relaciones entre nodos del grafo de código.
type CodeEdgeKind string

const (
	// EdgeDefines conecta un nodo de archivo con cada símbolo que declara.
	EdgeDefines CodeEdgeKind = "defines"
	// EdgeImports conecta un nodo de archivo con cada paquete que importa.
	EdgeImports CodeEdgeKind = "imports"
	// EdgeCalls conecta una función/método con cada función/método que llama.
	// Fase 1 resuelve por nombre (mismo paquete o vía import map), sin type
	// checking completo, así que confidence < 1.0 indica una resolución
	// best-effort en vez de exacta.
	EdgeCalls CodeEdgeKind = "calls"
)

func ValidCodeEdgeKind(s string) (CodeEdgeKind, bool) {
	switch CodeEdgeKind(s) {
	case EdgeDefines, EdgeImports, EdgeCalls:
		return CodeEdgeKind(s), true
	default:
		return "", false
	}
}

// CodeNode es un símbolo del código fuente indexado (archivo, paquete,
// función, método o tipo).
type CodeNode struct {
	ID        int64        `json:"id"`
	Project   string       `json:"project"`
	Kind      CodeNodeKind `json:"kind"`
	Name      string       `json:"name"`
	Package   string       `json:"package,omitempty"`
	File      string       `json:"file,omitempty"`
	Receiver  string       `json:"receiver,omitempty"`
	Signature string       `json:"signature,omitempty"`
	StartLine int          `json:"start_line,omitempty"`
	EndLine   int          `json:"end_line,omitempty"`
	Exported  bool         `json:"exported,omitempty"`
}

// CodeEdge es una relación dirigida entre dos CodeNode.
type CodeEdge struct {
	ID         int64        `json:"id"`
	Project    string       `json:"project"`
	FromID     int64        `json:"from_id"`
	ToID       int64        `json:"to_id"`
	Kind       CodeEdgeKind `json:"kind"`
	Confidence float64      `json:"confidence"`
}

// PackageStat resume cuántos símbolos indexó un paquete, usado en el resumen
// de get_context y en graph_status.
type PackageStat struct {
	Package string `json:"package"`
	Symbols int    `json:"symbols"`
}

// GraphStatus es un snapshot del estado del índice de código de un proyecto.
type GraphStatus struct {
	Files         int           `json:"files"`
	Nodes         int           `json:"nodes"`
	Edges         int           `json:"edges"`
	LastIndexedAt string        `json:"last_indexed_at,omitempty"`
	TopPackages   []PackageStat `json:"top_packages,omitempty"`
}
