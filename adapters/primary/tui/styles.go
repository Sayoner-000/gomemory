package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"mem/domain"
)

// ─── Styles ───────────────────────────────────────────────────────

// Paleta "Matrix" (tema oscuro fijo, sin AdaptiveColor) tomada tal cual de
// sst/opencode (matrix.json, modo dark) — no es una interpretación propia,
// son sus valores hex exactos mapeados por rol semántico:
//
//	background→matrixInk0  border→matrixInk3  borderActive/primary→rainGreen
//	textMuted→rainGray     text→rainGreenHi    secondary→rainCyan
//	accent→rainPurple      error→alertRed      warning→alertYellow
//	info→alertBlue         (Decisión usa rainGreenDim, el verde "asentado")
var (
	faint     color.Color = lipgloss.Color("#8ca391") // rainGray / textMuted
	highlight color.Color = lipgloss.Color("#2eff6a") // rainGreen / primary, borderActive — Arquitectura
	green     color.Color = lipgloss.Color("#1cc24b") // rainGreenDim — Decisión
	red       color.Color = lipgloss.Color("#ff4b4b") // alertRed / error — Bugfix
	blue      color.Color = lipgloss.Color("#30b3ff") // alertBlue / info — Patrón
	yellow    color.Color = lipgloss.Color("#e6ff57") // alertYellow / warning — Aprendizaje
	cyan      color.Color = lipgloss.Color("#00efff") // rainCyan / secondary — Hallazgo
	pink      color.Color = lipgloss.Color("#c770ff") // rainPurple / accent — Preferencia
	gray      color.Color = lipgloss.Color("#1e2a1b") // matrixInk3 / border — fondo de selección
	white     color.Color = lipgloss.Color("#62ff94") // rainGreenHi / text — texto sobre selección
	bg        color.Color = lipgloss.Color("#0a0e0a") // matrixInk0 / background
)

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

	groupHeaderStyle = lipgloss.NewStyle().
				Foreground(faint).
				Padding(0, 1).
				MarginTop(1).
				MarginBottom(1)

	typeTag = func(t string) string {
		return lipgloss.NewStyle().
			Background(typeColor(t)).
			Foreground(white).
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

	itemContent = lipgloss.NewStyle().
			Foreground(faint).
			Padding(0, 2).
			MaxWidth(80)

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
