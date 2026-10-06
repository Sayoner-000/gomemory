package console

import (
	"fmt"
	"strconv"
	"strings"
)

// Colores de los logos oficiales; compartidos por CLI y TUI.
const (
	Abismo  = "#061328"
	Brillo  = "#22e3f7"
	Cian    = "#06c1ee"
	Azur    = "#2c8ff0"
	Indigo  = "#3926e3"
	Violeta = "#6b56d4"
)

type Palette struct {
	Name, Background, Text, Muted, Primary, Secondary, Selection, OnAccent string
}

// Theme acepta dark, light y auto. COLORFGBG es una pista opcional;
// sin ella se usa dark. Nunca modifica el fondo de la terminal CLI.
func Theme(getenv func(string) string) Palette {
	mode := "dark"
	if getenv != nil {
		mode = strings.ToLower(strings.TrimSpace(getenv("GOMEMORY_THEME")))
		if mode != "dark" && mode != "light" && mode != "matrix" {
			parts := strings.Split(getenv("COLORFGBG"), ";")
			bg, err := strconv.Atoi(parts[len(parts)-1])
			mode = "dark"
			if err == nil && (bg == 7 || bg == 15) {
				mode = "light"
			}
		}
	}
	if mode == "matrix" {
		return Palette{"matrix", "#0a0f0a", "#62ff94", "#8ca391", "#2eff6a", "#00efff", "#1e2a1b", "#0a0f0a"}
	}
	if mode == "light" {
		return Palette{"light", "#f4f8ff", Abismo, "#53647e", Indigo, Violeta, "#e0e9fa", "#ffffff"}
	}
	return Palette{"dark", Abismo, "#e5f4ff", "#91a9c5", Brillo, Cian, "#132e4c", Abismo}
}

func foreground(hex string) string {
	n, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", n>>16, (n>>8)&255, n&255)
}
