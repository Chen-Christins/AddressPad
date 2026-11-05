package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"addresspad/internal/model"
	"addresspad/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func fixture() []model.Server {
	return []model.Server{
		{ID: "1", Name: "prod-web", Host: "10.0.0.1", User: "deploy", Port: 22, Group: "production", Tags: []string{"web", "中文"}},
		{ID: "2", Name: "staging", Host: "example.com", User: "ubuntu", Port: 2222, Group: "stage", Note: "not-searchable"},
		{ID: "3", Name: "数据库", Host: "db.local", Port: 22, Tags: []string{"存储"}},
	}
}

func testModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("NO_COLOR", "1")
	m := New(nil, "servers.yaml", nil)
	m.servers = sortedServers(fixture())
	m.refresh("")
	return update(m, tea.WindowSizeMsg{Width: 80, Height: 20})
}

func update(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func press(m Model, text string) Model {
	special := map[string]tea.KeyType{"esc": tea.KeyEsc, "enter": tea.KeyEnter, "ctrl+c": tea.KeyCtrlC, "ctrl+u": tea.KeyCtrlU, "ctrl+w": tea.KeyCtrlW, "backspace": tea.KeyBackspace, "up": tea.KeyUp, "down": tea.KeyDown}
	if k, ok := special[text]; ok {
		return update(m, tea.KeyMsg{Type: k})
	}
	return update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
}

func TestFilterServers(t *testing.T) {
	for _, tc := range []struct {
		query string
		names []string
	}{
		{"", []string{"prod-web", "staging", "数据库"}},
		{"PWB", []string{"prod-web"}},
		{"pwb dep 中文", []string{"prod-web"}},
		{"exm ub", []string{"staging"}},
		{"数库 存", []string{"数据库"}},
		{"prod stage", []string{}},
		{"not-searchable", []string{}},
		{"webdeploy", []string{}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			got := []string{}
			for _, s := range filterServers(fixture(), tc.query) {
				got = append(got, s.Name)
			}
			if !reflect.DeepEqual(got, tc.names) {
				t.Fatalf("got %v, want %v", got, tc.names)
			}
		})
	}
}

func TestSortServers(t *testing.T) {
	now := time.Now()
	servers := []model.Server{{Name: "Zulu"}, {Name: "beta"}, {Name: "Alpha"}, {Name: "recent", LastUsed: now}, {Name: "favorite", Favorite: true}}
	got := sortedServers(servers)
	want := []string{"favorite", "recent", "Alpha", "beta", "Zulu"}
	for i := range got {
		if got[i].Name != want[i] {
			t.Fatalf("sort = %+v", got)
		}
	}
	if servers[0].Name != "Zulu" {
		t.Fatal("sort modified source")
	}
	filtered := filterServers(got, "a")
	if filtered[0].Name != "favorite" {
		t.Fatal("filter changed sort order")
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		age  time.Duration
		want string
	}{
		{-time.Minute, "now"},
		{0, "now"},
		{999 * time.Millisecond, "now"},
		{time.Second, "1s ago"},
		{59 * time.Second, "59s ago"},
		{time.Minute, "1m ago"},
		{59 * time.Minute, "59m ago"},
		{time.Hour, "1h ago"},
		{23 * time.Hour, "23h ago"},
		{24 * time.Hour, "1d ago"},
		{6 * 24 * time.Hour, "6d ago"},
		{7 * 24 * time.Hour, "1w ago"},
		{29 * 24 * time.Hour, "4w ago"},
		{30 * 24 * time.Hour, "1mo ago"},
		{364 * 24 * time.Hour, "12mo ago"},
		{365 * 24 * time.Hour, "1y ago"},
		{2 * 365 * 24 * time.Hour, "2y ago"},
	} {
		if got := relativeTime(now.Add(-tc.age), now); got != tc.want {
			t.Fatalf("age %v = %q, want %q", tc.age, got, tc.want)
		}
	}
	if got := relativeTime(time.Time{}, now); got != "never" {
		t.Fatalf("zero time = %q, want never", got)
	}
}

func TestLastUsedColumn(t *testing.T) {
	m := testModel(t)
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	m.servers = []model.Server{
		{ID: "1", Name: "recent", Host: "a.example", Port: 22, LastUsed: now.Add(-5 * time.Minute)},
		{ID: "2", Name: "older", Host: "b.example", Port: 22, LastUsed: now.Add(-2 * time.Hour)},
		{ID: "3", Name: "unused", Host: "c.example", Port: 22},
	}
	m.refresh("")
	m = update(m, tea.WindowSizeMsg{Width: 100, Height: 20})
	for _, want := range []string{"5m ago", "2h ago", "never"} {
		if !strings.Contains(m.View(), want) {
			t.Fatalf("view missing %q:\n%s", want, m.View())
		}
	}
	m = update(m, tea.WindowSizeMsg{Width: 60, Height: 20})
	if !strings.Contains(m.View(), "5m ago") {
		t.Fatal("60-column layout should show the last-used column")
	}
	m = update(m, tea.WindowSizeMsg{Width: 50, Height: 20})
	if strings.Contains(m.View(), "5m ago") {
		t.Fatal("narrow layout should hide the last-used column")
	}
}

func TestLastUsedColumnAlignment(t *testing.T) {
	m := testModel(t)
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	m.servers = []model.Server{
		{ID: "1", Name: "root", Host: "154.37.219.7", User: "root", Port: 22, LastUsed: now.Add(-45 * time.Second)},
		{ID: "2", Name: "web", Host: "154.37.219.7", User: "web", Port: 22, LastUsed: now.Add(-21 * time.Minute)},
		{ID: "3", Name: "db", Host: "154.37.219.7", User: "db", Port: 22, Group: "prod", Tags: []string{"db"}, LastUsed: now.Add(-2 * time.Hour)},
	}
	m.refresh("")
	m = update(m, tea.WindowSizeMsg{Width: 100, Height: 12})
	lines := strings.Split(m.View(), "\n")
	usedColumn, detailsColumn := -1, -1
	for _, line := range lines {
		if strings.Contains(line, "USED") && strings.Contains(line, "DETAILS") {
			usedColumn = strings.Index(line, "USED")
			detailsColumn = strings.Index(line, "DETAILS")
			break
		}
	}
	if usedColumn < 0 || detailsColumn < 0 {
		t.Fatalf("column header missing:\n%s", m.View())
	}
	for _, tc := range []struct {
		time, detail string
	}{
		{"45s ago", ""},
		{"21m ago", ""},
		{"2h ago", "[prod]"},
	} {
		found := false
		for _, line := range lines {
			index := strings.Index(line, tc.time)
			if index < 0 {
				continue
			}
			found = true
			if index != usedColumn {
				t.Fatalf("%q starts at %d, want %d:\n%s", tc.time, index, usedColumn, m.View())
			}
			if tc.detail != "" {
				if detailIndex := strings.Index(line, tc.detail); detailIndex != detailsColumn {
					t.Fatalf("%q starts at %d, want %d:\n%s", tc.detail, detailIndex, detailsColumn, m.View())
				}
			}
		}
		if !found {
			t.Fatalf("%q not rendered:\n%s", tc.time, m.View())
		}
	}
}

func TestClockTick(t *testing.T) {
	m := testModel(t)
	if m.Init() == nil {
		t.Fatal("periodic clock tick not scheduled")
	}
	if _, cmd := m.Update(clockTickMsg{}); cmd == nil {
		t.Fatal("clock tick did not reschedule")
	}
}

func TestNavigationAndHelp(t *testing.T) {
	m := testModel(t)
	m = press(m, "G")
	if m.cursor != 2 {
		t.Fatal("G did not go to last")
	}
	m = press(m, "j")
	if m.cursor != 2 {
		t.Fatal("cursor moved beyond end")
	}
	m = press(m, "k")
	if m.cursor != 1 {
		t.Fatal("k did not move up")
	}
	m = press(m, "?")
	m = press(m, "g")
	if m.mode != modeHelp || m.cursor != 1 {
		t.Fatal("help did not block navigation")
	}
	if !strings.Contains(m.View(), "Help") {
		t.Fatal("help not rendered")
	}
	m = press(m, "q")
	if m.mode != modeList || m.cursor != 1 {
		t.Fatal("help close lost list state")
	}
	m = press(m, "g")
	if m.cursor != 0 {
		t.Fatal("g did not go to first")
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q did not return quit command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q did not quit")
	}
}

func TestSearchMode(t *testing.T) {
	m := press(testModel(t), "/")
	if m.mode != modeSearch || !m.search.Focused() {
		t.Fatal("search did not focus")
	}
	m = press(m, "pwb")
	if len(m.visible) != 1 || m.visible[0].Name != "prod-web" {
		t.Fatal("search did not filter")
	}
	m = press(m, "esc")
	if m.mode != modeList || len(m.visible) != 1 || m.search.Value() != "pwb" {
		t.Fatal("esc lost filter")
	}
	m = press(m, "/")
	m = press(m, "ctrl+u")
	m = press(m, "prod web")
	m = press(m, "ctrl+w")
	if strings.TrimSpace(m.search.Value()) != "prod" {
		t.Fatalf("ctrl+w: %q", m.search.Value())
	}
	m = press(m, "ctrl+u")
	m = press(m, "missing")
	if len(m.visible) != 0 || !strings.Contains(m.View(), "no match") {
		t.Fatal("no match state missing")
	}
	m = press(m, "esc")
	if m.search.Value() != "" || len(m.visible) != 3 {
		t.Fatal("no-match esc did not clear")
	}
}

func TestLayoutAndScrolling(t *testing.T) {
	m := testModel(t)
	for i := range 100 {
		m.servers = append(m.servers, model.Server{ID: fmt.Sprint(i + 10), Name: fmt.Sprintf("server-%03d 中文😀", i), Host: "localhost", Port: 22})
	}
	m.refresh("")
	for _, size := range [][2]int{{60, 20}, {200, 50}, {24, 8}, {59, 20}, {10, 3}} {
		m = update(m, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = press(m, "G")
		if m.cursor != len(m.visible)-1 {
			t.Fatal("last item not selected")
		}
		if m.cursor < m.offset || m.cursor >= m.offset+max(1, m.listRows()) {
			t.Fatal("selected item outside viewport")
		}
		for _, view := range []string{m.View(), press(m, "?").View()} {
			lines := strings.Split(view, "\n")
			if len(lines) > size[1] {
				t.Fatalf("too many lines at %v: %d", size, len(lines))
			}
			for _, line := range lines {
				if lipgloss.Width(line) > size[0] {
					t.Fatalf("line overflows %v: %q (%d)", size, line, lipgloss.Width(line))
				}
			}
			if strings.Contains(view, "\x1b") {
				t.Fatal("NO_COLOR output contains escape sequences")
			}
		}
	}
}

func TestFavoriteAndStatus(t *testing.T) {
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
	m := update(New(s, path, nil), tea.WindowSizeMsg{Width: 80, Height: 20})
	m = press(m, "G")
	id := m.visible[m.cursor].ID
	m = press(m, "f")
	if m.cursor != 0 || m.visible[0].ID != id || !m.visible[0].Favorite {
		t.Fatal("favorite sort lost selection")
	}
	if !strings.HasPrefix(m.status, "pinned") {
		t.Fatal("missing status")
	}
	generation := m.statusGeneration
	m = update(m, statusExpired{generation - 1})
	if m.status == "" {
		t.Fatal("old timer cleared newer status")
	}
	m = update(m, statusExpired{generation})
	if m.status != "" {
		t.Fatal("timer did not clear status")
	}
	if err := os.WriteFile(path, []byte("broken: ["), 0600); err != nil {
		t.Fatal(err)
	}
	m = press(m, "f")
	if !m.visible[0].Favorite || !strings.HasPrefix(m.status, "save failed") || !m.statusFailure {
		t.Fatal("failed favorite save changed UI state or lost failure status")
	}
	m.status = ""
	m.statusFailure = false
	m = update(m, statusExpired{m.statusGeneration})
	if m.status != "" || m.statusFailure {
		t.Fatal("expired success status kept failure flag")
	}
	m.notify("save failed", true)
	if !m.statusFailure {
		t.Fatal("persistent notification did not mark failure")
	}
}

func TestReadOnlyErrorAndEmptyState(t *testing.T) {
	m := update(New(nil, "bad.yaml", fmt.Errorf("line 12: invalid YAML")), tea.WindowSizeMsg{Width: 80, Height: 20})
	before := m.View()
	m = press(m, "a")
	if m.View() != before || !strings.Contains(before, "line 12") || !strings.Contains(before, "bad.yaml") {
		t.Fatal("error page not read-only or missing details")
	}
	m = update(New(nil, "empty.yaml", nil), tea.WindowSizeMsg{Width: 80, Height: 20})
	if !strings.Contains(m.View(), "No servers yet") {
		t.Fatal("missing empty state")
	}
	m = press(m, "G")
	if m.cursor != 0 {
		t.Fatal("empty cursor invalid")
	}
	if got := plain("name\x1b[31m\n"); strings.ContainsAny(got, "\x1b\n") {
		t.Fatal("terminal control characters not removed")
	}
}

func BenchmarkFilter1000(b *testing.B) {
	servers := make([]model.Server, 1000)
	for i := range servers {
		servers[i] = model.Server{Name: fmt.Sprintf("prod-web-%d", i), Host: "example.com", Tags: []string{"production", "web"}}
	}
	b.ResetTimer()
	for b.Loop() {
		filterServers(servers, "pwb prod")
	}
}
