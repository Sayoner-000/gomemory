package domain

import (
	"path"
	"path/filepath"
	"strings"
)

// NativeEditOverScriptsRule es el nombre de la regla de serie que exige usar
// herramientas de edición nativas (edit_file / Edit) en lugar de scripts o
// comodines de consola para modificar archivos de código fuente.
const NativeEditOverScriptsRule = "native-edit-over-scripts"

// codeFileExtensions incluye las extensiones de desarrollo estándar del mercado.
var codeFileExtensions = map[string]bool{
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".vue": true, ".svelte": true, ".astro": true,
	".go": true, ".py": true, ".rs": true,
	".java": true, ".kt": true, ".kts": true, ".scala": true,
	".cs": true, ".fs": true,
	".c": true, ".cpp": true, ".cc": true, ".cxx": true, ".h": true, ".hpp": true,
	".php": true, ".rb": true, ".swift": true,
	".sh": true, ".bash": true, ".zsh": true,
	".dart": true, ".ex": true, ".exs": true, ".sql": true,
}

func isCodeFile(s string) (string, bool) {
	s = strings.Trim(s, `"'`)
	ext := strings.ToLower(filepath.Ext(s))
	if codeFileExtensions[ext] {
		return ext, true
	}
	return "", false
}

// containsFileExtension verifica si script contiene ext (ej: ".h", ".go")
// respetando el límite de extensión, evitando falsos positivos donde la extensión
// es prefijo de otra extensión común (ej: ".html" no debe coincidir con ".h",
// ".css" no debe coincidir con ".c").
func containsFileExtension(script, ext string) bool {
	start := 0
	for {
		idx := strings.Index(script[start:], ext)
		if idx == -1 {
			return false
		}
		actualIdx := start + idx
		afterIdx := actualIdx + len(ext)
		if afterIdx >= len(script) {
			return true
		}
		next := script[afterIdx]
		// Si el carácter siguiente es alfanumérico o guion bajo, es parte de un identificador o extensión más larga
		if !((next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || (next >= '0' && next <= '9') || next == '_') {
			return true
		}
		start = actualIdx + 1
	}
}

// evalNativeEditOverScripts bloquea el uso de scripts inline o comodines de terminal
// para modificar archivos de código fuente.
func evalNativeEditOverScripts(in RuleInput) (RuleAction, string) {
	for _, c := range splitShell(in.Command, 0) {
		if c.Nested {
			continue
		}
		// 1. Redirecciones destructivas hacia código: `> code.ext`, `>> code.ext`
		for _, redir := range c.Redirects {
			if ext, ok := isCodeFile(redir); ok {
				return RuleBlock, "modificar código " + ext + " con redirección de consola (" + redir + ") en lugar de las herramientas nativas; " +
					"usa edit_file o Edit para garantizar diffs auditables y activar el contexto de grafo (HCD-028). " +
					"Si es intencionado, ejecútalo en tu terminal física o desactiva la regla (mem expand off rules)"
			}
		}

		toks := unwrapCommand(stripEnvAssignments(c.Tokens))
		if len(toks) == 0 {
			continue
		}
		cmdName := path.Base(toks[0])
		args := toks[1:]

		// 2. tee hacia archivos de código
		if cmdName == "tee" {
			for _, a := range args {
				if strings.HasPrefix(a, "-") {
					continue
				}
				if ext, ok := isCodeFile(a); ok {
					return RuleBlock, "modificar código " + ext + " con tee en lugar de las herramientas nativas; " +
						"usa edit_file o Edit para garantizar diffs auditables y activar el contexto de grafo (HCD-028). " +
						"Si es intencionado, ejecútalo en tu terminal física o desactiva la regla (mem expand off rules)"
				}
			}
		}

		// 3. sed / gsed con -i hacia archivos de código
		if cmdName == "sed" || cmdName == "gsed" {
			hasInPlace := false
			for _, a := range args {
				if a == "-i" || strings.HasPrefix(a, "-i") || a == "--in-place" {
					hasInPlace = true
					break
				}
			}
			if hasInPlace {
				for _, a := range args {
					if strings.HasPrefix(a, "-") {
						continue
					}
					if ext, ok := isCodeFile(a); ok {
						return RuleBlock, "modificar código " + ext + " con sed -i en lugar de las herramientas nativas; " +
							"usa edit_file o Edit para garantizar diffs auditables y activar el contexto de grafo (HCD-028). " +
							"Si es intencionado, ejecútalo en tu terminal física o desactiva la regla (mem expand off rules)"
					}
				}
			}
		}

		// 4. perl -i / -pi hacia archivos de código
		if cmdName == "perl" {
			hasInPlace := false
			for _, a := range args {
				if strings.Contains(a, "i") && strings.HasPrefix(a, "-") {
					hasInPlace = true
					break
				}
			}
			if hasInPlace {
				for _, a := range args {
					if strings.HasPrefix(a, "-") {
						continue
					}
					if ext, ok := isCodeFile(a); ok {
						return RuleBlock, "modificar código " + ext + " con perl -i en lugar de las herramientas nativas; " +
							"usa edit_file o Edit para garantizar diffs auditables y activar el contexto de grafo (HCD-028). " +
							"Si es intencionado, ejecútalo en tu terminal física o desactiva la regla (mem expand off rules)"
					}
				}
			}
		}

		// 5. Intérpretes inline: python, python3, node, bun, deno, ruby
		switch cmdName {
		case "python", "python3", "node", "nodejs", "bun", "deno", "ruby":
			hasInline := false
			var scriptText strings.Builder
			for i, a := range args {
				if a == "-c" || a == "-e" || a == "--eval" {
					hasInline = true
					if i+1 < len(args) {
						scriptText.WriteString(args[i+1])
						scriptText.WriteString("\n")
					}
				}
				if a == "-" {
					hasInline = true
				}
			}
			for _, s := range c.Stdin {
				scriptText.WriteString(s)
				scriptText.WriteString("\n")
			}
			if hasInline || len(c.Stdin) > 0 {
				fullScript := scriptText.String()
				for ext := range codeFileExtensions {
					if containsFileExtension(fullScript, ext) {
						return RuleBlock, "modificar código " + ext + " con script inline de " + cmdName + " en lugar de las herramientas nativas; " +
							"usa edit_file o Edit para garantizar diffs auditables y activar el contexto de grafo (HCD-028). " +
							"Si es intencionado, ejecútalo en tu terminal física o desactiva la regla (mem expand off rules)"
					}
				}
			}
		}
	}

	return RuleAllow, ""
}
