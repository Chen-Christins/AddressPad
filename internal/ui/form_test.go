package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"addresspad/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func formModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
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
	return update(New(s, path, nil), tea.WindowSizeMsg{Width: 80, Height: 20})
}

func formKey(m Model, key tea.KeyType) Model { return update(m, tea.KeyMsg{Type: key}) }

func TestFormAddAndEdit(t *testing.T) {
	m := press(formModel(t), "a")
	if m.mode != modeForm || m.form.focus != fieldName || m.form.dirty() {
		t.Fatal("invalid initial form")
	}
	m = press(m, "新服务器")
	m = formKey(m, tea.KeyTab)
	m = press(m, "example.cn")
	m.form.inputs[fieldUser].SetValue("deploy")
	m.form.inputs[fieldTags].SetValue(" web, 中文, web, ")
	m.form.inputs[fieldArgs].SetValue(`-o 'ProxyCommand=ssh -W %h:%p jump' ''`)
	m.form.note.SetValue("第一行\n第二行")
	m.form.favorite = true
	m = formKey(m, tea.KeyCtrlS)
	if m.mode != modeList || m.visible[m.cursor].Name != "新服务器" {
		t.Fatalf("add failed: %v %s", m.form.errors, m.form.saveError)
	}
	saved := m.visible[m.cursor]
	if saved.ID == "" || !saved.Favorite || !reflect.DeepEqual(saved.Tags, []string{"web", "中文"}) || saved.Note != "第一行\n第二行" {
		t.Fatalf("saved = %+v", saved)
	}
	if !reflect.DeepEqual(saved.ExtraArgs, []string{"-o", "ProxyCommand=ssh -W %h:%p jump", ""}) {
		t.Fatalf("extra args = %q", saved.ExtraArgs)
	}
	// Preserve metadata that is not exposed by the form.
	saved.LastUsed = time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	if err := m.store.Update(saved.ID, saved); err != nil {
		t.Fatal(err)
	}
	m.servers = sortedServers(m.store.Config().Servers)
	m.refresh(saved.ID)
	m.search.SetValue("新")
	m.refresh(saved.ID)
	m = press(m, "e")
	if m.form.dirty() {
		t.Fatal("edit starts dirty")
	}
	m.form.inputs[fieldName].SetValue("renamed")
	m.form.inputs[fieldHost].SetValue("new.example.cn")
	m = formKey(m, tea.KeyCtrlS)
	got := m.visible[m.cursor]
	if m.mode != modeList || got.ID != saved.ID || got.Name != "renamed" || !got.LastUsed.Equal(saved.LastUsed) || !reflect.DeepEqual(got.ExtraArgs, saved.ExtraArgs) {
		t.Fatalf("edit lost data: %+v", got)
	}
	if m.search.Value() != "" {
		t.Fatal("renamed entry hidden by filter")
	}
	c, err := store.Load(m.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Servers) != 4 {
		t.Fatal("edit created duplicate entry")
	}
}

func TestFormValidation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field int
		value string
	}{
		{"empty name", fieldName, " "},
		{"duplicate", fieldName, "PROD-WEB"},
		{"empty host", fieldHost, ""},
		{"option host", fieldHost, "-bad"},
		{"space host", fieldHost, "bad host"},
		{"zero port", fieldPort, "0"},
		{"high port", fieldPort, "65536"},
		{"noninteger port", fieldPort, "22.5"},
		{"empty port", fieldPort, ""},
		{"unclosed quote", fieldArgs, "'broken"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := press(formModel(t), "a")
			before, err := os.ReadFile(m.path)
			if err != nil {
				t.Fatal(err)
			}
			m.form.inputs[fieldName].SetValue("valid")
			m.form.inputs[fieldHost].SetValue("localhost")
			m.form.inputs[tc.field].SetValue(tc.value)
			m = formKey(m, tea.KeyCtrlS)
			if m.mode != modeForm || m.form.focus != tc.field || m.form.errors[tc.field] == "" {
				t.Fatalf("validation = %v, focus = %d", m.form.errors, m.form.focus)
			}
			after, err := os.ReadFile(m.path)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("validation changed file")
			}
			if !strings.Contains(m.View(), m.form.errors[tc.field]) {
				t.Fatal("inline error not visible")
			}
		})
	}
	// Multiple invalid fields focus the first, not a random map iteration entry.
	m := press(formModel(t), "a")
	m = formKey(m, tea.KeyCtrlS)
	if m.form.focus != fieldName || len(m.form.errors) != 2 {
		t.Fatalf("first error: %v", m.form.errors)
	}
}

func TestFormNavigationAndDiscard(t *testing.T) {
	m := press(formModel(t), "a")
	m = press(m, "esc")
	if m.mode != modeList {
		t.Fatal("clean cancel required confirmation")
	}
	m = press(m, "a")
	m = formKey(m, tea.KeyShiftTab)
	if m.form.focus != fieldArgs {
		t.Fatal("backward navigation did not wrap")
	}
	m = formKey(m, tea.KeyTab)
	m = press(m, "draft")
	m = press(m, "esc")
	if m.mode != modeConfirmDiscard || !strings.Contains(m.View(), "Discard unsaved") {
		t.Fatal("dirty cancel not protected")
	}
	m = press(m, "n")
	if m.mode != modeForm || m.form.inputs[fieldName].Value() != "draft" {
		t.Fatal("continue editing lost input")
	}
	m = press(m, "esc")
	m = press(m, "y")
	if m.mode != modeList || len(m.store.Config().Servers) != 3 {
		t.Fatal("discard saved draft")
	}
	m = press(m, "a")
	m.form.setFocus(fieldNote)
	m = press(m, "第一行")
	m = update(m, tea.KeyMsg{Type: tea.KeyEnter, Alt: true})
	m = press(m, "第二行")
	if m.form.note.Value() != "第一行\n第二行" {
		t.Fatalf("note = %q", m.form.note.Value())
	}
	m = formKey(m, tea.KeyEnter)
	if m.form.focus != fieldFavorite {
		t.Fatal("enter did not navigate")
	}
	m = press(m, " ")
	if !m.form.favorite {
		t.Fatal("space did not toggle")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c did not quit")
	}
}

func TestFormSaveFailureRetainsInput(t *testing.T) {
	m := press(formModel(t), "e")
	id := m.form.original.ID
	m.form.inputs[fieldName].SetValue("unsaved-name")
	// Force backup failure without changing the original data file.
	if err := os.Remove(m.path + ".bak"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(m.path+".bak", 0700); err != nil {
		t.Fatal(err)
	}
	m = formKey(m, tea.KeyCtrlS)
	if m.mode != modeForm || m.form.inputs[fieldName].Value() != "unsaved-name" || !strings.Contains(m.form.saveError, "save failed") {
		t.Fatal("save failure lost form")
	}
	for _, s := range m.store.Config().Servers {
		if s.ID == id && s.Name == "unsaved-name" {
			t.Fatal("failed save mutated memory")
		}
	}
	if err := os.Remove(m.path + ".bak"); err != nil {
		t.Fatal(err)
	}
	m.form.setFocus(fieldArgs)
	m = formKey(m, tea.KeyEnter)
	if m.mode != modeList || m.visible[m.cursor].Name != "unsaved-name" {
		t.Fatal("retry did not save")
	}
}

func TestDeleteConfirmation(t *testing.T) {
	m := formModel(t)
	selected := m.visible[m.cursor]
	m = press(m, "d")
	if m.mode != modeConfirmDelete || !strings.Contains(m.View(), selected.Name) || !strings.Contains(m.View(), selected.Host) {
		t.Fatal("confirmation missing target")
	}
	m = press(m, "j")
	if m.cursor != 0 {
		t.Fatal("confirmation allowed navigation")
	}
	m = press(m, "n")
	if m.mode != modeList || len(m.visible) != 3 {
		t.Fatal("cancel deleted entry")
	}
	m = press(m, "d")
	if err := os.Remove(m.path + ".bak"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(m.path+".bak", 0700); err != nil {
		t.Fatal(err)
	}
	m = press(m, "enter")
	if m.mode != modeConfirmDelete || len(m.visible) != 3 || !strings.Contains(m.status, "delete failed") {
		t.Fatal("failed delete changed state")
	}
	if err := os.Remove(m.path + ".bak"); err != nil {
		t.Fatal(err)
	}
	m = press(m, "y")
	if m.mode != modeList || len(m.visible) != 2 {
		t.Fatal("delete failed")
	}
	for _, s := range m.store.Config().Servers {
		if s.ID == selected.ID {
			t.Fatal("deleted entry persisted")
		}
	}
}

func TestFormLayouts(t *testing.T) {
	m := press(formModel(t), "e")
	for _, size := range [][2]int{{60, 20}, {200, 50}, {24, 8}, {10, 3}} {
		m = update(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for i := range fieldCount {
			m.form.setFocus(i)
			for _, confirm := range []bool{false, true} {
				m.mode = modeForm
				if confirm {
					m.mode = modeConfirmDiscard
				}
				view := m.View()
				if len(strings.Split(view, "\n")) > size[1] {
					t.Fatalf("height overflow %v", size)
				}
				for _, line := range strings.Split(view, "\n") {
					if lipgloss.Width(line) > size[0] {
						t.Fatalf("width overflow %v: %q (%d)", size, line, lipgloss.Width(line))
					}
				}
				if strings.Contains(view, "\x1b") {
					t.Fatal("NO_COLOR contains ANSI")
				}
			}
		}
	}
}

func TestArgsRoundTrip(t *testing.T) {
	for _, args := range [][]string{{}, {"-o", "ProxyCommand=ssh -W %h:%p jump"}, {"", "single'quote", `C:\Users\中文\key`, "$HOME", "a\"b"}} {
		got, err := parseArgs(formatArgs(args))
		if err != nil || !reflect.DeepEqual(got, args) {
			t.Fatalf("round trip %q = %q, %v", args, got, err)
		}
	}
	got, err := parseArgs(`-o "ForwardAgent=yes" path\ with\ spaces`)
	if err != nil || !reflect.DeepEqual(got, []string{"-o", "ForwardAgent=yes", "path with spaces"}) {
		t.Fatalf("parse = %q %v", got, err)
	}
	for _, bad := range []string{"'bad", `"bad`, "bad\\"} {
		if _, err := parseArgs(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestEmptyEditAndDelete(t *testing.T) {
	m := New(nil, "empty.yaml", nil)
	for _, key := range []string{"e", "d"} {
		m = press(m, key)
		if m.mode != modeList || m.status != "No server selected" {
			t.Fatalf("empty %s failed", key)
		}
	}
}
