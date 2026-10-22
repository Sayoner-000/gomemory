package domain

import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

var skillCodeTool = regexp.MustCompile("`([a-z]+(?:_[a-z]+)+)`")
var skillPackageManager = regexp.MustCompile(`\b(npm|npx|pnpm|yarn)\b`)

// ValidateSkillBody validates portable skills before publication or installation.
func ValidateSkillBody(name, body string, knownTools []string) []string {
	var errs []string
	fm, _, ok := ParseFrontmatter([]byte(body))
	if !ok || fm["name"] != name {
		errs = append(errs, "frontmatter name must match directory")
	}
	if n := utf8.RuneCountInString(fm["description"]); n == 0 || n > 300 {
		errs = append(errs, "description must contain 1–300 characters")
	}
	for _, section := range []string{"## Capacidades", "## Procedimiento"} {
		if !strings.Contains(body, section) {
			errs = append(errs, "missing "+section)
		}
	}
	for _, word := range []string{"~/.claude", "~/.codex", "~/.config/opencode"} {
		if strings.Contains(body, word) {
			errs = append(errs, "harness path: "+word)
		}
	}
	if skillPackageManager.MatchString(body) {
		errs = append(errs, "package manager commands are not portable")
	}
	for _, m := range skillCodeTool.FindAllStringSubmatch(body, -1) {
		if !slices.Contains(knownTools, m[1]) {
			errs = append(errs, "unknown MCP tool: "+m[1])
		}
	}
	table := false
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "## Por harness") {
			table = true
			continue
		}
		if strings.HasPrefix(line, "## ") {
			table = false
		}
		for _, cmd := range []string{"/compact", "/context", "/status"} {
			if strings.Contains(line, cmd) && !(table && strings.HasPrefix(strings.TrimSpace(line), "|")) {
				errs = append(errs, "native command outside harness table: "+cmd)
			}
		}
	}
	if !strings.Contains(body, "Inspirada en ") || !strings.Contains(body, "de ECC (affaan-m/ECC, MIT). Reinterpretada para gomemory; ver NOTICE.") {
		errs = append(errs, "missing ECC attribution")
	}
	return errs
}
