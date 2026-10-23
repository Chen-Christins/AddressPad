package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"addresspad/internal/model"
	"addresspad/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

type processTestModel struct {
	Model
	result *error
}

func (m processTestModel) Init() tea.Cmd {
	return func() tea.Msg { return tea.KeyMsg{Type: tea.KeyEnter} }
}

func (m processTestModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.Model.Update(msg)
	m.Model = next.(Model)
	if result, ok := msg.(connectedMsg); ok {
		*m.result = result.err
		return m, tea.Quit
	}
	return m, cmd
}

func TestExecProcessIntegration(t *testing.T) {
	if os.Getenv("ADDRESSPAD_UI_SSH_HELPER") == "1" {
		fmt.Println("simulated SSH output")
		if os.Getenv("ADDRESSPAD_UI_SSH_FAIL") == "1" {
			os.Exit(23)
		}
		os.Exit(0)
	}
	for _, fail := range []bool{false, true} {
		m := testModel(t)
		m.command = func(model.Server) (*exec.Cmd, error) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestExecProcessIntegration$")
			cmd.Env = append(os.Environ(), "ADDRESSPAD_UI_SSH_HELPER=1")
			if fail {
				cmd.Env = append(cmd.Env, "ADDRESSPAD_UI_SSH_FAIL=1")
			}
			return cmd, nil
		}
		var result error
		var out bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		p := tea.NewProgram(processTestModel{m, &result}, tea.WithContext(ctx), tea.WithInput(strings.NewReader("")), tea.WithOutput(&out), tea.WithoutRenderer(), tea.WithoutSignalHandler())
		_, err := p.Run()
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "simulated SSH output") {
			t.Fatal("child stdout was not handed to terminal output")
		}
		if fail {
			var exit *exec.ExitError
			if !errors.As(result, &exit) || exit.ExitCode() != 23 {
				t.Fatalf("result = %v", result)
			}
		} else if result != nil {
			t.Fatalf("success result = %v", result)
		}
	}
}

func TestConnectTransitions(t *testing.T) {
	for _, search := range []bool{false, true} {
		m := testModel(t)
		if search {
			m = press(m, "/")
			m = press(m, "prod")
		}
		m.command = func(s model.Server) (*exec.Cmd, error) {
			if s.ID != m.visible[m.cursor].ID {
				t.Fatal("wrong selected server")
			}
			return exec.Command("ssh", "--", s.Host), nil
		}
		next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = next.(Model)
		if m.mode != modeConnecting || cmd == nil {
			t.Fatal("connection did not start")
		}
		m = press(m, "q")
		if m.mode != modeConnecting {
			t.Fatal("keys processed while connecting")
		}
		m = update(m, connectedMsg{id: "1"})
		want := modeList
		if search {
			want = modeSearch
		}
		if m.mode != want || m.status != "ssh exited with code 0" {
			t.Fatalf("return state: %+v", m)
		}
		if search && (m.search.Value() != "prod" || !m.search.Focused()) {
			t.Fatal("search state lost")
		}
	}
}

func TestConnectErrorsAndEmpty(t *testing.T) {
	m := testModel(t)
	m.command = func(model.Server) (*exec.Cmd, error) { return nil, errors.New("install ssh") }
	m = press(m, "enter")
	if m.mode != modeList || !strings.Contains(m.status, "install ssh") {
		t.Fatal("launch error missing")
	}
	m = update(m, connectedMsg{id: "1", err: errors.New("launch failed")})
	if !strings.Contains(m.status, "could not start") {
		t.Fatal("start error missing")
	}
	m.visible = nil
	m = press(m, "enter")
	if m.status != "No server selected" {
		t.Fatal("empty connect not handled")
	}
	m = press(m, "c")
	if m.status != "No server selected" {
		t.Fatal("empty copy not handled")
	}
}

func TestConnectionUpdatesLastUsed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.yaml")
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, server := range fixture() {
		if err := s.Add(server); err != nil {
			t.Fatal(err)
		}
	}
	m := New(s, path, nil)
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	m = update(m, connectedMsg{id: "2"})
	if m.visible[0].ID != "2" || !m.visible[0].LastUsed.Equal(now) {
		t.Fatal("success did not update and sort")
	}
	c, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Servers[1].LastUsed.Equal(now) {
		t.Fatal("last_used was not persisted")
	}
	m = update(m, connectedMsg{id: "1", err: errors.New("failed")})
	if !s.Config().Servers[0].LastUsed.IsZero() {
		t.Fatal("failure updated last_used")
	}
	if err := os.WriteFile(path, []byte("broken: ["), 0600); err != nil {
		t.Fatal(err)
	}
	m = update(m, connectedMsg{id: "1"})
	if !strings.Contains(m.status, "save failed") || !s.Config().Servers[0].LastUsed.IsZero() {
		t.Fatal("save failure not retained")
	}
}

func TestCopyCommand(t *testing.T) {
	m := testModel(t)
	m.copy = func(text string) error {
		if !strings.Contains(text, "--") {
			t.Fatal("missing separator")
		}
		return nil
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = update(next.(Model), cmd())
	if !strings.HasPrefix(m.status, "copied:") {
		t.Fatal("copy success missing")
	}
	m.copy = func(string) error { return errors.New("no clipboard tool") }
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = update(next.(Model), cmd())
	if m.mode != modeCopyFallback || !strings.Contains(m.View(), m.copyFallback) || m.copyFallback == "" {
		t.Fatal("full fallback command not rendered")
	}
	m = press(m, "esc")
	if m.mode != modeList || m.copyFallback != "" {
		t.Fatal("fallback did not close")
	}
}
