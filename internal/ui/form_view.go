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
		focused := i == f.focus
		marker := "  "
		labelStyle := m.styles.label
		if focused {
			marker = m.styles.accent.Render("> ")
			labelStyle = m.styles.focusLabel
			focusLine = len(lines)
		}
		lines = append(lines, marker+labelStyle.Render(clip(label, max(0, m.width-2))))
		switch i {
		case fieldNote:
			for _, line := range strings.Split(f.note.View(), "\n") {
				lines = append(lines, "  "+line)
			}
		case fieldFavorite, fieldSavePassword:
			value := "[ ]"
			if i == fieldFavorite && f.favorite {
				value = "[x]"
			}
			if i == fieldSavePassword && f.savePassword {
				value = "[x]"
			}
			lines = append(lines, "  "+m.styles.accent.Render(value))
		default:
			lines = append(lines, "  "+f.inputs[i].View())
		}
		if err := f.errors[i]; err != "" {
			lines = append(lines, m.styles.failure.Render(clip("  "+err, m.width)))
		}
	}
	rows := max(1, m.height-5)
	start := max(0, min(focusLine-1, len(lines)-rows))
	result := []string{m.formHeader(title), m.rule()}
	for i := range rows {
		if start+i < len(lines) {
			result = append(result, lines[start+i])
		} else {
			result = append(result, "")
		}
	}
	result = append(result, m.rule())
	message := f.saveError
	messageStyle := m.styles.muted
	if message != "" {
		messageStyle = m.styles.failure
	} else {
		message = fmt.Sprintf("Field %d/%d · tab/↑/↓ navigate", f.focus+1, fieldCount)
	}
	result = append(result,
		messageStyle.Render(clip(message, m.width)),
		m.styles.muted.Render(clip("ctrl+s save · esc cancel · ctrl+c quit", m.width)))
	return strings.Join(result, "\n")
}

func (m *Model) resizeForm() {
	for i := range m.form.inputs {
		m.form.inputs[i].Width = max(1, m.width-4)
	}
	if len(m.form.inputs) > 0 {
		m.form.note.SetWidth(max(1, m.width-4))
	}
}

func (m Model) confirmationView(base string) string {
	lines := strings.Split(base, "\n")
	return strings.Join(overlayLines(lines, m.confirmPanel(), m.width, m.height), "\n")
}
