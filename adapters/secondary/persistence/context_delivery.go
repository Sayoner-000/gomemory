package persistence

import (
	"database/sql"
	"fmt"
)

// El registro de entregas responde a una pregunta concreta: qué material ya
// recibió el agente en esta sesión.
//
// Sin él, la operación de contexto para planificar reenviaba íntegro el
// historial que la operación de contexto general acababa de entregar. Medido:
// 180 de 180 líneas idénticas, unos 6.100 tokens cobrados dos veces.
//
// El registro se acota a la sesión a propósito (FR-012). Una sesión nueva no
// hereda la creencia de que el contexto ya se entregó, porque heredarla dejaría
// al agente arrancando sin él.

// RecordContextDelivery anota qué se entregó por un canal en una sesión.
// La clave primaria es el par sesión-canal, así que una entrega posterior
// reemplaza a la anterior: lo que importa es lo último que el agente recibió.
func RecordContextDelivery(db *sql.DB, sessionID, kind, contentHash string) error {
	if sessionID == "" || kind == "" {
		return nil // sin sesión activa no hay nada que acotar
	}
	_, err := db.Exec(
		`INSERT INTO context_deliveries (session_id, kind, content_hash, delivered_at)
		 VALUES (?, ?, ?, `+Now+`)
		 ON CONFLICT(session_id, kind) DO UPDATE SET
		   content_hash = excluded.content_hash,
		   delivered_at = excluded.delivered_at`,
		sessionID, kind, contentHash)
	if err != nil {
		return fmt.Errorf("registrar entrega de contexto: %w", err)
	}
	return nil
}

// LastContextDelivery devuelve el identificador del contenido entregado por un
// canal en una sesión. ok=false significa que ese canal no ha entregado nada
// en esa sesión, que es distinto de haber entregado algo vacío.
func LastContextDelivery(db *sql.DB, sessionID, kind string) (string, bool) {
	if sessionID == "" || kind == "" {
		return "", false
	}
	var h string
	err := db.QueryRow(
		`SELECT content_hash FROM context_deliveries WHERE session_id = ? AND kind = ?`,
		sessionID, kind).Scan(&h)
	if err != nil {
		return "", false
	}
	return h, true
}

// DeliveryLogRepository adapta el registro de entregas a su puerto.
//
// Resuelve la sesión activa en CADA llamada y no al construirse: una sesión
// puede abrirse o cerrarse durante la vida del proceso, y un identificador
// capturado al arrancar acotaría la supresión a una sesión que ya terminó.
type DeliveryLogRepository struct {
	db      *sql.DB
	project string
}

func NewDeliveryLogRepository(db *sql.DB, project string) *DeliveryLogRepository {
	return &DeliveryLogRepository{db: db, project: project}
}

// sessionID devuelve la sesión activa, o cadena vacía si no hay ninguna. Sin
// sesión no hay ámbito al que acotar la supresión, y las operaciones quedan
// inertes: el documento se entrega completo.
func (r *DeliveryLogRepository) sessionID() string {
	s, err := ActiveSession(r.db, r.project)
	if err != nil || s == nil {
		return ""
	}
	return s.ID
}

func (r *DeliveryLogRepository) Last(kind string) (string, bool) {
	return LastContextDelivery(r.db, r.sessionID(), kind)
}

func (r *DeliveryLogRepository) Record(kind, hash string) error {
	return RecordContextDelivery(r.db, r.sessionID(), kind, hash)
}
