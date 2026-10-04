package domain

import "strconv"

// Conversation es una interacción continua de una persona con un agente, desde
// que abre hasta que cierra; sobrevive a las compactaciones (feature 035,
// data-model.md). Es la dueña del estado por turno: huella, debounces y aviso
// pendiente. Antes ese estado era por proyecto y pasaba de una conversación a
// la siguiente (el «checkpoint pegado» de Codex).
type Conversation struct {
	ID        string `json:"id"`
	StartedAt int64  `json:"started_at"`
}

// IsNewConversation decide si incomingID abre una conversación nueva. La
// identidad es el session_id del host: una reanudación lo conserva, así que no
// reinicia el estado. Sin id del host no hay con qué comparar: se continúa la
// conversación registrada y solo se abre una si no hay ninguna (C-004 de
// acr_715249c3: antes, un `mem session start` manual o un arranque sin id
// borraban el estado de la conversación en curso).
func IsNewConversation(stored *Conversation, incomingID string) bool {
	if stored == nil {
		return true
	}
	return incomingID != "" && stored.ID != incomingID
}

// ShouldRotate decide si una sesión de memoria lleva demasiado tiempo inactiva
// para seguir abierta. Cubre a Codex, que no tiene evento de fin de sesión, y a
// OpenCode, que solo cierra al terminar el proceso. Un desfase negativo de
// reloj nunca rota.
func ShouldRotate(idleSecs int64) bool {
	return idleSecs > StaleSessionSecs
}

// NewLocalConversationID arma el id de una conversación que el host no
// identificó. suffix lo aporta el adaptador (aleatorio): el dominio no hace I/O.
func NewLocalConversationID(epoch int64, suffix string) string {
	return "local-" + strconv.FormatInt(epoch, 10) + "-" + suffix
}
