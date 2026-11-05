package domain

// StorageOptimizationReport contiene el balance detallado del saneamiento y
// optimización de almacenamiento ejecutado sobre un proyecto.
type StorageOptimizationReport struct {
	Project string `json:"project"`
	DryRun  bool   `json:"dry_run"`

	OrphanRelationsCleaned      int64 `json:"orphan_relations_cleaned"`
	DuplicateRelationsMerged    int64 `json:"duplicate_relations_merged"`
	SelfRelationsCleaned        int64 `json:"self_relations_cleaned"`
	OrphanCodeNodesCleaned      int64 `json:"orphan_code_nodes_cleaned"`
	OrphanCodeEdgesCleaned      int64 `json:"orphan_code_edges_cleaned"`
	UnreferencedPackagesCleaned int64 `json:"unreferenced_packages_cleaned"`
	FTSOptimized                bool  `json:"fts_optimized"`
	DatabaseOptimized           bool  `json:"database_optimized"`
}

// TotalCleaned devuelve el total de elementos anómalos o huérfanos retirados.
func (r StorageOptimizationReport) TotalCleaned() int64 {
	return r.OrphanRelationsCleaned + r.DuplicateRelationsMerged + r.SelfRelationsCleaned +
		r.OrphanCodeNodesCleaned + r.OrphanCodeEdgesCleaned + r.UnreferencedPackagesCleaned
}

// ExternalProviderOptimizeReport resume el resultado del mantenimiento sobre
// un proveedor externo de grafos (codebase-memory-mcp o codegraph).
type ExternalProviderOptimizeReport struct {
	Provider         string `json:"provider"`
	Available        bool   `json:"available"`
	SnapshotsCleaned int    `json:"snapshots_cleaned"`
	StatusSummary    string `json:"status_summary"`
	Reindexed        bool   `json:"reindexed"`
	Nodes            int    `json:"nodes,omitempty"`
	Edges            int    `json:"edges,omitempty"`
}
