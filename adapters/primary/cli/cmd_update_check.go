package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/release"
	"mem/application/ports"
	"mem/domain"
)

// releaseQueryTimeout acota cada consulta a la API de releases (§8).
const releaseQueryTimeout = 5 * time.Second

// NewReleasePort construye el adaptador de releases con las bases
// configurables por entorno (GOMEMORY_RELEASE_API_BASE/_DOWNLOAD_BASE). Lo usa
// el composition root al cablear Deps.
func NewReleasePort() ports.ReleasePort {
	return release.New(releaseAPIBase, releaseDownloadBase, releaseRepo, releaseQueryTimeout)
}

func releasePortOf(deps *Deps) ports.ReleasePort {
	if deps != nil && deps.ReleasePort != nil {
		return deps.ReleasePort
	}
	return NewReleasePort()
}

func updateCheckRepoOf(deps *Deps) ports.UpdateCheckRepository {
	if deps != nil && deps.UpdateCheckRepo != nil {
		return deps.UpdateCheckRepo
	}
	return persistence.UpdateCheckRepository{}
}

// updateCheckDisabled aplica FR-031: la variable de entorno lo apaga para
// todo el usuario; el ajuste, para el proyecto.
func updateCheckDisabled(root string) bool {
	switch os.Getenv("GOMEMORY_NO_UPDATE_CHECK") {
	case "", "0", "false":
	default:
		return true
	}
	return root != "" && persistence.ReadSettings(root).UpdateCheckDisabled
}

// CmdUpdateCheck renueva la caché de versiones. Lo lanza session-start en
// segundo plano (FR-029): sin salida y siempre con código 0.
func CmdUpdateCheck(deps *Deps, _ []string) {
	root, _ := persistence.FindRoot()
	if updateCheckDisabled(root) {
		return
	}
	refreshUpdateCache(context.Background(), releasePortOf(deps), updateCheckRepoOf(deps), time.Now())
}

// refreshUpdateCache consulta la última release con ETag y guarda el
// resultado. Un fallo conserva lo conocido (domain.UpdateCheck.RecordFailure).
func refreshUpdateCache(ctx context.Context, rel ports.ReleasePort, repo ports.UpdateCheckRepository, now time.Time) domain.UpdateCheck {
	cache, _ := repo.Read(ctx)
	tag, etag, _, err := rel.Latest(ctx, cache.ETag)
	if err != nil {
		cache = cache.RecordFailure(now, err)
	} else {
		cache = cache.RecordSuccess(now, tag, etag)
	}
	_ = repo.Write(ctx, cache)
	return cache
}

// spawnUpdateCheck lanza `mem update-check` desacoplado del hook: el arranque
// de la sesión nunca espera a la red (SC-006).
func spawnUpdateCheck(root string) {
	self, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(self, "update-check")
	cmd.Dir = root
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	detach(cmd)
	_ = cmd.Start()
}

func updateNoticePath(root string) string {
	return filepath.Join(root, persistence.MemDir, ".update-notice")
}
