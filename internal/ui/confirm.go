package ui

import tea "github.com/charmbracelet/bubbletea"

func (m *Model) confirm(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if m.mode == modeConfirmDiscard {
		if key == "y" {
			m.mode = modeList
		}
		if key == "n" || key == "esc" {
			m.mode = modeForm
			return m.form.setFocus(m.form.focus)
		}
		return nil
	}
	if key == "n" || key == "esc" {
		m.mode = modeList
		return nil
	}
	if key != "y" && key != "enter" {
		return nil
	}
	if m.store == nil {
		return m.notify("delete failed: storage is unavailable", true)
	}
	if err := m.store.Delete(m.deleteTarget.ID); err != nil {
		return m.notify("delete failed: "+err.Error(), true)
	}
	oldCursor := m.cursor
	m.servers = sortedServers(m.store.Config().Servers)
	m.refresh("")
	m.cursor = oldCursor
	m.clamp()
	m.mode = modeList
	return m.notify("deleted "+m.deleteTarget.Name, false)
}
