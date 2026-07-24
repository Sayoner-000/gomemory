package domain

import "time"

// CodeProviderSnapshot es el estado cacheado de un proveedor EXTERNO de grafo
// de código (p.ej. codebase-memory-mcp). Se persiste en `.memory/` y es lo
// único que toca el hot path (armar contexto / decidir guardar): una lectura
// local instantánea, nunca una llamada al proveedor. Ver
// application/ports/code_graph_provider.go para el contrato de no-bloqueo.
type CodeProviderSnapshot struct {
	Provider     string            `json:"provider"`
	RootPath     string            `json:"root_path"`
	Available    bool              `json:"available"`
	CheckedAt    time.Time         `json:"checked_at"`
	Architecture *CodeArchitecture `json:"architecture,omitempty"`
}

// Stale indica si el snapshot superó el TTL y conviene disparar un refresco en
// background. Un snapshot vacío (CheckedAt cero) siempre es stale.
func (s CodeProviderSnapshot) Stale(ttl time.Duration) bool {
	return time.Since(s.CheckedAt) > ttl
}

// CodeArchitecture es el resumen COMPACTO del grafo externo que se embebe en
// get_context. No es un volcado del grafo: solo lo justo para orientar al
// agente (totales, lenguajes, módulos de facto y hotspots). Las consultas
// profundas se hacen directo contra las tools del proveedor.
type CodeArchitecture struct {
	TotalNodes int            `json:"total_nodes"`
	TotalEdges int            `json:"total_edges"`
	Languages  []CodeLangStat `json:"languages,omitempty"`
	Clusters   []CodeCluster  `json:"clusters,omitempty"`
	Hotspots   []CodeHotspot  `json:"hotspots,omitempty"`
}

// CodeLangStat es un lenguaje detectado y cuántos archivos aporta.
type CodeLangStat struct {
	Language  string `json:"language"`
	FileCount int    `json:"file_count"`
}

// CodeCluster es un módulo de facto detectado por community detection
// (Leiden/Louvain) sobre el grafo de llamadas/imports: la seam arquitectónica
// real, que a menudo cruza el layout de carpetas.
type CodeCluster struct {
	Label    string   `json:"label"`
	Members  int      `json:"members"`
	Cohesion float64  `json:"cohesion"`
	TopNodes []string `json:"top_nodes,omitempty"`
}

// CodeHotspot es un símbolo muy referenciado (fan-in alto): candidato a punto
// de impacto ante un cambio.
type CodeHotspot struct {
	Name  string `json:"name"`
	FanIn int    `json:"fan_in"`
	// File es la ruta (relativa al root del proyecto) donde vive el símbolo.
	// get_architecture no la expone: se resuelve aparte vía search_code
	// durante Refresh (ver adapters/secondary/codegraph/codebasememory).
	// omitempty: un snapshot cacheado de una versión anterior sigue siendo
	// JSON válido, simplemente sin este dato hasta el próximo refresco.
	File string `json:"file,omitempty"`
}

// CodeImpactAnnotation es el resultado de consultar el snapshot cacheado por
// un filepath concreto (feature 010, Historia 1): no se persiste como
// entidad propia, vive adjunta al content de la memoria que la generó.
type CodeImpactAnnotation struct {
	Hotspot bool
	Symbol  string
	FanIn   int
}
