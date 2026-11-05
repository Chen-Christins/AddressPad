package ui

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"addresspad/internal/model"
	"addresspad/internal/sshcmd"
	tea "github.com/charmbracelet/bubbletea"
)

// clearExecCommand clears the normal screen after Bubble Tea leaves the
// alternate screen and before SSH starts. Without it, output from the previous
// SSH session remains visible above the next session. It clears again after SSH
// exits so quitting AddressPad does not reveal leftover SSH output; the TUI
// reports the session result in its own status bar.
type clearExecCommand struct {
	*exec.Cmd
	out io.Writer
}

func (c *clearExecCommand) SetStdin(r io.Reader) {
	if c.Stdin == nil {
		c.Stdin = r
	}
}

func (c *clearExecCommand) SetStdout(w io.Writer) {
	if c.Stdout == nil {
		c.Stdout = w
	}
	c.out = c.Stdout
}

func (c *clearExecCommand) SetStderr(w io.Writer) {
	if c.Stderr == nil {
		c.Stderr = w
	}
}

func (c *clearExecCommand) Run() error {
	if c.out != nil {
		_, _ = io.WriteString(c.out, "\x1b[2J\x1b[H")
	}
	err := c.Cmd.Run()
	if c.out != nil {
		_, _ = io.WriteString(c.out, "\x1b[2J\x1b[H")
	}
	return err
}

type connectedMsg struct {
	id      string
	started time.Time
	err     error
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
	m.pendingWarning = ""
	if s.PasswordSaved && m.secrets != nil {
		var exeErr error
		executable := ""
		if m.executable != nil {
			executable, exeErr = m.executable()
		} else {
			exeErr = errors.New("executable path is unavailable")
		}
		if exeErr == nil {
			exeErr = sshcmd.EnableAskpass(cmd, executable, s.ID)
		}
		if exeErr != nil {
			m.pendingWarning = "saved password unavailable: " + exeErr.Error()
		}
	}
	m.resumeMode = m.mode
	m.mode = modeConnecting
	m.search.Blur()
	started := m.now()
	return tea.Exec(&clearExecCommand{Cmd: cmd}, func(err error) tea.Msg {
		return connectedMsg{id: s.ID, started: started, err: err}
	})
}

func (m *Model) connected(msg connectedMsg) tea.Cmd {
	m.mode = m.resumeMode
	code := sshcmd.ExitCode(msg.err)
	status := fmt.Sprintf("SSH session ended (exit code %d); back in AddressPad", code)
	persistent := code != 0
	if code == -1 {
		status = "SSH could not start: " + msg.err.Error()
	}
	if m.pendingWarning != "" {
		status += "; " + m.pendingWarning
		persistent = true
		m.pendingWarning = ""
	}
	// A clean SSH exit is the only reliable success signal exposed by OpenSSH.
	if code == 0 && m.store != nil {
		usedAt := msg.started
		if usedAt.IsZero() {
			usedAt = m.now()
		}
		for _, s := range m.store.Config().Servers {
			if s.ID != msg.id {
				continue
			}
			s.LastUsed = usedAt
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
