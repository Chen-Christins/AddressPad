package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportShortcut(t *testing.T) {
	m := formModel(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path := filepath.Join(home, ".ssh", "config")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("Host imported\n HostName new.example\nHost prod-web\n HostName must-not-overwrite\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m = press(m, "i")
	if len(m.visible) != 4 || !strings.Contains(m.status, "imported 1 servers") {
		t.Fatalf("import: %s", m.status)
	}
	for _, s := range m.store.Config().Servers {
		if s.Name == "prod-web" && s.Host != "10.0.0.1" {
			t.Fatal("shortcut overwrote existing server")
		}
	}
	m = press(m, "i")
	if len(m.visible) != 4 || !strings.Contains(m.status, "imported 0 servers") {
		t.Fatal("repeat import duplicated servers")
	}
	if err := os.WriteFile(path, []byte("Host bad\n Port invalid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m = press(m, "i")
	if len(m.visible) != 4 || !strings.Contains(m.status, "import failed") {
		t.Fatal("failed import changed list")
	}
}
