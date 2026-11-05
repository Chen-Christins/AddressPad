package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// panelRow renders styled segments clipped to width and padded to fill it.
func panelRow(parts []rowPart, width int) string {
	content := fitParts(parts, width)
	if pad := width - lipgloss.Width(content); pad > 0 {
		content += strings.Repeat(" ", pad)
	}
	return content
}

// panelLines builds a rounded panel that never exceeds maxHeight rows.
func panelLines(width, maxHeight int, title string, body [][]rowPart, hint string, st styles) []string {
	width = max(16, width)
	if maxHeight < 3 {
		return []string{clip(strings.Repeat("─", width), width)}
	}
	inner := max(1, width-4)
	top := st.border.Render("╭" + strings.Repeat("─", width-2) + "╮")
	bottom := st.border.Render("╰" + strings.Repeat("─", width-2) + "╯")
	titleRow := st.border.Render("│ ") +
		st.title.Render(column(clip(title, inner), inner)) +
		st.border.Render(" │")
	lines := []string{top, titleRow}

	hint = clip(hint, inner)
	budget := maxHeight - 3 // top, title and bottom are fixed
	if budget > 0 && len(body) > 0 {
		reserveHint := 0
		if hint != "" && budget >= 3 {
			reserveHint = 1
		}
		bodyLimit := min(len(body), budget-1-reserveHint)
		if bodyLimit > 0 {
			lines = append(lines, st.border.Render("├"+strings.Repeat("─", width-2)+"┤"))
			for _, row := range body[:bodyLimit] {
				lines = append(lines, st.border.Render("│ ")+panelRow(row, inner)+st.border.Render(" │"))
			}
			if reserveHint == 1 {
				lines = append(lines, st.border.Render("│ ")+st.muted.Render(column(hint, inner))+st.border.Render(" │"))
			}
		} else if hint != "" && budget >= 1 {
			lines = append(lines, st.border.Render("│ ")+st.muted.Render(column(hint, inner))+st.border.Render(" │"))
		}
	} else if hint != "" && budget >= 1 {
		lines = append(lines, st.border.Render("│ ")+st.muted.Render(column(hint, inner))+st.border.Render(" │"))
	}
	lines = append(lines, bottom)
	return lines
}

// overlayLines replaces the rows covered by panel, centering it on screen.
func overlayLines(base, panel []string, width, height int) []string {
	lines := make([]string, height)
	copy(lines, base)
	if len(panel) == 0 {
		return lines
	}
	panelWidth := 0
	for _, line := range panel {
		panelWidth = max(panelWidth, lipgloss.Width(line))
	}
	left := max(0, (width-panelWidth)/2)
	top := max(0, (height-len(panel))/2)
	for i, line := range panel {
		if top+i >= height {
			break
		}
		content := line
		if w := lipgloss.Width(content); w > width {
			content = clip(content, width)
		}
		lines[top+i] = strings.Repeat(" ", left) + content +
			strings.Repeat(" ", max(0, width-left-lipgloss.Width(content)))
	}
	return lines
}

func helpBody(st styles, inner int) [][]rowPart {
	keyWidth := min(9, max(5, inner/2))
	items := [][2]string{
		{"k / up", "Move up"},
		{"j / down", "Move down"},
		{"g / G", "First / last server"},
		{"/", "Search (subsequence, AND words)"},
		{"enter", "Connect using system SSH"},
		{"a/e/d", "Add / edit / delete"},
		{"f", "Toggle favorite and save"},
		{"c", "Copy SSH command"},
		{"i", "Import ~/.ssh/config"},
		{"esc", "Back; clear if no match"},
		{"q / ^c", "Quit"},
	}
	body := make([][]rowPart, 0, len(items))
	for _, item := range items {
		body = append(body, []rowPart{
			{column(item[0], keyWidth), st.accent},
			{item[1], st.text},
		})
	}
	return body
}

func (m Model) helpPanel() []string {
	width := min(64, max(20, m.width-2))
	body := helpBody(m.styles, max(1, width-4))
	return panelLines(width, m.height, "Help", body, "? / esc / q closes", m.styles)
}

func (m Model) confirmPanel() []string {
	width := min(58, max(20, m.width-2))
	title := "Delete server?"
	hint := "y/enter: delete · n/esc: cancel"
	body := [][]rowPart{
		{{m.deleteTarget.Name, m.styles.name}},
		{{serverAddress(m.deleteTarget), m.styles.muted}},
	}
	if m.status != "" {
		body = append(body, []rowPart{{m.status, m.styles.failure}})
	}
	if m.mode == modeConfirmDiscard {
		title = "Discard unsaved changes?"
		hint = "y: discard · n/esc: keep editing"
		body = [][]rowPart{{{"Your edits have not been saved.", m.styles.muted}}}
	}
	return panelLines(width, m.height, title, body, hint, m.styles)
}

func (m Model) errorView() string {
	width := min(72, max(20, m.width-2))
	inner := max(1, width-4)
	body := [][]rowPart{
		{{"Path: ", m.styles.muted}, {m.path, m.styles.text}},
	}
	for _, line := range wrapText(m.loadError.Error(), inner) {
		body = append(body, []rowPart{{line, m.styles.failure}})
	}
	hint := "Fix the file and restart. q / ctrl+c to quit."
	panel := panelLines(width, m.height, "Configuration error (read-only)", body, hint, m.styles)
	return strings.Join(overlayLines(make([]string, m.height), panel, m.width, m.height), "\n")
}

func wrapText(text string, width int) []string {
	width = max(1, width)
	text = plain(text)
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		current := ""
		for _, word := range words {
			for lipgloss.Width(word) > width {
				head, rest := splitWidth(word, width)
				if current != "" {
					lines = append(lines, current)
					current = ""
				}
				lines = append(lines, head)
				word = rest
			}
			if word == "" {
				continue
			}
			switch {
			case current == "":
				current = word
			case lipgloss.Width(current)+1+lipgloss.Width(word) <= width:
				current += " " + word
			default:
				lines = append(lines, current)
				current = word
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "")
	}
	return lines
}

func splitWidth(s string, width int) (string, string) {
	used := 0
	for i, r := range s {
		runeWidth := lipgloss.Width(string(r))
		if used+runeWidth > width {
			if i == 0 {
				end := len(string(r))
				return s[:end], s[end:]
			}
			return s[:i], s[i:]
		}
		used += runeWidth
	}
	return s, ""
}

func (m Model) formHeader(title string) string {
	position := m.styles.muted.Render(fmt.Sprintf("%d/%d", m.form.focus+1, fieldCount))
	available := m.width - lipgloss.Width(position) - 1
	if available < 1 {
		return m.styles.title.Render(clip(title, m.width))
	}
	left := m.styles.title.Render(clip(title, available))
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(position)
	if gap < 1 {
		return m.styles.title.Render(clip(title, m.width))
	}
	return left + strings.Repeat(" ", gap) + position
}
