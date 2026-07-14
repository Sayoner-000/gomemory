package cli

import (
	"fmt"
	"strings"

	"mem/domain"
)

// memListExtractChars es el largo del extracto por resultado en las tools de
// consulta (search_memories/list_memories). Referencia engram: ~100 tokens por
// resultado. El detalle íntegro queda en get_memory (renderMemoryDetail).
const memListExtractChars = 160

// renderSearchResults formatea resultados de búsqueda en modo compacto
// (progressive disclosure, capa 1): id + tipo + título + extracto acotado.
func renderSearchResults(mems []domain.Memory) string {
	var sb strings.Builder
	for _, m := range mems {
		sb.WriteString(fmt.Sprintf("[%d] %s | %s\n  %s\n\n", m.ID, m.Type, m.Title, domain.Extract(m.Content, memListExtractChars)))
	}
	return sb.String()
}

// renderMemoryList formatea el listado reciente con el mismo extracto compacto
// que la búsqueda (helper unificado).
func renderMemoryList(mems []domain.Memory) string {
	var sb strings.Builder
	for _, m := range mems {
		sb.WriteString(fmt.Sprintf("[%d] %s | %s\n  %s\n\n", m.ID, m.Type, m.Title, domain.Extract(m.Content, memListExtractChars)))
	}
	return sb.String()
}

// renderMemoryDetail formatea el detalle íntegro de una memoria (progressive
// disclosure, capa 3): devuelve el contenido completo SIN truncar.
func renderMemoryDetail(m domain.Memory) string {
	sessionInfo := ""
	if m.SessionID != "" {
		sessionInfo = fmt.Sprintf("\nSesión: %s", m.SessionID[:min(len(m.SessionID), 8)])
	}
	fileInfo := ""
	if m.Filepath != "" {
		fileInfo = fmt.Sprintf("\nArchivo: %s", m.Filepath)
	}
	promptInfo := ""
	if m.OriginPrompt != "" {
		promptInfo = fmt.Sprintf("\nPrompt originante: %s", m.OriginPrompt)
	}
	return fmt.Sprintf("ID: %d\nTipo: %s\nTítulo: %s\nFecha: %s%s%s%s\n\n%s",
		m.ID, m.Type, m.Title, m.CreatedAt, sessionInfo, fileInfo, promptInfo, m.Content)
}
