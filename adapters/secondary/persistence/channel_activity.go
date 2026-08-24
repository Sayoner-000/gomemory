package persistence

import (
	"database/sql"
	"fmt"
	"time"
)

// El registro de actividad responde a una pregunta que el informe de estado no
// podía contestar: ¿este canal sigue funcionando?
//
// El informe comprobaba que el artefacto existiera. Un complemento presente
// cuyo agente renombró la operación que usa queda muerto y el archivo sigue en
// su sitio, así que presencia y salud no son lo mismo. Aquí se anota cuándo se
// ejerció cada canal por última vez, y qué falló si algo falló.

// ChannelActivity es lo que se sabe de la salud de un canal.
type ChannelActivity struct {
	// FiredAt es la última vez que el canal se ejerció correctamente. Cero
	// significa que nunca lo hizo, que es distinto de haberlo hecho hace mucho.
	FiredAt time.Time
	// LastError es el último fallo registrado al intentar ejercerlo. Vacío si
	// no hubo ninguno.
	LastError   string
	LastErrorAt time.Time
}

// RecordChannelActivity anota que un canal acaba de ejercerse.
//
// No borra el último fallo: saber que un canal volvió a funcionar y que antes
// falló son dos datos, y el informe decide qué hacer con cada uno.
func RecordChannelActivity(db *sql.DB, project, agent, scope, kind string) error {
	if project == "" || agent == "" || kind == "" {
		return nil
	}
	_, err := db.Exec(
		`INSERT INTO channel_activity (project, agent, scope, kind, fired_at)
		 VALUES (?, ?, ?, ?, `+Now+`)
		 ON CONFLICT(project, agent, scope, kind) DO UPDATE SET fired_at = excluded.fired_at`,
		project, agent, scope, kind)
	if err != nil {
		return fmt.Errorf("registrar actividad de canal: %w", err)
	}
	return nil
}

// RecordChannelError anota un fallo al ejercer un canal, conservando la fecha
// del último uso correcto.
//
// Es la contrapartida de las rutas de error del complemento, que hasta ahora
// absorbían el fallo sin dejar rastro (FR-012).
func RecordChannelError(db *sql.DB, project, agent, scope, kind, msg string) error {
	if project == "" || agent == "" || kind == "" {
		return nil
	}
	_, err := db.Exec(
		`INSERT INTO channel_activity (project, agent, scope, kind, last_error, last_error_at)
		 VALUES (?, ?, ?, ?, ?, `+Now+`)
		 ON CONFLICT(project, agent, scope, kind) DO UPDATE SET
		   last_error = excluded.last_error,
		   last_error_at = excluded.last_error_at`,
		project, agent, scope, kind, msg)
	if err != nil {
		return fmt.Errorf("registrar fallo de canal: %w", err)
	}
	return nil
}

// LastChannelActivity devuelve lo que se sabe de un canal. ok=false significa
// que nunca se ejerció ni falló: no hay nada que reportar sobre él.
func LastChannelActivity(db *sql.DB, project, agent, scope, kind string) (ChannelActivity, bool) {
	var firedAt, errAt sql.NullString
	var lastErr string
	err := db.QueryRow(
		`SELECT fired_at, last_error, last_error_at FROM channel_activity
		 WHERE project = ? AND agent = ? AND scope = ? AND kind = ?`,
		project, agent, scope, kind).Scan(&firedAt, &lastErr, &errAt)
	if err != nil {
		return ChannelActivity{}, false
	}
	return ChannelActivity{
		FiredAt:     parseStoredTime(firedAt),
		LastError:   lastErr,
		LastErrorAt: parseStoredTime(errAt),
	}, true
}

// SessionsSince cuenta las sesiones abiertas desde un instante.
//
// Es lo que permite distinguir un canal inactivo porque nadie trabajó de uno
// que no responde habiendo trabajo (FR-011). Sin este dato, una semana de
// vacaciones se reportaría igual que un complemento roto.
//
// Devuelve -1 si la consulta falla: "cero sesiones" y "no se pudo saber" son
// información distinta, y el informe no debe fingir un dato que no tiene.
func SessionsSince(db *sql.DB, project string, since time.Time) int {
	var n int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM sessions WHERE project = ? AND created_at >= ?`,
		project, since.Format("2006-01-02 15:04:05")).Scan(&n)
	if err != nil {
		return -1
	}
	return n
}

// parseStoredTime interpreta las marcas de tiempo del esquema, que se guardan
// en hora local sin zona. Un valor ausente o ilegible produce el cero, que los
// llamadores ya tratan como "nunca".
func parseStoredTime(v sql.NullString) time.Time {
	if !v.Valid || v.String == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.ParseInLocation(layout, v.String, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ChannelActivityRepository adapta el registro de actividad a su puerto.
type ChannelActivityRepository struct {
	db      *sql.DB
	project string
}

func NewChannelActivityRepository(db *sql.DB, project string) *ChannelActivityRepository {
	return &ChannelActivityRepository{db: db, project: project}
}

func (r *ChannelActivityRepository) RecordFired(agent, scope, kind string) error {
	return RecordChannelActivity(r.db, r.project, agent, scope, kind)
}

func (r *ChannelActivityRepository) RecordError(agent, scope, kind, msg string) error {
	return RecordChannelError(r.db, r.project, agent, scope, kind, msg)
}

func (r *ChannelActivityRepository) Last(agent, scope, kind string) (time.Time, string, bool) {
	a, ok := LastChannelActivity(r.db, r.project, agent, scope, kind)
	return a.FiredAt, a.LastError, ok
}

func (r *ChannelActivityRepository) SessionsSince(since time.Time) int {
	return SessionsSince(r.db, r.project, since)
}
