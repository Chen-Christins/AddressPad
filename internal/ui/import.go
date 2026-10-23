package ui

import (
	"fmt"
	"strings"

	"addresspad/internal/sshconfig"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) importSSH() tea.Cmd {
	result, err := sshconfig.Parse("~/.ssh/config")
	if err != nil {
		return m.notify("import failed: "+err.Error(), true)
	}
	if m.store == nil {
		return m.notify("import failed: storage unavailable", true)
	}
	c, actions, err := sshconfig.Merge(m.store.Config(), result.Servers, false)
	if err != nil {
		return m.notify("import failed: "+err.Error(), true)
	}
	count := 0
	for _, action := range actions {
		if strings.HasPrefix(action, "import ") {
			count++
		}
	}
	if count > 0 {
		if err := m.store.Replace(c); err != nil {
			return m.notify("import failed: "+err.Error(), true)
		}
		m.servers = sortedServers(m.store.Config().Servers)
		m.refresh("")
	}
	return m.notify(fmt.Sprintf("imported %d servers; skipped %d", count, len(actions)-count+len(result.Skipped)), false)
}
