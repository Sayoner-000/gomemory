package usecases

import (
	"context"
	"fmt"
	"strings"

	"mem/application/ports"
)

func validAutomaticScope(scope ports.AutomaticOriginalScope) bool {
	return strings.TrimSpace(scope.Project) != "" && strings.TrimSpace(scope.SessionID) != "" &&
		(scope.Agent == "codex" || scope.Agent == "claude" || scope.Agent == "opencode")
}

// GrantAutomaticOriginals is called after persistence and before publication.
// It never infers session identity from current process or project state.
func GrantAutomaticOriginals(ctx context.Context, originals ports.OriginalStoreRepository, grants ports.AutomaticOriginalGrantsRepository, scope ports.AutomaticOriginalScope, refs []string) error {
	if !validAutomaticScope(scope) || originals == nil || grants == nil || len(refs) == 0 {
		return ErrOriginalNotFound
	}
	for _, ref := range refs {
		if _, _, err := RetrieveOriginal(ctx, originals, scope.Project, ref); err != nil {
			return err
		}
	}
	if err := grants.Grant(ctx, scope, refs); err != nil {
		return fmt.Errorf("grant automatic original access: %w", err)
	}
	return nil
}

// RetrieveAutomaticOriginal checks the grant before touching the original, so
// a foreign session cannot read content or renew its lifetime. Manual CLI/MCP
// retrieval continues to use RetrieveOriginal with its existing project scope.
func RetrieveAutomaticOriginal(ctx context.Context, originals ports.OriginalStoreRepository, grants ports.AutomaticOriginalGrantsRepository, scope ports.AutomaticOriginalScope, ref string) (string, ports.OriginalMeta, error) {
	if !validAutomaticScope(scope) || originals == nil || grants == nil {
		return "", ports.OriginalMeta{}, ErrOriginalNotFound
	}
	ref = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ref), "ref="))
	allowed, err := grants.Allows(ctx, scope, ref)
	if err != nil {
		return "", ports.OriginalMeta{}, fmt.Errorf("check automatic original access: %w", err)
	}
	if !allowed {
		return "", ports.OriginalMeta{}, ErrOriginalNotFound
	}
	return RetrieveOriginal(ctx, originals, scope.Project, ref)
}
