package ports

// CompressionCoverageScope evita trasladar una observación a otro agente,
// versión, modalidad o clase de contenido. No contiene datos de la sesión.
type CompressionCoverageScope struct {
	Agent        string `json:"agent"`
	Version      string `json:"version"`
	Mode         string `json:"mode"`
	ContentClass string `json:"content_class"`
}

// CompressionCoverageEvidence describe una observación, no una certificación
// criptográfica. Source identifica su procedencia; las huellas evitan guardar
// contenido crudo. Configuración, ejecución y consumo son hechos distintos.
type CompressionCoverageEvidence struct {
	Scope           CompressionCoverageScope `json:"scope"`
	Mechanism       string                   `json:"mechanism"`
	Configured      bool                     `json:"configured"`
	Executed        bool                     `json:"executed"`
	Consumed        bool                     `json:"consumed"`
	Visibility      string                   `json:"visibility"`
	OriginalSHA256  string                   `json:"original_sha256,omitempty"`
	DeliveredSHA256 string                   `json:"delivered_sha256,omitempty"`
	OriginalBytes   int64                    `json:"original_bytes"`
	DeliveredBytes  int64                    `json:"delivered_bytes"`
	Partial         bool                     `json:"partial"`
	Unavailable     bool                     `json:"unavailable"`
	Reason          string                   `json:"reason"`
	Source          string                   `json:"source"`
}

// CompressionCoverageAssessment expresa únicamente la cobertura del Scope.
// verified no demuestra ahorro neto, facturación ni cobertura de toda la sesión.
type CompressionCoverageAssessment struct {
	Scope     CompressionCoverageScope `json:"scope"`
	Mechanism string                   `json:"mechanism,omitempty"`
	Status    string                   `json:"status"`
	Reason    string                   `json:"reason"`
}

// CompressionCoverageSnapshot aporta alcances actuales y observaciones al
// diagnóstico sin ejecutar sondas. nil significa que no hay capturas cargadas.
// El productor debe limitar las observaciones al proyecto correspondiente.
type CompressionCoverageSnapshot struct {
	Scopes   []CompressionCoverageScope
	Evidence []CompressionCoverageEvidence
}
