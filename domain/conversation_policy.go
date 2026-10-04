package domain

// Política del estado por conversación y de la guarda de hooks (feature 035).
// Es la ÚNICA fuente de estas cifras, igual que compression_policy.go.
const (
	// StaleSessionSecs: inactividad tras la que una sesión de memoria se cierra
	// al iniciar una conversación nueva (4 h, aclaración de la spec).
	StaleSessionSecs = 4 * 3600
	// HookLockPurgeSecs: los bloqueos de reentrada más viejos se purgan.
	HookLockPurgeSecs = 60
	// HookCompletedReceiptSecs: plazo para reconocer una copia tardía del
	// mismo prompt_id después de que el primer hook terminó correctamente.
	// Es una ventana de deduplicación del evento del host, no la duración física
	// de dos procesos solapados. La guarda ofrece ejecución a lo sumo una vez
	// durante este plazo; no puede confirmar si el host recibió la primera salida.
	HookCompletedReceiptSecs = 3600
)
