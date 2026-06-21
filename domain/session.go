package domain

import (
	"crypto/rand"
	"fmt"
)

func NewSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type Session struct {
	ID        string  `json:"id"`
	Project   string  `json:"project"`
	Summary   string  `json:"summary"`
	CreatedAt string  `json:"created_at"`
	EndedAt   *string `json:"ended_at,omitempty"`
}
