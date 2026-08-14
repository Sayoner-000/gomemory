package compression

import (
	"regexp"
	"strconv"
	"strings"

	"mem/application/ports"
)

// charsPerToken es la misma heurística de orden de magnitud que
// adapters/secondary/tokens.ApproximateTokenCounter (~4 caracteres por
// token). Se duplica aquí, en vez de importar el paquete tokens, para que
// este adaptador no dependa de otro adaptador — BuildContextPack usa su
// propio ports.TokenCounter inyectado para la contabilidad autoritativa del
// ContextPack; este valor es solo la estimación interna que decide
// Compressed (feature 015, research.md §4/§5).
const charsPerToken = 4

func approxTokens(s string) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	t := (n + charsPerToken - 1) / charsPerToken
	if t < 1 {
		t = 1
	}
	return t
}

// fencedCodeBlock reconoce bloques ```...``` (incluida la variante con
// lenguaje tipo ```go) para excluirlos por completo de la compresión.
var fencedCodeBlock = regexp.MustCompile("(?s)```.*?```")

// blankLines reconoce dos o más saltos de línea consecutivos (con espacios
// en blanco entremedio) como separador de párrafo.
var blankLines = regexp.MustCompile(`\n[ \t]*\n[ \t\n]*`)

// repeatedInlineSpace colapsa espacios/tabs repetidos DENTRO de una línea.
// Nunca toca saltos de línea, así que nunca reordena ni fusiona líneas
// distintas de una lista o de un bloque con formato propio.
var repeatedInlineSpace = regexp.MustCompile(`[ \t]{2,}`)

// StructuralCompressor implementa ports.Compressor de forma determinista y
// sin dependencia de un LLM (FR-009): colapsa whitespace repetido y párrafos
// duplicados idénticos. Por construcción nunca toca un carácter que no sea
// whitespace, así que preserva siempre código, URLs, rutas, identificadores,
// mensajes de error, números y versiones — los flags Preserve* de
// CompressionOptions documentan esa garantía, no activan una rama distinta
// del algoritmo.
type StructuralCompressor struct{}

func (StructuralCompressor) Compress(input string, opts ports.CompressionOptions) (ports.CompressionResult, error) {
	rawTokens := approxTokens(input)

	if opts.Level == ports.CompressionNone {
		return ports.CompressionResult{Content: input, RawTokens: rawTokens, Tokens: rawTokens}, nil
	}

	output := compressStructural(input)
	tokens := approxTokens(output)
	return ports.CompressionResult{
		Content:    output,
		RawTokens:  rawTokens,
		Tokens:     tokens,
		Compressed: output != input,
	}, nil
}

// compressStructural extrae los bloques de código para dejarlos intactos,
// colapsa espacios repetidos y párrafos duplicados en el resto, y restaura
// los bloques de código en su posición original.
func compressStructural(input string) string {
	var codeBlocks []string
	placeholder := func(i int) string { return "\x00CODEBLOCK" + strconv.Itoa(i) + "\x00" }

	withoutCode := fencedCodeBlock.ReplaceAllStringFunc(input, func(block string) string {
		codeBlocks = append(codeBlocks, block)
		return placeholder(len(codeBlocks) - 1)
	})

	paragraphs := blankLines.Split(withoutCode, -1)
	seen := make(map[string]struct{}, len(paragraphs))
	kept := make([]string, 0, len(paragraphs))
	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		collapsed := repeatedInlineSpace.ReplaceAllString(trimmed, " ")
		if _, dup := seen[collapsed]; dup {
			continue
		}
		seen[collapsed] = struct{}{}
		kept = append(kept, collapsed)
	}
	result := strings.Join(kept, "\n\n")

	for i, block := range codeBlocks {
		result = strings.Replace(result, placeholder(i), block, 1)
	}
	return result
}
