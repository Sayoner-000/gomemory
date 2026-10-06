package tui

import (
	"image/color"
	"mem/adapters/primary/console"
	"os"

	"charm.land/lipgloss/v2"

	"mem/domain"
)

// ─── Styles ───────────────────────────────────────────────────────

// Identidad goMemory: oscuro y claro, con roles accesibles por tema.
var (
	faint      color.Color = lipgloss.Color("#91a9c5")
	highlight  color.Color = lipgloss.Color(console.Brillo)
	green      color.Color = lipgloss.Color(console.Cian)
	red        color.Color = lipgloss.Color("#ff777f")
	blue       color.Color = lipgloss.Color(console.Azur)
	yellow     color.Color = lipgloss.Color("#f4c66b")
	cyan       color.Color = lipgloss.Color(console.Cian)
	pink       color.Color = lipgloss.Color(console.Violeta)
	gray       color.Color = lipgloss.Color("#132e4c")
	white      color.Color = lipgloss.Color("#e5f4ff")
	accentText color.Color = lipgloss.Color(console.Abismo)
)

var currentPalette = console.Theme(os.Getenv)

func themePalette(name string) console.Palette {
	return console.Theme(func(key string) string {
		if key == "GOMEMORY_THEME" && name != "" {
			return name
		}
		return os.Getenv(key)
	})
}

func applyTheme(names ...string) {
	name := ""
	if len(names) > 0 {
		name = names[0]
	}
	p := themePalette(name)
	currentPalette = p
	faint, highlight = lipgloss.Color(p.Muted), lipgloss.Color(p.Primary)
	green, cyan = lipgloss.Color(p.Secondary), lipgloss.Color(p.Secondary)
	blue, pink = lipgloss.Color(console.Azur), lipgloss.Color(console.Violeta)
	gray, white, accentText = lipgloss.Color(p.Selection), lipgloss.Color(p.Text), lipgloss.Color(p.OnAccent)
	red, yellow = lipgloss.Color(p.ErrorColor()), lipgloss.Color(p.WarningColor())
	if p.Name == "matrix" {
		green = lipgloss.Color("#1cc24b")
		blue, pink = lipgloss.Color("#30b3ff"), lipgloss.Color("#c770ff")
	}
	appStyle = appStyle.Foreground(white).Background(lipgloss.Color(p.Background))
	titleStyle = titleStyle.Foreground(highlight)
	subtitleStyle = subtitleStyle.Foreground(faint)
	itemSelected = itemSelected.Background(gray).Foreground(white)
	detailBorder = detailBorder.BorderForeground(highlight)
	listBorder = listBorder.BorderForeground(highlight)
	helpStyle = helpStyle.Foreground(faint)
	formLabel = formLabel.Foreground(highlight)
	errorStyle = errorStyle.Foreground(red)
	dangerStyle = dangerStyle.Foreground(red)
	backHintStyle = backHintStyle.Foreground(faint)
	statusLineStyle = statusLineStyle.Foreground(faint)
}

func typeColor(t string) color.Color {
	switch t {
	case string(domain.Architecture):
		return highlight
	case string(domain.Decision):
		return green
	case string(domain.Bugfix):
		return red
	case string(domain.Pattern):
		return blue
	case string(domain.Learning):
		return yellow
	case string(domain.Discovery):
		return cyan
	case string(domain.Preference):
		return pink
	default:
		return faint
	}
}

func typeIcon(t string) string {
	switch t {
	case string(domain.Architecture):
		return "▲"
	case string(domain.Decision):
		return "◆"
	case string(domain.Bugfix):
		return "✕"
	case string(domain.Pattern):
		return "■"
	case string(domain.Learning):
		return "●"
	case string(domain.Discovery):
		return "◇"
	case string(domain.Preference):
		return "♥"
	default:
		return "●"
	}
}

func typeLabel(t string) string {
	switch t {
	case string(domain.Architecture):
		return "Arquitectura"
	case string(domain.Decision):
		return "Decisión"
	case string(domain.Bugfix):
		return "Bugfix"
	case string(domain.Pattern):
		return "Patrón"
	case string(domain.Learning):
		return "Aprendizaje"
	case string(domain.Discovery):
		return "Hallazgo"
	case string(domain.Preference):
		return "Preferencia"
	default:
		return t
	}
}

var (
	appStyle = lipgloss.NewStyle().
			Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(faint).
			Italic(true)

	typeTag = func(t string) string {
		ink := lipgloss.Color("#ffffff")
		if t == string(domain.Architecture) || t == string(domain.Discovery) || t == string(domain.Decision) || (currentPalette.Name != "light" && (t == string(domain.Learning) || t == string(domain.Bugfix) || t == string(domain.Pattern))) {
			ink = accentText
		}
		return lipgloss.NewStyle().
			Background(typeColor(t)).
			Foreground(ink).
			Padding(0, 1).
			Bold(true).
			Render(typeIcon(t) + " " + typeLabel(t))
	}

	itemNormal = lipgloss.NewStyle().
			Padding(0, 2)

	itemSelected = lipgloss.NewStyle().
			Padding(0, 2).
			Background(gray).
			Foreground(white)

	detailBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(highlight).
			Padding(1, 2)

	listBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(highlight).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(faint).
			PaddingTop(1).
			BorderTop(true).
			BorderStyle(lipgloss.NormalBorder())

	formStyle = lipgloss.NewStyle().
			MarginTop(1)

	formLabel = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			MarginRight(1)

	formInput = lipgloss.NewStyle().
			MarginBottom(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	dangerStyle = lipgloss.NewStyle().
			Foreground(red).
			Bold(true)

	backHintStyle = lipgloss.NewStyle().
			Foreground(faint)

	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true)

	statusLineStyle = lipgloss.NewStyle().
			Foreground(faint).
			Italic(true)
)
