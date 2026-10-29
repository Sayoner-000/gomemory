package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ContentRevision identifica contenido Markdown normalizado con los primeros
// 12 caracteres hexadecimales de su SHA-256.
func ContentRevision(content []byte) string {
	normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t\r")
	}
	normalized = strings.Join(lines, "\n")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])[:12]
}
