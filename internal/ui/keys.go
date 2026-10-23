package ui

import "github.com/charmbracelet/bubbles/key"

var keys = struct {
	up, down, first, last, search, help, quit key.Binding
}{
	up:     key.NewBinding(key.WithKeys("up", "k", "ctrl+p"), key.WithHelp("↑/k", "up")),
	down:   key.NewBinding(key.WithKeys("down", "j", "ctrl+n"), key.WithHelp("↓/j", "down")),
	first:  key.NewBinding(key.WithKeys("g"), key.WithHelp("g/G", "first/last")),
	last:   key.NewBinding(key.WithKeys("G")),
	search: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	quit:   key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
}
