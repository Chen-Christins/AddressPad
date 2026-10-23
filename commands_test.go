package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"addresspad/internal/model"
	"addresspad/internal/store"
)

func invoke(t *testing.T, path string, args ...string) (int, string, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	code := run(append([]string{"--config", path}, args...), &out, &stderr)
	return code, out.String(), stderr.String()
}

func TestAddListCLI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "servers.yaml")
	for _, args := range [][]string{{"list"}, {"list", "--json"}, {"add", "--help"}, {"help", "import"}} {
		if code, _, err := invoke(t, path, args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, err)
		}
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("read/help created config directory")
	}
	if code, out, err := invoke(t, path, "add", "--name", "生产", "--host", "::1", "--user", "deploy", "--port", "2222", "--identity", "~/.ssh/my key", "--proxy-jump", "jump", "--group", "prod", "--tag", " web ", "--tag", "web", "--note", "中文备注", "--favorite", "--extra-arg=-A"); code != 0 || !strings.Contains(out, "added 生产") {
		t.Fatalf("add: %d %s %s", code, out, err)
	}
	if code, _, err := invoke(t, path, "add", "--name", "second", "--host", "example.com"); code != 0 {
		t.Fatal(err)
	}
	code, out, stderr := invoke(t, path, "list", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("list: %d %s", code, stderr)
	}
	var config model.Config
	if err := json.Unmarshal([]byte(out), &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Servers) != 2 || config.Servers[0].Name != "生产" || config.Servers[0].Port != 2222 || config.Servers[0].Tags[0] != "web" {
		t.Fatalf("JSON = %s", out)
	}
	if strings.Contains(out, `"tags": null`) || strings.Contains(out, `"extra_args": null`) {
		t.Fatal("JSON contains null arrays")
	}
	if code, out, err := invoke(t, path, "list"); code != 0 || !strings.Contains(out, "deploy@[::1]:2222") || !strings.Contains(out, "中文备注") {
		t.Fatalf("text list: %s %s", out, err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "--name", "SECOND", "--host", "host"}, {"add", "--name", "invalid", "--host", "host", "--port", "0"},
		{"add", "--name", "invalid", "--host", "host", "--port", "65536"}, {"add", "--name", "invalid", "--host", "-bad"},
		{"add", "--name", "invalid"}, {"add", "--host", "host"}, {"list", "extra"}, {"add", "--bad"}, {"help", "missing"}, {"connect"},
	} {
		if code, _, stderr := invoke(t, path, args...); code != 2 {
			t.Fatalf("%v: got %d %s", args, code, stderr)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("failed command modified file: %v", args)
		}
	}
}

func TestImportCLI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "servers.yaml")
	source := filepath.Join(dir, "ssh_config")
	writeSource := func(text string) {
		t.Helper()
		if err := os.WriteFile(source, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeSource("Host prod alias\n HostName example.com\n User deploy\nHost *\n User ignored\n")
	if code, out, err := invoke(t, path, "import", "--ssh-config", source, "--dry-run"); code != 0 || !strings.Contains(out, "would import prod") {
		t.Fatalf("preview: %d %s %s", code, out, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("dry-run created directory")
	}
	if code, _, err := invoke(t, path, "import", "--ssh-config", source); code != 0 {
		t.Fatal(err)
	}
	c, err := store.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Servers) != 2 {
		t.Fatal("multi-alias import failed")
	}
	c.Servers[0].Favorite = true
	c.Servers[0].Note = "keep"
	id := c.Servers[0].ID
	if err := store.Save(path, c); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeSource("Host prod\n HostName changed.example\n")
	if code, out, err := invoke(t, path, "import", "--ssh-config", source); code != 0 || !strings.Contains(out, "skip prod") {
		t.Fatalf("skip %d %s %s", code, out, err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("skip-only import modified file")
	}
	if code, _, err := invoke(t, path, "import", "--ssh-config", source, "--overwrite"); code != 0 {
		t.Fatal(err)
	}
	c, err = store.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Servers[0].Host != "changed.example" || c.Servers[0].ID != id || !c.Servers[0].Favorite || c.Servers[0].Note != "keep" {
		t.Fatal("overwrite lost metadata")
	}
	before, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeSource("Host valid\nHost invalid\n Port bad\n")
	if code, _, _ := invoke(t, path, "import", "--ssh-config", source); code != 1 {
		t.Fatal("invalid import succeeded")
	}
	after, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed parse partially imported")
	}
}

func TestResolveServer(t *testing.T) {
	servers := []model.Server{{Name: "prod-web"}, {Name: "prod-db"}, {Name: "staging"}, {Name: "数据库"}}
	for _, tc := range []struct {
		name, want string
		invalid    bool
	}{{"PROD-WEB", "prod-web", false}, {"web", "prod-web", false}, {"prod", "", true}, {"prdo-web", "", true}, {"数据", "数据库", false}} {
		s, err := resolveServer(servers, tc.name)
		if (err != nil) != tc.invalid || (!tc.invalid && s.Name != tc.want) {
			t.Fatalf("%s: %+v %v", tc.name, s, err)
		}
		if tc.name == "prod" && !strings.Contains(err.Error(), "candidates") {
			t.Fatal("missing ambiguity candidates")
		}
		if tc.name == "prdo-web" && !strings.Contains(err.Error(), "closest names: prod-web") {
			t.Fatal("missing closest names")
		}
	}
}

func TestDirectConnectProcess(t *testing.T) {
	if os.Getenv("ADDRESSPAD_CLI_HELPER") == "1" {
		fmt.Fprintln(os.Stdout, "fake ssh output")
		fmt.Fprintln(os.Stderr, "fake ssh error output")
		if os.Getenv("ADDRESSPAD_CLI_FAIL") == "1" {
			os.Exit(23)
		}
		os.Exit(0)
	}
	path := filepath.Join(t.TempDir(), "servers.yaml")
	if code, _, err := invoke(t, path, "add", "--name", "prod", "--host", "localhost"); code != 0 {
		t.Fatal(err)
	}
	for _, fail := range []bool{true, false} {
		var out, stderr bytes.Buffer
		c := cli{path: path, in: strings.NewReader(""), out: &out, stderr: &stderr, command: func(model.Server) (*exec.Cmd, error) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestDirectConnectProcess$")
			cmd.Env = append(os.Environ(), "ADDRESSPAD_CLI_HELPER=1")
			if fail {
				cmd.Env = append(cmd.Env, "ADDRESSPAD_CLI_FAIL=1")
			}
			return cmd, nil
		}}
		code := c.connect("PROD")
		want := 0
		if fail {
			want = 23
		}
		if code != want {
			t.Fatalf("code %d want %d", code, want)
		}
		if !strings.Contains(out.String(), "fake ssh output") || !strings.Contains(stderr.String(), "fake ssh error output") {
			t.Fatal("streams not handed to child")
		}
		config, err := store.Read(path)
		if err != nil {
			t.Fatal(err)
		}
		if config.Servers[0].LastUsed.IsZero() != fail {
			t.Fatal("incorrect last_used update")
		}
	}
}

func TestDirectConnectDispatchAndHelp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "servers.yaml")
	for _, args := range [][]string{{"unknown-name"}, {"connect", "unknown-name"}} {
		code, _, err := invoke(t, path, args...)
		if code != 1 || !strings.Contains(err, "no server matches") {
			t.Fatalf("%v: %d %s", args, code, err)
		}
	}
	for _, command := range []string{"add", "list", "import", "connect"} {
		code, out, err := invoke(t, path, command, "--help")
		if code != 0 || !strings.Contains(out+err, "Usage:") {
			t.Fatalf("%s help: %d %s %s", command, code, out, err)
		}
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("queries/help created configuration")
	}
}
