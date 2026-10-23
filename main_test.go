package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		code int
		text string
	}{
		{"version", []string{"--version"}, 0, "apad dev"},
		{"help", []string{"--help"}, 0, "Usage: apad"},
		{"short help", []string{"-h"}, 0, "Usage: apad"},
		{"help command", []string{"help"}, 0, "Usage: apad"},
		{"unknown command", []string{"missing"}, 2, "unknown command"},
		{"unknown flag", []string{"--missing"}, 2, "flag provided but not defined"},
		{"missing config value", []string{"--config"}, 2, "flag needs an argument"},
		{"invalid config command", []string{"config"}, 2, "unknown command"},
		{"pending TUI", nil, 1, "TUI is not implemented"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			if code := run(tc.args, &out, &stderr); code != tc.code {
				t.Fatalf("code = %d, want %d", code, tc.code)
			}
			if !strings.Contains(out.String()+stderr.String(), tc.text) {
				t.Fatalf("unexpected output: %s%s", &out, &stderr)
			}
		})
	}
}

func TestConfigPathDoesNotCreateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "servers.yaml")
	var out, stderr bytes.Buffer
	if code := run([]string{"--config", path, "config", "path"}, &out, &stderr); code != 0 {
		t.Fatalf("code = %d: %s", code, &stderr)
	}
	if strings.TrimSpace(out.String()) != path {
		t.Fatalf("path = %q", out.String())
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("created directory: %v", err)
	}
}
