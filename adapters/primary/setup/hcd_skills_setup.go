package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func InstallHCDSkills(bodies, wrappers map[string]string) ([]string, error) {
	var names []string
	for name := range bodies {
		if name != "hcd" && !strings.HasPrefix(name, "hcd-") {
			return nil, fmt.Errorf("invalid HCD skill name")
		}
		if strings.ContainsAny(name, "/\\.") {
			return nil, fmt.Errorf("invalid HCD skill name")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var written []string
	for _, name := range names {
		paths, e := InstallAgentSkill(name, bodies[name])
		written = append(written, paths...)
		if e != nil {
			return written, e
		}
	}
	home, e := os.UserHomeDir()
	if e != nil {
		return written, e
	}
	base := filepath.Join(home, ".config", "opencode")
	if info, e := os.Stat(base); e != nil || !info.IsDir() {
		return written, nil
	}
	for _, name := range names {
		body := wrappers[name+".md"]
		if strings.TrimSpace(bodies[name]) == "" || strings.TrimSpace(body) == "" {
			continue
		}
		dest := filepath.Join(base, "commands", name+".md")
		previous, e := os.ReadFile(dest)
		if e == nil && string(previous) == body {
			continue
		}
		if e != nil && !os.IsNotExist(e) {
			return written, e
		}
		if e = writeManagedFile(dest, []byte(body)); e != nil {
			return written, e
		}
		written = append(written, dest)
	}
	return written, nil
}
