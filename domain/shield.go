package domain

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// ShieldFinding es un hallazgo del escaneo de configuración de los agentes
// (feature 040, contracts/shield.md). Detail nunca contiene el valor de un
// secreto.
type ShieldFinding struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Category string `json:"category"`
	Severity string `json:"severity"`
	Detail   string `json:"detail"`
}

// ShieldReport es el resultado de un escaneo.
type ShieldReport struct {
	Hash      string          `json:"hash"`
	ScannedAt time.Time       `json:"scanned_at"`
	Findings  []ShieldFinding `json:"findings"`
	Skipped   []string        `json:"skipped"`
}

// Sanitized quita los controles de terminal de todo texto del informe. Las
// rutas y los detalles salen de archivos que pueden venir en un clon (o de un
// informe cacheado manipulado), así que se depuran antes de pintarlos
// (CWE-150, H-03 de la auditoría de 040).
func (r ShieldReport) Sanitized() ShieldReport {
	out := r
	out.Hash = StripTerminalControls(r.Hash)
	out.Findings = make([]ShieldFinding, len(r.Findings))
	for i, f := range r.Findings {
		f.File = StripTerminalControls(f.File)
		f.Category = StripTerminalControls(f.Category)
		f.Severity = StripTerminalControls(f.Severity)
		f.Detail = StripTerminalControls(f.Detail)
		out.Findings[i] = f
	}
	out.Skipped = make([]string, len(r.Skipped))
	for i, p := range r.Skipped {
		out.Skipped[i] = StripTerminalControls(p)
	}
	return out
}

// Summary es la línea de resumen de mem expand status: total de hallazgos por
// severidad y fecha del escaneo, sin volcar rutas ni detalles.
func (r ShieldReport) Summary() string {
	if len(r.Findings) == 0 {
		return fmt.Sprintf("sin hallazgos (%s)", r.ScannedAt.Format("2006-01-02 15:04"))
	}
	counts := map[string]int{}
	for _, f := range r.Findings {
		counts[f.Severity]++
	}
	parts := []string{}
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		if counts[sev] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[sev], sev))
			delete(counts, sev)
		}
	}
	others := 0
	for _, n := range counts {
		others += n
	}
	if others > 0 {
		parts = append(parts, fmt.Sprintf("%d otros", others))
	}
	return fmt.Sprintf("%d hallazgos (%s) · %s · detalle: mem doctor --shield",
		len(r.Findings), strings.Join(parts, ", "), r.ScannedAt.Format("2006-01-02 15:04"))
}

// shieldReferencePercent reconoce %VAR% (cmd.exe / PowerShell).
var shieldReferencePercent = regexp.MustCompile(`^%[A-Za-z_][A-Za-z0-9_]*%$`)

// shieldReference dice si un valor es una referencia a otra fuente y no una
// credencial literal: $VAR, ${VAR}, env:VAR, {env:VAR} y {file:ruta} de
// OpenCode y %VAR%. Las regiones <private> cerradas las quita literal.
func shieldReference(v string) bool {
	for _, p := range []string{"$", "env:", "{env:", "{file:"} {
		if strings.HasPrefix(v, p) {
			return true
		}
	}
	return shieldReferencePercent.MatchString(v)
}

// shieldRemote: descarga canalizada a un intérprete, también tras
// sudo/doas (con opciones), env (con asignaciones) o con ruta (R3-04). Los
// intérpretes incluyen shells, python, node, perl, php, ruby y PowerShell
// (R4-03).
var shieldRemote = regexp.MustCompile(`(?i)\b(curl|wget|irm|iwr|invoke-restmethod|invoke-webrequest)\b[^\n|]*\|\s*(?:(?:sudo|doas)(?:\s+-\S+(?:\s+[\w.-]+)?)*\s+|env(?:\s+-\S+|\s+\w+=\S*)*\s+)*(?:\S*/)?(sh|bash|zsh|dash|ksh|pwsh|powershell|iex|invoke-expression|python[0-9.]*|node|perl|php|ruby)\b`)

// shieldRemoteInline: la descarga como argumento del intérprete, sin
// tubería: `iex (irm …)`, `iex ((New-Object Net.WebClient).DownloadString(…))`,
// `bash <(curl …)` y `bash -c "$(curl …)"` (R4-03).
var shieldRemoteInline = regexp.MustCompile(`(?i)\b(?:iex|invoke-expression)\b[^\n]*\b(?:irm|iwr|invoke-restmethod|invoke-webrequest|downloadstring)\b|\b(?:sh|bash|zsh|dash|ksh|python[0-9.]*|node|perl|ruby)\b[^\n]*(?:<\(|\$\()\s*(?:curl|wget)\b`)
var shieldCredential = regexp.MustCompile(`(?i)(TOKEN|KEY|SECRET|PASSWORD|AUTHORIZATION|COOKIE|BEARER)`)

// shieldAuthHeader reconoce una cabecera de autenticación dentro de un
// argumento (`--header "Authorization: Bearer <valor>"`) y captura el valor.
var shieldAuthHeader = regexp.MustCompile(`(?i)\b(?:proxy-)?authorization\s*:\s*(?:bearer|basic|token)\s+(\S+)`)

// shieldAuthScheme es el esquema que precede a la credencial en una
// cabecera: `Bearer ${TOKEN}` sigue siendo una referencia.
var shieldAuthScheme = regexp.MustCompile(`(?i)^(?:bearer|basic|token)\s+`)

// ScanAgentConfig is pure. The reader supplies PATH and unsafe-directory evidence.
func ScanAgentConfig(path string, raw []byte, home string, pathDirs, unsafePaths []string) []ShieldFinding {
	text := redactClosedPrivateKeepLines(string(raw))
	findings := []ShieldFinding{}
	file := path
	if home != "" {
		file = strings.ReplaceAll(path, home, "~")
	}
	file = StripTerminalControls(RedactSecrets(file))
	add := func(cat, severity, detail string, line int) {
		findings = append(findings, ShieldFinding{File: file, Line: line, Category: cat, Severity: severity, Detail: detail})
	}
	for _, hit := range FindSecrets(text) {
		add("secret", "critical", "Credencial en claro ("+hit.Category+")", hit.Line)
	}
	command := func(cmd string) {
		if shieldRemote.MatchString(cmd) || shieldRemoteInline.MatchString(cmd) {
			add("remote-exec", "high", "Hook descarga y ejecuta código remoto", 0)
		}
		for _, tokens := range SplitShellCommands(cmd) {
			if len(tokens) == 0 {
				continue
			}
			for i, p := range tokens {
				if i > 0 && !strings.HasSuffix(p, ".sh") && !strings.HasSuffix(p, ".ps1") {
					continue
				}
				p = strings.Trim(p, "\"'")
				if !filepath.IsAbs(p) && !strings.HasPrefix(p, "/") {
					continue
				}
				inside := func(parent string) bool {
					rel, err := filepath.Rel(parent, p)
					return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
				}
				temp := inside("/tmp") || inside("/private/tmp") || inside("/var/tmp")
				if temp || slices.Contains(unsafePaths, p) || (!inside(home) && !slices.Contains(pathDirs, filepath.Dir(p))) {
					add("hook-outside-home", "medium", "Hook fuera del home y del PATH o en directorio inseguro", 0)
					return
				}
			}
		}
	}
	literal := func(v string) bool {
		// Una región <private> cerrada no es un literal; una apertura mal
		// formada no basta para descartar el valor (R3-03).
		v = strings.TrimSpace(redactClosedPrivateKeepLines(v))
		v = shieldAuthScheme.ReplaceAllString(v, "")
		return v != "" && !shieldReference(v)
	}
	authHeader := func(v string) bool {
		for _, m := range shieldAuthHeader.FindAllStringSubmatch(v, -1) {
			if literal(m[1]) {
				return true
			}
		}
		return false
	}
	var doc any
	if json.Unmarshal([]byte(text), &doc) == nil {
		var walk func(any, string, bool, bool)
		walk = func(node any, key string, inMCP, inHooks bool) {
			switch v := node.(type) {
			case map[string]any:
				for k, item := range v {
					mcp := inMCP || k == "mcpServers" || k == "mcp_servers" || k == "mcp"
					hooks := inHooks || k == "hooks"
					if str, ok := item.(string); ok {
						if (k == "sandbox_mode" && str == "danger-full-access") || (k == "approval_policy" && str == "never") {
							add("broad-permission", "medium", "Configuración con permisos amplios", 0)
						}
						if hooks && k == "command" {
							command(str)
						}
						if mcp && ((shieldCredential.MatchString(k) && literal(str)) || authHeader(str) || len(FindSecrets(str)) > 0) {
							add("mcp-inline-credential", "high", "Servidor MCP con credencial literal", 0)
						}
					}
					walk(item, k, mcp, hooks)
				}
			case []any:
				for _, item := range v {
					if str, ok := item.(string); ok {
						if key == "allow" && (str == "Bash" || str == "Bash(*)" || str == "Bash(:*)" || str == "*" || str == "mcp__*") {
							add("broad-permission", "medium", "Permiso de herramientas demasiado amplio", 0)
						}
						if inMCP && (authHeader(str) || len(FindSecrets(str)) > 0) {
							add("mcp-inline-credential", "high", "Servidor MCP con credencial literal", 0)
						}
					}
					walk(item, key, inMCP, inHooks)
				}
			}
		}
		walk(doc, "", false, false)
	} else if strings.HasSuffix(path, ".toml") {
		section := ""
		for i, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "[") {
				section = line
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.Trim(strings.TrimSpace(key), "\"'")
			value = frontmatterValue(strings.TrimSpace(value))
			if (key == "sandbox_mode" && value == "danger-full-access") || (key == "approval_policy" && value == "never") {
				add("broad-permission", "medium", "Configuración de Codex con permisos amplios", i+1)
			}
			if strings.Contains(section, "hooks") && key == "command" {
				command(value)
			}
			if strings.Contains(section, "mcp_servers") && ((shieldCredential.MatchString(key) && literal(value)) || authHeader(value) || len(FindSecrets(value)) > 0) {
				add("mcp-inline-credential", "high", "Servidor MCP con credencial literal", i+1)
			}
		}
	}
	return findings
}
