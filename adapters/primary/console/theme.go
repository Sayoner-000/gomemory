package console

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"mem/assets"
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
	Warning, Error                                                         string
	Logo                                                                   [4]string
}

func (p Palette) WarningColor() string {
	return p.Warning
}

func (p Palette) ErrorColor() string {
	return p.Error
}

var palettes = func() map[string]Palette {
	var p map[string]Palette
	if err := json.Unmarshal(assets.ConsoleThemes, &p); err != nil {
		panic("paleta embebida inválida: " + err.Error())
	}
	return p
}()

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
	return palettes[mode]
}

func foreground(hex string) string {
	n, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", n>>16, (n>>8)&255, n&255)
}
