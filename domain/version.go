package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// UpdateCheckTTL es el intervalo mínimo entre dos consultas de la última
// versión publicada (FR-028, mismo criterio que Codex).
const UpdateCheckTTL = 24 * time.Hour

// Version es una versión semántica de gomemory. Pre va vacío en las releases
// estables.
type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

// ParseVersion acepta "v2.26.4" y "2.26.4" (con espacios alrededor) y rechaza
// cualquier otra forma, incluida la salida completa de `mem version`.
func ParseVersion(s string) (Version, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p == "" || strings.HasPrefix(p, "+") {
			return Version{}, false
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2], Pre: pre}, true
}

func (v Version) String() string {
	s := fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}

// Newer indica si a es una release estable posterior a b. Una prerrelease
// nunca cuenta como más nueva: el aviso de versión solo anuncia releases
// estables (R4).
func Newer(a, b Version) bool {
	if a.Pre != "" {
		return false
	}
	if a.Major != b.Major {
		return a.Major > b.Major
	}
	if a.Minor != b.Minor {
		return a.Minor > b.Minor
	}
	if a.Patch != b.Patch {
		return a.Patch > b.Patch
	}
	return b.Pre != "" // 2.27.0 es posterior a 2.27.0-rc.1
}
