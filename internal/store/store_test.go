package store

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"addresspad/internal/model"
)

func testPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "nested", "servers.yaml")
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRoundTripBackupAndPermissions(t *testing.T) {
	path := testPath(t)
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Version != 1 || c.Servers == nil {
		t.Fatalf("empty config = %+v", c)
	}
	original := readFile(t, path)
	s := model.Server{Name: "生产", Host: "example.com", Note: "中文备注", IdentityFile: "~/.ssh/key", Tags: []string{" web ", "web"}}
	s.Normalize()
	c.Servers = append(c.Servers, s)
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c, loaded) {
		t.Fatalf("round trip:\n%+v\n%+v", c, loaded)
	}
	if !bytes.Equal(original, readFile(t, path+".bak")) {
		t.Fatal("backup differs from original")
	}
	if runtime.GOOS != "windows" {
		for _, file := range []string{path, path + ".bak"} {
			info, err := os.Stat(file)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0600 {
				t.Fatalf("%s mode = %o", file, info.Mode().Perm())
			}
		}
		info, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0700 {
			t.Fatalf("directory mode = %o", info.Mode().Perm())
		}
	}
	files, err := filepath.Glob(path + ".tmp.*")
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary files: %v, %v", files, err)
	}
}

func TestCorruptFilesAreNeverOverwritten(t *testing.T) {
	for _, content := range []string{
		"version: [", "version: 2\nservers: []\n", "servers: wrong\n",
		"servers:\n  - name: x\n    host: a\n    port: invalid\n",
		"version: 1\n---\nversion: 1\n", "", "servers:\n  - name: x\n    host: -bad\n",
	} {
		t.Run(content, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "servers.yaml")
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("Load accepted corrupt configuration")
			}
			if err := Save(path, model.Config{}); err == nil {
				t.Fatal("Save overwrote corrupt configuration")
			}
			if string(readFile(t, path)) != content {
				t.Fatal("file changed")
			}
			if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
				t.Fatalf("backup created: %v", err)
			}
		})
	}
}

func TestCRUD(t *testing.T) {
	s, err := Open(testPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Add(model.Server{Name: "Prod", Host: "a"}); err != nil {
		t.Fatal(err)
	}
	id := s.Config().Servers[0].ID
	before := s.Config()
	if err := s.Add(model.Server{Name: "prod", Host: "b"}); err == nil {
		t.Fatal("duplicate accepted")
	}
	if !reflect.DeepEqual(before, s.Config()) {
		t.Fatal("failed add mutated memory")
	}
	if err := s.ToggleFavorite(id); err != nil {
		t.Fatal(err)
	}
	if !s.Config().Servers[0].Favorite {
		t.Fatal("favorite not toggled")
	}
	server := s.Config().Servers[0]
	server.Name = "Renamed"
	server.ID = "replacement"
	if err := s.Update(id, server); err != nil {
		t.Fatal(err)
	}
	if got := s.Config().Servers[0]; got.ID != id || got.Name != "Renamed" {
		t.Fatalf("updated = %+v", got)
	}
	if err := s.Delete("missing"); err == nil {
		t.Fatal("missing delete succeeded")
	}
	if err := s.Update("missing", server); err == nil {
		t.Fatal("missing update succeeded")
	}
	if err := s.ToggleFavorite("missing"); err == nil {
		t.Fatal("missing toggle succeeded")
	}
	if err := s.Delete(id); err != nil {
		t.Fatal(err)
	}
	if len(s.Config().Servers) != 0 {
		t.Fatal("delete failed")
	}
}

func TestFailedSaveKeepsMemoryAndFile(t *testing.T) {
	path := testPath(t)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Add(model.Server{Name: "a", Host: "a", Tags: []string{"one"}}); err != nil {
		t.Fatal(err)
	}
	before := s.Config()
	original := readFile(t, path)
	// A directory at the backup path reliably fails even when running as root.
	if err := os.Remove(path + ".bak"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".bak", 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.ToggleFavorite(before.Servers[0].ID); err == nil {
		t.Fatal("expected backup failure")
	}
	if !reflect.DeepEqual(before, s.Config()) || !bytes.Equal(original, readFile(t, path)) {
		t.Fatal("failed save changed state")
	}
	copy := s.Config()
	copy.Servers[0].Tags[0] = "modified"
	if s.Config().Servers[0].Tags[0] != "one" {
		t.Fatal("Config exposes backing slices")
	}
}

func TestResolvePath(t *testing.T) {
	env := filepath.Join(t.TempDir(), "env.yaml")
	t.Setenv("ADDRESSPAD_CONFIG", env)
	if got, err := ResolvePath(""); err != nil || got != env {
		t.Fatalf("env = %q, %v", got, err)
	}
	explicit := filepath.Join(t.TempDir(), "explicit.yaml")
	if got, err := ResolvePath(explicit); err != nil || got != explicit {
		t.Fatalf("explicit = %q, %v", got, err)
	}
	t.Setenv("ADDRESSPAD_CONFIG", "")
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ResolvePath(""); err != nil || got != filepath.Join(dir, "addresspad", "servers.yaml") {
		t.Fatalf("default = %q, %v", got, err)
	}
}

func TestDefaultsUnknownFieldsAndConcurrentReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.yaml")
	content := "unknown: ignored\nservers:\n  - name: manual\n    host: localhost\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Servers[0].ID == "" || c.Servers[0].Port != 22 {
		t.Fatal("missing defaults")
	}
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(readFile(t, path)), "unknown") {
		t.Fatal("unknown fields persisted")
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			got, err := Load(path)
			if err != nil {
				t.Error(err)
				return
			}
			if !reflect.DeepEqual(c, got) {
				t.Error("concurrent read differs")
			}
		})
	}
	wg.Wait()
}
