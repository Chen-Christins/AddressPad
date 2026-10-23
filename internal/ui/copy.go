package ui

// Keep the complete command as a single logical line. The terminal handles
// soft wrapping, avoiding inserted newlines inside quoted arguments.
func (m Model) copyView() string {
	return clip("Clipboard unavailable — copy command below", m.width) + "\n" +
		clip("PowerShell on Windows; POSIX shell elsewhere", m.width) + "\n\n" + m.copyFallback + "\n\n" + clip("esc / q to return", m.width)
}
