package cli

import (
	"bytes"
	"encoding/json"
	"os"
)

// transcriptTailBytes acota cuánto del archivo de transcript se lee: el
// checkpoint solo necesita el turno más reciente, no la sesión completa.
const transcriptTailBytes = 300 * 1024

// turnActivity resume qué tocó un turno: archivos editados/escritos y
// comandos ejecutados. Lo produce tanto el parser de transcript de Claude
// Code como el payload que manda el plugin de OpenCode.
type turnActivity struct {
	Files    []string
	Commands []string
}

func (a turnActivity) empty() bool {
	return len(a.Files) == 0 && len(a.Commands) == 0
}

// transcriptEntry es el subconjunto de campos que nos interesan de una línea
// del transcript JSONL de Claude Code (formato Anthropic Messages API
// anidado: message.content es un string para mensajes humanos planos, o un
// array de bloques text/tool_use para mensajes del asistente).
type transcriptEntry struct {
	Type    string `json:"type"`
	Message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// extractLastTurnActivity lee la cola del transcript JSONL de Claude Code y
// devuelve los archivos editados/escritos y comandos ejecutados desde el
// último mensaje humano real (message.content como string plano, no un
// tool_result) hasta el final del archivo. Best-effort: si ese límite no
// aparece dentro de la cola leída, usa toda la cola disponible en vez de
// fallar — un checkpoint que abarque de más es preferible a uno que no se
// genere.
func extractLastTurnActivity(transcriptPath string) turnActivity {
	var activity turnActivity

	data, err := readTail(transcriptPath, transcriptTailBytes)
	if err != nil || len(data) == 0 {
		return activity
	}

	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))

	turnStart := 0
	for i := len(lines) - 1; i >= 0; i-- {
		var entry transcriptEntry
		if err := json.Unmarshal(lines[i], &entry); err != nil {
			continue
		}
		if entry.Type != "user" || entry.Message.Role != "user" {
			continue
		}
		if isPlainJSONString(entry.Message.Content) {
			turnStart = i
			break
		}
	}

	seenFiles := make(map[string]bool)
	for _, line := range lines[turnStart:] {
		var entry transcriptEntry
		if err := json.Unmarshal(line, &entry); err != nil || entry.Type != "assistant" {
			continue
		}
		var blocks []contentBlock
		if err := json.Unmarshal(entry.Message.Content, &blocks); err != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type != "tool_use" {
				continue
			}
			switch b.Name {
			case "Edit", "Write", "MultiEdit", "NotebookEdit":
				var in struct {
					FilePath string `json:"file_path"`
				}
				if json.Unmarshal(b.Input, &in) == nil && in.FilePath != "" && !seenFiles[in.FilePath] {
					seenFiles[in.FilePath] = true
					activity.Files = append(activity.Files, in.FilePath)
				}
			case "Bash":
				var in struct {
					Command string `json:"command"`
				}
				if json.Unmarshal(b.Input, &in) == nil && in.Command != "" {
					activity.Commands = append(activity.Commands, in.Command)
				}
			}
		}
	}

	return activity
}

func isPlainJSONString(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '"'
}

// readTail devuelve los últimos maxBytes de un archivo (o el archivo entero
// si es más chico), recortando la primera línea parcial para no romper el
// parseo JSON de la línea siguiente.
func readTail(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := info.Size()
	offset := int64(0)
	if size > maxBytes {
		offset = size - maxBytes
	}
	if _, err := f.Seek(offset, 0); err != nil {
		return nil, err
	}

	data := make([]byte, size-offset)
	if _, err := f.Read(data); err != nil && len(data) == 0 {
		return nil, err
	}

	if offset > 0 {
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
			data = data[idx+1:]
		}
	}
	return data, nil
}
