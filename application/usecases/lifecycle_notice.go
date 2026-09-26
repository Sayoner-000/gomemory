package usecases

import (
	"time"

	"mem/domain"
)

// UpdateNoticeInput reúne lo que decide el aviso de versión en session-start.
// Es un valor para que la decisión sea pura y determinista en pruebas (el
// reloj entra como Now, constitución §10).
type UpdateNoticeInput struct {
	// Current es la versión instalada (version.Version, sin "v").
	Current    string
	Cache      domain.UpdateCheck
	CacheOK    bool
	Now        time.Time
	Disabled   bool
	LastNotice domain.UpdateNotice
	SessionID  string
}

// UpdateNoticeDecision dice si hay que avisar, si hay que refrescar la caché
// en segundo plano y qué marcar para no repetir el aviso.
type UpdateNoticeDecision struct {
	Notice        string
	RefreshNeeded bool
	Mark          *domain.UpdateNotice
}

// DecideUpdateNotice aplica FR-028…FR-031: sin nada que hacer si el aviso está
// desactivado; refresco si la caché falta o venció; aviso si hay una release
// estable más nueva que no se anunció ya en esta sesión.
func DecideUpdateNotice(in UpdateNoticeInput) UpdateNoticeDecision {
	if in.Disabled {
		return UpdateNoticeDecision{}
	}
	d := UpdateNoticeDecision{RefreshNeeded: !in.CacheOK || in.Cache.Stale(in.Now)}
	if !in.CacheOK {
		return d
	}
	latest, ok := domain.ParseVersion(in.Cache.Latest)
	if !ok {
		return d
	}
	current, ok := domain.ParseVersion(in.Current)
	if !ok || !domain.Newer(latest, current) {
		return d
	}
	if in.LastNotice.Version == latest.String() && in.LastNotice.Session == in.SessionID {
		return d
	}
	d.Notice = "gomemory " + latest.String() + " disponible (tienes " + current.String() + ") → mem update"
	d.Mark = &domain.UpdateNotice{Version: latest.String(), Session: in.SessionID}
	return d
}
