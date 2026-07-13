package domain

// ExportVersion es la versión del formato de bundle portable. Se incrementa si
// el esquema cambia de forma incompatible.
const ExportVersion = 1

// ExportBundle es el formato portable (cross-OS) de un conjunto de memorias y
// sus relaciones, para mover conocimiento entre proyectos y máquinas con
// distinto S.O. Es JSON UTF-8 autocontenido: sin ids acoplados a una base de
// datos (se remapean por RefID) ni rutas absolutas de máquina.
type ExportBundle struct {
	Version    int              `json:"version"`
	ExportedAt string           `json:"exported_at"`
	Source     string           `json:"source_project"`
	Memories   []ExportMemory   `json:"memories"`
	Relations  []ExportRelation `json:"relations"`
}

// ExportMemory es una memoria en el bundle. RefID es el id original en el
// proyecto de origen: sirve para remapear las relaciones al importar (los ids
// reales cambian entre bases). Se excluyen session_id (específico del entorno)
// y project (se remapea al proyecto destino al importar).
type ExportMemory struct {
	RefID        int64  `json:"ref_id"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	Filepath     string `json:"filepath,omitempty"`
	OriginPrompt string `json:"origin_prompt,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ExportRelation es una relación (sinapsis o veredicto de juez) entre dos
// memorias del bundle, referida por sus RefID de origen, no por ids reales.
type ExportRelation struct {
	RefA       int64   `json:"ref_a"`
	RefB       int64   `json:"ref_b"`
	Relation   string  `json:"relation"`
	Confidence float64 `json:"confidence"`
	Reasoning  string  `json:"reasoning,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

// ImportReport resume el resultado de importar un bundle en un proyecto.
type ImportReport struct {
	MemoriesImported  int
	MemoriesSkipped   int
	RelationsImported int
	RelationsSkipped  int
}
