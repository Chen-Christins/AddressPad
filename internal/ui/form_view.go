package ui

import (
	"fmt"
	"strings"
)

func (m Model) formView() string {
	f := m.form
	title := "Add server"
	if f.editing {
		title = "Edit server: " + f.original.Name
	}
	lines := []string{}
	focusLine := 0
	for i, label := range fieldLabels {
		marker := "  "
		if i == f.focus {
			marker = "> "
			focusLine = len(lines)
		}
		lines = append(lines, clip(marker+label, m.width))
		switch i {
		case fieldNote:
			for _, line := range strings.Split(f.note.View(), "\n") {
				lines = append(lines, line)
			}
		case fieldFavorite:
			value := "[ ]"
			if f.favorite {
				value = "[x]"
			}
			lines = append(lines, "  "+value)
		default:
			lines = append(lines, "  "+f.inputs[i].View())
		}
		if err := f.errors[i]; err != "" {
			lines = append(lines, m.styles.failure.Render(clip("  "+err, m.width)))
		}
	}
	rows := max(1, m.height-4)
	start := max(0, min(focusLine-1, len(lines)-rows))
	result := []string{m.styles.title.Render(clip(title, m.width))}
	for i := range rows {
		if start+i < len(lines) {
			result = append(result, lines[start+i])
		} else {
			result = append(result, "")
		}
	}
	message := f.saveError
	if message == "" {
		message = fmt.Sprintf("Field %d/%d · tab/↑/↓ navigate", f.focus+1, fieldCount)
	}
	result = append(result, clip(message, m.width), clip("ctrl+s save · esc cancel · ctrl+c quit", m.width), "")
	return strings.Join(result, "\n")
}

func (m *Model) resizeForm() {
	for i := range m.form.inputs {
		m.form.inputs[i].Width = max(1, m.width-4)
	}
	if len(m.form.inputs) > 0 {
		m.form.note.SetWidth(max(1, m.width-2))
	}
}

func (m Model) confirmationView(base string) string {
	text := []string{"Discard unsaved changes?", "y: discard · n/esc: keep editing"}
	if m.mode == modeConfirmDelete {
		address := m.deleteTarget.Host
		if m.deleteTarget.User != "" {
			address = m.deleteTarget.User + "@" + address
		}
		text = []string{"Delete server?", m.deleteTarget.Name, address, "y/enter: delete · n/esc: cancel", m.status}
	}
	lines := strings.Split(base, "\n")
	height := min(len(text)+2, m.height)
	width := min(64, m.width)
	start := max(0, (m.height-height)/2)
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	for i := range height {
		line := "+" + strings.Repeat("-", max(0, width-2)) + "+"
		if i > 0 && i < height-1 {
			line = "|" + column(text[i-1], width-2) + "|"
		}
		lines[start+i] = strings.Repeat(" ", max(0, (m.width-width)/2)) + line
	}
	return strings.Join(lines, "\n")
}
