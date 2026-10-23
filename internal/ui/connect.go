package ui

import (
	"fmt"
	"os/exec"
	"time"

	"addresspad/internal/model"
	"addresspad/internal/sshcmd"
	tea "github.com/charmbracelet/bubbletea"
)

type connectedMsg struct {
	id  string
	err error
}
type copiedMsg struct {
	command string
	err     error
}

func (m *Model) connect() tea.Cmd {
	if len(m.visible) == 0 {
		return m.notify("No server selected", false)
	}
	s := m.visible[m.cursor]
	cmd, err := m.command(s)
	if err != nil {
		return m.notify(err.Error(), true)
	}
	m.resumeMode = m.mode
	m.mode = modeConnecting
	m.search.Blur()
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return connectedMsg{s.ID, err} })
}

func (m *Model) connected(msg connectedMsg) tea.Cmd {
	m.mode = m.resumeMode
	code := sshcmd.ExitCode(msg.err)
	status := fmt.Sprintf("ssh exited with code %d", code)
	persistent := code != 0
	if code == -1 {
		status = "ssh could not start: " + msg.err.Error()
	}
	// A clean SSH exit is the only reliable success signal exposed by OpenSSH.
	if code == 0 && m.store != nil {
		for _, s := range m.store.Config().Servers {
			if s.ID != msg.id {
				continue
			}
			s.LastUsed = m.now()
			if err := m.store.Update(s.ID, s); err != nil {
				status += "; save failed: " + err.Error()
				persistent = true
			} else {
				m.servers = sortedServers(m.store.Config().Servers)
				m.refresh(s.ID)
			}
			break
		}
	}
	commands := []tea.Cmd{tea.ClearScreen, m.notify(status, persistent)}
	if m.mode == modeSearch {
		commands = append(commands, m.search.Focus())
	}
	return tea.Batch(commands...)
}

func (m *Model) copyCommand() tea.Cmd {
	if len(m.visible) == 0 {
		return m.notify("No server selected", false)
	}
	text, err := sshcmd.Display(m.visible[m.cursor])
	if err != nil {
		return m.notify(err.Error(), true)
	}
	copy := m.copy
	return func() tea.Msg { return copiedMsg{text, copy(text)} }
}

// Injectable dependencies keep UI tests from opening connections or clipboards.
type commandBuilder func(model.Server) (*exec.Cmd, error)
type clock func() time.Time
