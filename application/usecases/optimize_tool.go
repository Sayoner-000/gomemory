package usecases

import "mem/application/ports"

// ToolDescriptor es la porción de una definición de tool MCP-like que
// OptimizeToolDescription puede acortar (feature 015, Historia 5).
type ToolDescriptor struct {
	Name        string
	Description string
	// Schema es opaco: nunca se parsea ni se muta, solo se copia tal cual.
	Schema []byte
}

// OptimizeToolDescription acorta Description con el mismo Compressor
// determinista que usa BuildContextPack (CompressionStructural) — sin
// lógica específica de MCP aquí. Name y Schema salen siempre idénticos
// byte-por-byte al input (FR-017): la superficie funcional de la tool nunca
// cambia, solo su texto descriptivo.
func OptimizeToolDescription(t ToolDescriptor, compressor ports.Compressor) (ToolDescriptor, error) {
	if t.Description == "" {
		return t, nil
	}

	res, err := compressor.Compress(t.Description, ports.CompressionOptions{Level: ports.CompressionStructural})
	if err != nil {
		// FR-011: un fallo de compresión no debe romper el registro de la
		// tool — se sigue con la descripción original.
		return t, nil
	}

	return ToolDescriptor{
		Name:        t.Name,
		Description: res.Content,
		Schema:      t.Schema,
	}, nil
}
