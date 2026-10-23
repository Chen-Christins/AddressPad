package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadNeverCreatesOrRepairs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "servers.yaml")
	c, err := Read(path)
	if err != nil || c.Version != 1 || c.Servers == nil {
		t.Fatalf("Read = %+v, %v", c, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("Read created directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("version: 1\nservers: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("broken: ["), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("Read accepted corrupt config")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "broken: [" {
		t.Fatal("Read modified corrupt config")
	}
	if _, err := Read(filepath.Dir(path)); err == nil {
		t.Fatal("Read accepted directory")
	}
}
