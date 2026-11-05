package ui

import (
	"fmt"
	"strings"
	"time"

	"addresspad/internal/model"
)

const lastUsedWidth = 8

// relativeTime renders an age compactly; zero means the server was never used.
func relativeTime(t, now time.Time) string {
	if t.IsZero() {
		return "never"
	}
	age := now.Sub(t)
	if age < time.Second {
		return "now"
	}
	if seconds := int(age / time.Second); seconds < 60 {
		return fmt.Sprintf("%ds ago", seconds)
	}
	if minutes := int(age / time.Minute); minutes < 60 {
		return fmt.Sprintf("%dm ago", minutes)
	}
	if hours := int(age / time.Hour); hours < 24 {
		return fmt.Sprintf("%dh ago", hours)
	}
	days := int(age / (24 * time.Hour))
	if days < 7 {
		return fmt.Sprintf("%dd ago", days)
	}
	if days < 30 {
		return fmt.Sprintf("%dw ago", days/7)
	}
	if days < 365 {
		return fmt.Sprintf("%dmo ago", days/30)
	}
	return fmt.Sprintf("%dy ago", days/365)
}

func serverRow(s model.Server, width int, now time.Time) string {
	star := " "
	if s.Favorite {
		star = "★"
	}
	address := s.Host
	if strings.Contains(address, ":") {
		address = "[" + address + "]"
	}
	if s.User != "" {
		address = s.User + "@" + address
	}
	address = fmt.Sprintf("%s:%d", address, s.Port)
	// serverRow receives width-2 for the selection prefix; 58 keeps the
	// aligned columns visible at a 60-column terminal.
	if width < 58 {
		return clip(star+" "+s.Name+"  "+address, width)
	}
	nameWidth := min(28, width/4)
	addressWidth := min(44, width/3)
	row := star + " " + column(s.Name, nameWidth) + " " + column(address, addressWidth)
	row += " " + rightColumn(relativeTime(s.LastUsed, now), lastUsedWidth)
	if s.Group != "" {
		row += " [" + plain(s.Group) + "]"
	}
	for _, tag := range s.Tags {
		row += " #" + plain(tag)
	}
	if s.Note != "" {
		row += "  " + plain(s.Note)
	}
	return clip(row, width)
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.width < 24 || m.height < 8 {
		return fitLines([]string{"Terminal too small", "Minimum: 24×8", "q / ctrl+c to quit"}, m.width, m.height)
	}
	if m.mode == modeError {
		return fitLines([]string{"Configuration could not be loaded (read-only)", "Path: " + m.path, m.loadError.Error(), "", "Fix the file and restart. q / ctrl+c to quit."}, m.width, m.height)
	}
	if m.mode == modeCopyFallback {
		return m.copyView()
	}
	if m.mode == modeForm || m.mode == modeConfirmDiscard {
		view := m.formView()
		if m.mode == modeConfirmDiscard {
			view = m.confirmationView(view)
		}
		return view
	}
	lines := []string{m.styles.title.Render(clip(fmt.Sprintf("AddressPad  %d/%d servers", len(m.visible), len(m.servers)), m.width))}
	if m.mode == modeSearch {
		lines = append(lines, m.search.View())
	} else {
		lines = append(lines, m.styles.muted.Render(clip("Filter: "+m.search.Value(), m.width)))
	}
	lines = append(lines, strings.Repeat("─", m.width))
	now := time.Now()
	if m.now != nil {
		now = m.now()
	}
	rows := m.height - 5
	for row := range rows {
		i := m.offset + row
		text := ""
		if i < len(m.visible) {
			prefix := "  "
			if i == m.cursor {
				prefix = "> "
			}
			text = prefix + serverRow(m.visible[i], m.width-2, now)
			if i == m.cursor {
				text = m.styles.selected.Render(column(text, m.width))
			}
		} else if row == 0 {
			if len(m.servers) == 0 {
				text = "No servers yet. Press a to add a server."
			} else {
				text = "no match — esc to clear"
			}
			text = clip(text, m.width)
		}
		lines = append(lines, text)
	}
	lines = append(lines, strings.Repeat("─", m.width))
	status := m.status
	if status == "" {
		if m.mode == modeSearch {
			status = "SEARCH | type to filter · ↑/↓ select · esc back"
		} else {
			status = "LIST " + m.position() + " | enter connect · a/e/d edit · / search · ? help · q quit"
		}
	}
	lines = append(lines, m.styles.muted.Render(clip(status, m.width)))
	if m.mode == modeHelp {
		lines = m.helpOverlay(lines)
	}
	if m.mode == modeConfirmDelete {
		return m.confirmationView(strings.Join(lines, "\n"))
	}
	return strings.Join(lines, "\n")
}

func fitLines(lines []string, width, height int) string {
	result := make([]string, min(len(lines), height))
	for i := range result {
		result[i] = clip(lines[i], width)
	}
	return strings.Join(result, "\n")
}

func (m Model) helpOverlay(lines []string) []string {
	content := []string{
		"Help — ? / esc / q closes", "↑/k/ctrl+p   Move up", "↓/j/ctrl+n   Move down", "g / G       First / last server",
		"/           Search (subsequence, AND words)", "esc         Back; clear if no match", "f           Toggle favorite and save", "q / ctrl+c  Quit",
		"enter       Connect using system SSH", "c           Copy SSH command", "a / e / d   Add / edit / delete", "i           Import ~/.ssh/config (skip existing)",
	}
	height := min(len(content)+2, m.height-2)
	width := min(58, m.width-2)
	top := (m.height - height) / 2
	left := (m.width - width) / 2
	for row := range height {
		text := ""
		if row == 0 || row == height-1 {
			text = "+" + strings.Repeat("-", width-2) + "+"
		} else {
			text = "|" + column(content[row-1], width-2) + "|"
		}
		// The panel replaces these rows without changing the underlying state.
		lines[top+row] = strings.Repeat(" ", left) + m.styles.title.Render(text)
	}
	return lines
}
