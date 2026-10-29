package cli

import (
	"context"
	"fmt"
	"mem/application/usecases"
	"mem/domain"
	"os"
	"time"
)

// withDialectClient completa el cliente del heartbeat HCD cuando el payload no
// lo trae: Codex no lo envía, pero es el único que registra turn-end con
// --emit=json. Sin esto, su heartbeat quedaba atribuido a Claude.
func withDialectClient(payload map[string]any, dialect hookDialect) map[string]any {
	if payload == nil || dialect != dialectJSON {
		return payload
	}
	if client, _ := payload["client"].(string); client == "" {
		payload["client"] = "codex"
	}
	return payload
}

func recordHCDHeartbeat(deps *Deps, root, project string, payload map[string]any, activity turnActivity) {
	beat := deps.HCDHeartbeat
	if beat == nil {
		if deps.HCD == nil {
			return
		}
		beat = deps.HCD.Heartbeat
	}
	settings := domain.HCDSettings{}
	if deps.SettingsRepo != nil {
		settings = deps.SettingsRepo.Read(root).HCDSettings
	}
	validated, e := domain.ValidateHCDSettings(settings)
	if e != nil {
		fmt.Fprintf(os.Stderr, "mem hook: hcd heartbeat: %v; se ignora\n", e)
		return
	}
	sessionID, _ := payload["session_id"].(string)
	if sessionID == "" && deps.SessionRepo != nil {
		if session, e := deps.SessionRepo.Active(project); e == nil && session != nil {
			sessionID = session.ID
		}
	}
	client, _ := payload["client"].(string)
	if client == "" {
		client = "claude"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*validated.HeartbeatTimeoutMS)*time.Millisecond)
	defer cancel()
	if e = beat(ctx, usecases.HCDHeartbeatInput{SessionID: sessionID, Client: client, Files: activity.Files}); e != nil {
		fmt.Fprintf(os.Stderr, "mem hook: hcd heartbeat: %v; se ignora\n", e)
	}
}
