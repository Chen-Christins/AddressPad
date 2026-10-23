package ui

import (
	"addresspad/internal/model"
	"fmt"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

const (
	fieldName = iota
	fieldHost
	fieldUser
	fieldPort
	fieldIdentity
	fieldProxy
	fieldGroup
	fieldTags
	fieldNote
	fieldFavorite
	fieldArgs
	fieldCount
)

var fieldLabels = []string{"Name *", "Host *", "User", "Port", "Identity file", "Proxy jump", "Group", "Tags (comma separated)", "Note (alt+enter: newline)", "Favorite (space: toggle)", "Extra args (quoted words)"}

type serverForm struct {
	inputs          []textinput.Model
	note            textarea.Model
	favorite        bool
	focus           int
	original        model.Server
	initial         []string
	initialFavorite bool
	editing         bool
	errors          map[int]string
	saveError       string
}

func newForm(server model.Server, editing bool, style styles) serverForm {
	f := serverForm{original: server, editing: editing, favorite: server.Favorite, initialFavorite: server.Favorite, errors: map[int]string{}}
	port := server.Port
	if port == 0 {
		port = 22
	}
	values := []string{server.Name, server.Host, server.User, strconv.Itoa(port), server.IdentityFile, server.ProxyJump, server.Group, strings.Join(server.Tags, ", "), server.Note, "", formatArgs(server.ExtraArgs)}
	for _, value := range values {
		input := textinput.New()
		input.Prompt = ""
		input.SetValue(value)
		input.TextStyle, input.PromptStyle, input.Cursor.Style = style.muted, style.muted, style.muted
		f.inputs = append(f.inputs, input)
	}
	f.note = textarea.New()
	f.note.Prompt = ""
	f.note.ShowLineNumbers = false
	f.note.CharLimit = 0
	f.note.SetValue(server.Note)
	f.note.SetHeight(3)
	f.note.FocusedStyle = textarea.Style{Text: style.muted, CursorLine: style.muted, Placeholder: style.muted}
	f.note.BlurredStyle = f.note.FocusedStyle
	f.note.Cursor.Style = style.muted
	f.note.KeyMap.InsertNewline.SetEnabled(false)
	f.initial = append([]string{}, values...)
	return f
}

func (f serverForm) values() []string {
	values := make([]string, len(f.inputs))
	for i := range values {
		values[i] = f.inputs[i].Value()
	}
	values[fieldNote] = f.note.Value()
	return values
}

func (f serverForm) dirty() bool {
	return !reflect.DeepEqual(f.values(), f.initial) || f.favorite != f.initialFavorite
}

func (f *serverForm) setFocus(index int) tea.Cmd {
	for i := range f.inputs {
		f.inputs[i].Blur()
	}
	f.note.Blur()
	f.focus = (index + fieldCount) % fieldCount
	if f.focus == fieldNote {
		return f.note.Focus()
	}
	if f.focus == fieldFavorite {
		return nil
	}
	return f.inputs[f.focus].Focus()
}

func (m *Model) openForm(editing bool) tea.Cmd {
	server := model.Server{}
	if editing {
		if len(m.visible) == 0 {
			return m.notify("No server selected", false)
		}
		server = m.visible[m.cursor]
	}
	m.form = newForm(server, editing, m.styles)
	m.mode = modeForm
	return m.form.setFocus(0)
}

func (m *Model) saveForm() tea.Cmd {
	f := &m.form
	f.errors = map[int]string{}
	f.saveError = ""
	v := f.values()
	s := f.original
	s.Name, s.Host, s.User = strings.TrimSpace(v[fieldName]), v[fieldHost], v[fieldUser]
	s.IdentityFile, s.ProxyJump, s.Group = v[fieldIdentity], v[fieldProxy], v[fieldGroup]
	s.Tags, s.Note, s.Favorite = strings.Split(v[fieldTags], ","), v[fieldNote], f.favorite
	if s.Name == "" {
		f.errors[fieldName] = "Name is required"
	}
	for _, existing := range m.servers {
		if strings.EqualFold(existing.Name, s.Name) && (!f.editing || existing.ID != s.ID) {
			f.errors[fieldName] = "Name already exists (case insensitive)"
		}
	}
	probe := model.Server{Name: "probe", Host: s.Host, Port: 22}
	if err := probe.Validate(); err != nil {
		f.errors[fieldHost] = "Host is required, without whitespace or leading '-'"
	}
	port, err := strconv.Atoi(v[fieldPort])
	if err != nil || port < 1 || port > 65535 {
		f.errors[fieldPort] = "Port must be an integer between 1 and 65535"
	}
	s.Port = port
	s.ExtraArgs, err = parseArgs(v[fieldArgs])
	if err != nil {
		f.errors[fieldArgs] = err.Error()
	}
	for i := range fieldCount {
		if _, invalid := f.errors[i]; invalid {
			return f.setFocus(i)
		}
	}
	s.Normalize()
	if m.store == nil {
		f.saveError = "save failed: storage is unavailable"
		return nil
	}
	if f.editing {
		err = m.store.Update(s.ID, s)
	} else {
		err = m.store.Add(s)
	}
	if err != nil {
		f.saveError = "save failed: " + err.Error()
		return nil
	}
	m.servers = sortedServers(m.store.Config().Servers)
	if len(filterServers([]model.Server{s}, m.search.Value())) == 0 {
		m.search.SetValue("")
	}
	m.refresh(s.ID)
	m.mode = modeList
	return m.notify("saved "+s.Name, false)
}

func (m *Model) updateForm(msg tea.Msg) tea.Cmd {
	f := &m.form
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			if f.dirty() {
				m.mode = modeConfirmDiscard
				return nil
			}
			m.mode = modeList
			return nil
		case "ctrl+s":
			return m.saveForm()
		case "tab", "down":
			return f.setFocus(f.focus + 1)
		case "shift+tab", "up":
			return f.setFocus(f.focus - 1)
		case "enter":
			if f.focus == fieldCount-1 {
				return m.saveForm()
			}
			return f.setFocus(f.focus + 1)
		case "alt+enter":
			if f.focus == fieldNote {
				f.note.InsertString("\n")
			}
			return nil
		case " ":
			if f.focus == fieldFavorite {
				f.favorite = !f.favorite
				return nil
			}
		}
	}
	var cmd tea.Cmd
	if f.focus == fieldNote {
		f.note, cmd = f.note.Update(msg)
	} else if f.focus != fieldFavorite {
		f.inputs[f.focus], cmd = f.inputs[f.focus].Update(msg)
	}
	return cmd
}

// parseArgs accepts quoted words and backslash escapes, without shell expansion.
func parseArgs(text string) ([]string, error) {
	args := []string{}
	var word strings.Builder
	var quote rune
	started, escaped := false, false
	for _, r := range text {
		if escaped {
			word.WriteRune(r)
			escaped = false
			started = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if unicode.IsSpace(r) {
			if started {
				args = append(args, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		word.WriteRune(r)
		started = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("Unclosed quote in extra args")
	}
	if escaped {
		return nil, fmt.Errorf("Trailing escape in extra args")
	}
	if started {
		args = append(args, word.String())
	}
	return args, nil
}

func formatArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}
	return strings.Join(quoted, " ")
}
