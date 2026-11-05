package ui

// Keep the complete command as a single logical line. The terminal handles
// soft wrapping, avoiding inserted newlines inside quoted arguments.
func (m Model) copyView() string {
	lines := []string{
		m.styles.title.Render(clip("Clipboard unavailable", m.width)),
		m.rule(),
		m.styles.muted.Render(clip("PowerShell on Windows; POSIX shell elsewhere", m.width)),
		"",
		m.copyFallback,
		"",
		m.styles.muted.Render(clip("esc / q to return", m.width)),
	}
	return fitLines(lines, m.width, m.height)
}
