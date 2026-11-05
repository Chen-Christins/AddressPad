package ui

import (
	"fmt"
	"strings"
	"time"

	"addresspad/internal/model"
	"github.com/charmbracelet/lipgloss"
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

func serverAddress(s model.Server) string {
	address := s.Host
	if strings.Contains(address, ":") {
		address = "[" + address + "]"
	}
	if s.User != "" {
		address = s.User + "@" + address
	}
	return fmt.Sprintf("%s:%d", address, s.Port)
}

func serverRow(s model.Server, width int, now time.Time) string {
	star := " "
	if s.Favorite {
		star = "*"
	}
	address := serverAddress(s)
	// serverRow receives width-2 for the selection prefix; 58 keeps the
	// aligned columns visible at a 60-column terminal.
	if width < 58 {
		return clip(star+" "+s.Name+"  "+address, width)
	}
	nameWidth := min(28, width/4)
	addressWidth := min(44, width/3)
	row := star + " " + column(s.Name, nameWidth) + " " + column(address, addressWidth)
	row += " " + column(relativeTime(s.LastUsed, now), lastUsedWidth)
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

type rowPart struct {
	text  string
	style lipgloss.Style
}

func fitParts(parts []rowPart, width int) string {
	var b strings.Builder
	remaining := max(0, width)
	for _, part := range parts {
		if remaining == 0 {
			break
		}
		text := clip(part.text, remaining)
		b.WriteString(part.style.Render(text))
		remaining -= lipgloss.Width(text)
	}
	return b.String()
}

func serverRowStyled(s model.Server, width int, now time.Time, st styles) string {
	star := " "
	if s.Favorite {
		star = "*"
	}
	address := serverAddress(s)
	if width < 58 {
		return clip(star+" "+s.Name+"  "+address, width)
	}
	nameWidth := min(28, width/4)
	addressWidth := min(44, width/3)
	fixed := 4 + nameWidth + addressWidth + lastUsedWidth
	parts := make([]rowPart, 0, 2+len(s.Tags))
	if s.Group != "" {
		parts = append(parts, rowPart{" [" + plain(s.Group) + "]", st.group})
	}
	for _, tag := range s.Tags {
		parts = append(parts, rowPart{" #" + plain(tag), st.tag})
	}
	if s.Note != "" {
		parts = append(parts, rowPart{"  " + plain(s.Note), st.muted})
	}
	row := st.favorite.Render(star) + " " +
		st.name.Render(column(s.Name, nameWidth)) + " " +
		st.muted.Render(column(address, addressWidth)) + " " +
		st.muted.Render(column(relativeTime(s.LastUsed, now), lastUsedWidth))
	return row + fitParts(parts, width-fixed)
}

func (m Model) listRows() int { return max(1, m.height-6) }

func (m Model) headerView() string {
	brand := m.styles.title.Render("AddressPad")
	noun := "servers"
	if len(m.servers) == 1 {
		noun = "server"
	}
	count := m.styles.muted.Render(fmt.Sprintf("%d/%d %s", len(m.visible), len(m.servers), noun))
	left := brand + "  " + count
	if lipgloss.Width(left) > m.width {
		left = brand
	}
	if fill := m.width - lipgloss.Width(left) - 1; fill > 0 {
		left += " " + m.styles.rule.Render(strings.Repeat("─", fill))
	}
	return left
}

func (m Model) filterView() string {
	if m.mode == modeSearch {
		return m.search.View()
	}
	prompt := m.styles.accent.Render("/ ")
	query := m.search.Value()
	if query == "" {
		return prompt + m.styles.muted.Render(clip("type to filter", max(0, m.width-2)))
	}
	return prompt + m.styles.text.Render(clip(query, max(0, m.width-2)))
}

func (m Model) columnHeader() string {
	width := m.width - 2
	header := "    NAME  ADDRESS"
	if width >= 58 {
		nameWidth := min(28, width/4)
		addressWidth := min(44, width/3)
		header = "    " + column("NAME", nameWidth) + " " +
			column("ADDRESS", addressWidth) + " " +
			column("USED", lastUsedWidth) + " DETAILS"
	}
	return m.styles.header.Render(clip(header, m.width))
}

func (m Model) rule() string {
	return m.styles.rule.Render(strings.Repeat("─", m.width))
}

func (m Model) centeredLine(text string, style lipgloss.Style) string {
	text = clip(text, m.width)
	left := max(0, (m.width-lipgloss.Width(text))/2)
	return strings.Repeat(" ", left) + style.Render(text)
}

func (m Model) emptyBody(rows int) []string {
	content := []string{
		m.centeredLine("No servers yet", m.styles.title),
		m.centeredLine("press a to add · i to import", m.styles.muted),
	}
	if len(m.servers) > 0 {
		content = []string{
			m.centeredLine("No matches", m.styles.title),
			m.centeredLine("no match — esc to clear filter", m.styles.muted),
		}
	}
	top := max(0, (rows-len(content))/2)
	body := make([]string, rows)
	for i := range content {
		if top+i < rows {
			body[top+i] = content[i]
		}
	}
	return body
}

func (m Model) listBody(rows int, now time.Time) []string {
	if len(m.visible) == 0 {
		return m.emptyBody(rows)
	}
	body := make([]string, 0, rows)
	for row := range rows {
		i := m.offset + row
		if i >= len(m.visible) {
			body = append(body, "")
			continue
		}
		if i == m.cursor {
			text := "> " + serverRow(m.visible[i], m.width-2, now)
			body = append(body, m.styles.selected.Render(column(text, m.width)))
			continue
		}
		body = append(body, "  "+serverRowStyled(m.visible[i], m.width-2, now, m.styles))
	}
	return body
}

func listHint(position string, width int) string {
	switch {
	case width >= 110:
		return "LIST " + position + "  ·  enter connect  ·  a add  ·  e edit  ·  d delete  ·  / search  ·  ? help  ·  q quit"
	case width >= 90:
		return "LIST " + position + "  ·  enter connect  ·  a/e/d edit  ·  / search  ·  ? help  ·  q quit"
	case width >= 50:
		return "LIST " + position + "  ·  enter · a/e/d · / · ? · q"
	default:
		return "LIST " + position
	}
}

func searchHint(width int) string {
	switch {
	case width >= 60:
		return "SEARCH  ·  type to filter  ·  ↑/↓ select  ·  esc back"
	case width >= 40:
		return "SEARCH · type · ↑/↓ · esc"
	default:
		return "SEARCH"
	}
}

func (m Model) statusView() string {
	text := m.status
	style := m.styles.muted
	marker := m.styles.accent
	if text == "" {
		if m.mode == modeSearch {
			text = searchHint(m.width)
		} else {
			text = listHint(m.position(), m.width)
		}
	} else if m.statusFailure {
		style, marker = m.styles.failure, m.styles.failure
	} else {
		style, marker = m.styles.success, m.styles.success
	}
	return marker.Render("| ") + style.Render(clip(text, max(0, m.width-2)))
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.width < 24 || m.height < 8 {
		return fitLines([]string{"Terminal too small", "Minimum: 24×8", "q / ctrl+c to quit"}, m.width, m.height)
	}
	if m.mode == modeError {
		return m.errorView()
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
	lines := []string{m.headerView(), m.filterView(), m.columnHeader(), m.rule()}
	now := time.Now()
	if m.now != nil {
		now = m.now()
	}
	lines = append(lines, m.listBody(m.listRows(), now)...)
	lines = append(lines, m.rule(), m.statusView())
	if m.mode == modeHelp {
		lines = overlayLines(lines, m.helpPanel(), m.width, m.height)
	}
	if m.mode == modeConfirmDelete {
		return strings.Join(overlayLines(lines, m.confirmPanel(), m.width, m.height), "\n")
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
