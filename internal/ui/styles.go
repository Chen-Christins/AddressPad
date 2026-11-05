package ui

import (
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

type styles struct {
	title, selected, muted, text, name, accent, favorite, group, tag lipgloss.Style
	success, failure, rule, header, label, focusLabel, border        lipgloss.Style
}

func newStyles() styles {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return styles{}
	}
	accent := lipgloss.AdaptiveColor{Light: "30", Dark: "80"}
	text := lipgloss.AdaptiveColor{Light: "235", Dark: "252"}
	return styles{
		title:    lipgloss.NewStyle().Bold(true).Foreground(accent),
		selected: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "18", Dark: "230"}).Background(lipgloss.AdaptiveColor{Light: "153", Dark: "24"}),
		muted:    lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "240", Dark: "245"}),
		text:     lipgloss.NewStyle().Foreground(text),
		name:     lipgloss.NewStyle().Bold(true).Foreground(text),
		accent:   lipgloss.NewStyle().Foreground(accent),
		favorite: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "136", Dark: "220"}),
		group:    lipgloss.NewStyle().Foreground(accent),
		tag:      lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "90", Dark: "177"}),
		success:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "114"}),
		failure:  lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "160", Dark: "203"}),
		rule:     lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "238"}),
		header:   lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "242", Dark: "245"}),
		label:    lipgloss.NewStyle().Foreground(text),
		focusLabel: lipgloss.NewStyle().Bold(true).
			Foreground(accent),
		border: lipgloss.NewStyle().Foreground(accent),
	}
}

// Configuration data is plain text, never terminal control sequences.
func plain(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

func clip(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = plain(s)
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > width-1 {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String() + "…"
}

func column(s string, width int) string {
	s = clip(s, width)
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}
