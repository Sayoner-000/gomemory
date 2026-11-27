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

// textFileExtensions son los archivos de texto del proyecto que también se
// editan con las herramientas nativas (HCD-066, Q1): documentación,
// plantillas, skills y configuración.
var textFileExtensions = map[string]bool{
	".md": true, ".tmpl": true, ".html": true, ".css": true, ".json": true,
	".yaml": true, ".yml": true, ".toml": true, ".txt": true, ".xml": true,
}

func isCodeFile(s string) (string, bool) {
	s = strings.Trim(s, `"'`)
	ext := strings.ToLower(filepath.Ext(s))
	if codeFileExtensions[ext] {
		return ext, true
	}
	return "", false
}

// editReason es el motivo de bloqueo común. La salida de escape («si es
// intencionado…») la añade ruleReason a toda regla que bloquea.
func editReason(ext, via string) string {
	what := "el archivo " + ext
	if codeFileExtensions[ext] {
		what = "código " + ext
	}
	return "modificar " + what + " con " + via + " en lugar de las herramientas nativas; " +
		"usa edit_file o Edit para garantizar diffs auditables y activar el contexto de grafo (HCD-028)"
}

// editedFile dice si s es un archivo cuya modificación exige la herramienta
// nativa. El código, siempre. Un archivo de texto, siempre en la edición in
// situ (sed -i, perl -i); en una redirección o un tee solo si la ruta es del
// proyecto, porque escribir una salida en /tmp o en el home es legítimo (Q3).
func editedFile(s string, inPlace bool) (string, bool) {
	if ext, ok := isCodeFile(s); ok {
		return ext, true
	}
	s = strings.Trim(s, `"'`)
	ext := strings.ToLower(filepath.Ext(s))
	if !textFileExtensions[ext] || (!inPlace && externalPath(s)) {
		return "", false
	}
	return ext, true
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
			if ext, ok := editedFile(redir, false); ok {
				return RuleBlock, editReason(ext, "redirección de consola ("+redir+")")
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
				if ext, ok := editedFile(a, false); ok {
					return RuleBlock, editReason(ext, "tee")
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
					if ext, ok := editedFile(a, true); ok {
						return RuleBlock, editReason(ext, "sed -i")
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
					if ext, ok := editedFile(a, true); ok {
						return RuleBlock, editReason(ext, "perl -i")
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
						return RuleBlock, editReason(ext, "script inline de "+cmdName)
					}
				}
			}
		}
	}

	return RuleAllow, ""
}
