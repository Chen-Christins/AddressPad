// Package ui implements the terminal interface. All state changes occur in Update.
package ui

import (
	"fmt"
	"io"
	"time"

	"addresspad/internal/model"
	"addresspad/internal/sshcmd"
	"addresspad/internal/store"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type mode int

const (
	modeList mode = iota
	modeSearch
	modeHelp
	modeError
	modeConnecting
	modeCopyFallback
)

type statusExpired struct{ generation uint64 }

type Model struct {
	store            *store.Store
	servers, visible []model.Server
	mode             mode
	returnMode       mode
	search           textinput.Model
	cursor, offset   int
	width, height    int
	status           string
	statusGeneration uint64
	loadError        error
	path             string
	styles           styles
	resumeMode       mode
	command          commandBuilder
	copy             func(string) error
	now              clock
	copyFallback     string
}

func New(s *store.Store, path string, loadError error) Model {
	input := textinput.New()
	input.Prompt = "/ "
	input.CharLimit = 256
	// Unstyled input also respects NO_COLOR regardless of the global color profile.
	input.PromptStyle = newStyles().muted
	input.TextStyle = newStyles().muted
	input.Cursor.Style = newStyles().muted
	m := Model{store: s, path: path, loadError: loadError, search: input, styles: newStyles(), command: sshcmd.Command, copy: sshcmd.Copy, now: time.Now}
	if loadError != nil {
		m.mode = modeError
	} else if s != nil {
		m.servers = sortedServers(s.Config().Servers)
	}
	m.refresh("")
	return m
}

func Run(path string, input io.Reader, output io.Writer) error {
	s, loadError := store.Open(path)
	_, err := tea.NewProgram(New(s, path, loadError), tea.WithInput(input), tea.WithOutput(output), tea.WithAltScreen()).Run()
	return err
}

func (m Model) Init() tea.Cmd { return nil }

func (m *Model) refresh(selectedID string) {
	m.visible = filterServers(m.servers, m.search.Value())
	m.cursor = 0
	for i, s := range m.visible {
		if s.ID == selectedID {
			m.cursor = i
			break
		}
	}
	m.offset = 0
	m.clamp()
}

func (m *Model) clamp() {
	m.cursor = max(0, min(m.cursor, len(m.visible)-1))
	rows := max(1, m.height-5)
	m.offset = max(0, min(m.offset, max(0, len(m.visible)-rows)))
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
}

func (m *Model) notify(text string, persistent bool) tea.Cmd {
	m.status = text
	m.statusGeneration++
	if persistent {
		return nil
	}
	generation := m.statusGeneration
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return statusExpired{generation} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case connectedMsg:
		cmd := m.connected(msg)
		return m, cmd
	case copiedMsg:
		if msg.err != nil {
			m.resumeMode = m.mode
			m.mode = modeCopyFallback
			m.copyFallback = msg.command
			return m, m.notify(msg.err.Error(), true)
		}
		return m, m.notify("copied: "+msg.command, false)
	case tea.WindowSizeMsg:
		m.width, m.height = max(0, msg.Width), max(0, msg.Height)
		m.search.Width = max(1, m.width-2)
		m.clamp()
		return m, nil
	case statusExpired:
		if msg.generation == m.statusGeneration {
			m.status = ""
		}
		return m, nil
	case tea.KeyMsg:
		if m.mode == modeConnecting {
			return m, nil
		}
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.mode == modeCopyFallback {
			if msg.String() == "esc" || msg.String() == "q" {
				m.mode = m.resumeMode
				m.copyFallback = ""
				m.status = ""
			}
			return m, nil
		}
		if m.mode == modeError {
			if key.Matches(msg, keys.quit) {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.mode == modeHelp {
			if msg.String() == "esc" || key.Matches(msg, keys.help, keys.quit) {
				m.mode = m.returnMode
			}
			return m, nil
		}
		if m.mode == modeSearch {
			switch msg.String() {
			case "esc":
				if len(m.visible) == 0 {
					m.search.SetValue("")
					m.refresh("")
				}
				m.search.Blur()
				m.mode = modeList
				return m, nil
			case "enter":
				cmd := m.connect()
				return m, cmd
			case "up", "ctrl+p":
				m.cursor--
				m.clamp()
				return m, nil
			case "down", "ctrl+n":
				m.cursor++
				m.clamp()
				return m, nil
			case "ctrl+u":
				m.search.SetValue("")
				m.refresh("")
				return m, nil
			}
			before := m.search.Value()
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			if m.search.Value() != before {
				m.refresh("")
			}
			return m, cmd
		}
		switch {
		case key.Matches(msg, keys.quit):
			return m, tea.Quit
		case key.Matches(msg, keys.up):
			m.cursor--
		case key.Matches(msg, keys.down):
			m.cursor++
		case key.Matches(msg, keys.first):
			m.cursor = 0
		case key.Matches(msg, keys.last):
			m.cursor = len(m.visible) - 1
		case key.Matches(msg, keys.search):
			m.mode = modeSearch
			return m, m.search.Focus()
		case key.Matches(msg, keys.help):
			m.returnMode = m.mode
			m.mode = modeHelp
		case msg.String() == "esc":
			m.search.SetValue("")
			m.refresh("")
		case msg.String() == "f" && len(m.visible) > 0 && m.store != nil:
			s := m.visible[m.cursor]
			if err := m.store.ToggleFavorite(s.ID); err != nil {
				return m, m.notify("save failed: "+err.Error(), true)
			}
			m.servers = sortedServers(m.store.Config().Servers)
			m.refresh(s.ID)
			verb := "pinned "
			if s.Favorite {
				verb = "unpinned "
			}
			return m, m.notify(verb+s.Name, false)
		case msg.String() == "enter":
			cmd := m.connect()
			return m, cmd
		case msg.String() == "c":
			cmd := m.copyCommand()
			return m, cmd
		case msg.String() == "a", msg.String() == "e", msg.String() == "d":
			return m, m.notify("Server forms will be available in M5", false)
		case msg.String() == "i":
			return m, m.notify("SSH config import will be available in M6", false)
		}
		m.clamp()
		return m, nil
	}
	if m.mode == modeSearch {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) position() string {
	if len(m.visible) == 0 {
		return "0/0"
	}
	return fmt.Sprintf("%d/%d", m.cursor+1, len(m.visible))
}
