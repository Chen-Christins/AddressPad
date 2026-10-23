package sshconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"addresspad/internal/model"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestParseFixture(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path := filepath.Join(home, ".ssh", "config")
	write(t, filepath.Join(home, ".ssh", "conf.d", "a.conf"), "Host included\n HostName=10.0.0.2\n Port 2200\n")
	write(t, filepath.Join(home, ".ssh", "conf.d", "b.conf"), "Host second\n User ops\n")
	write(t, path, `# literal aliases, quotes and inline comments
Host foo bar *.wild !excluded
 HostName example.com
 User deploy
 Port 2222
 Port 9999
 IdentityFile "~/.ssh/my key" # keep tilde
 ProxyJump ops@jump:22
 ForwardAgent yes
Host *
 User ignored
Match exec "never execute anything"
 Include nonexistent.conf
 User ignored
Host plain
Include conf.d/*.conf
Host chinese
 HostName 数据.example
`)
	r, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, s := range r.Servers {
		got = append(got, s.Name)
	}
	if !reflect.DeepEqual(got, []string{"foo", "bar", "plain", "included", "second", "chinese"}) {
		t.Fatalf("aliases = %v", got)
	}
	for _, s := range r.Servers[:2] {
		if s.Host != "example.com" || s.User != "deploy" || s.Port != 2222 || s.IdentityFile != "~/.ssh/my key" || s.ProxyJump != "ops@jump:22" {
			t.Fatalf("mapped = %+v", s)
		}
	}
	if r.Servers[2].Host != "plain" || r.Servers[2].User != "" || r.Servers[2].Port != 22 {
		t.Fatal("wildcard or Match options leaked")
	}
	if r.Servers[3].Port != 2200 || len(r.Includes) != 2 || len(r.Skipped) != 4 {
		t.Fatalf("result = %+v", r)
	}
}

func TestParseFailures(t *testing.T) {
	for _, text := range []string{"Host", "Host a\n Port bad", "Host a\n Port 0", "Host a\n Port 65536", "Host a\n HostName -bad", "Host a\n User a b", "Host 'unclosed"} {
		t.Run(text, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config")
			write(t, path, text)
			r, err := Parse(path)
			if err == nil || len(r.Servers) != 0 {
				t.Fatalf("accepted invalid config: %+v %v", r, err)
			}
		})
	}
	if _, err := Parse(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing source accepted")
	}
}

func TestIncludesCycleDepthAndContext(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	write(t, a, fmt.Sprintf("Include %q\n", filepath.ToSlash(b)))
	write(t, b, fmt.Sprintf("Include %q\n", filepath.ToSlash(a)))
	if _, err := Parse(a); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle: %v", err)
	}
	for i := 0; i <= MaxDepth+1; i++ {
		text := "Host end\n"
		if i <= MaxDepth {
			text = fmt.Sprintf("Include %q\n", filepath.ToSlash(filepath.Join(dir, fmt.Sprint(i+1))))
		}
		write(t, filepath.Join(dir, fmt.Sprint(i)), text)
	}
	if _, err := Parse(filepath.Join(dir, "0")); err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("depth: %v", err)
	}
	write(t, b, " User included-user\n")
	write(t, a, fmt.Sprintf("Host context\n Include %q\n HostName context.example\n", filepath.ToSlash(b)))
	r, err := Parse(a)
	if err != nil || len(r.Servers) != 1 || r.Servers[0].User != "included-user" || r.Servers[0].Host != "context.example" {
		t.Fatalf("Include context: %+v %v", r, err)
	}
	// Reusing an include is not a recursion cycle.
	write(t, a, fmt.Sprintf("Host a\n Include %q\nHost b\n Include %q\n", filepath.ToSlash(b), filepath.ToSlash(b)))
	if _, err := Parse(a); err != nil {
		t.Fatalf("repeated nonrecursive include: %v", err)
	}
}

func TestSplit(t *testing.T) {
	for _, tc := range []struct {
		line string
		want []string
	}{
		{`HostName=example.com`, []string{"HostName", "example.com"}},
		{`HostName = example.com`, []string{"HostName", "example.com"}},
		{`HostName =example.com`, []string{"HostName", "example.com"}},
		{`IdentityFile "C:\Users\name\my key"`, []string{"IdentityFile", `C:\Users\name\my key`}},
		{`IdentityFile '/key#name' # comment`, []string{"IdentityFile", "/key#name"}},
		{`User deploy #comment`, []string{"User", "deploy"}},
	} {
		got, err := split(tc.line)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("split(%q) = %q, %v", tc.line, got, err)
		}
	}
}

func TestMerge(t *testing.T) {
	old := model.Server{ID: "stable", Name: "Prod", Host: "old", User: "old", Port: 22, Favorite: true, Note: "中文", Group: "prod", Tags: []string{"tag"}, ExtraArgs: []string{"-A"}, LastUsed: time.Now()}
	c := model.Config{Version: 1, Servers: []model.Server{old}}
	imports := []model.Server{{Name: "prod", Host: "new", User: "new", Port: 2222, IdentityFile: "key", ProxyJump: "jump"}, {Name: "second", Host: "second"}}
	for _, overwrite := range []bool{false, true} {
		got, actions, err := Merge(c, imports, overwrite)
		if err != nil || len(got.Servers) != 2 || len(actions) != 2 {
			t.Fatalf("merge: %+v %v", got, err)
		}
		s := got.Servers[0]
		if overwrite {
			if s.Host != "new" || s.User != "new" || s.Port != 2222 || s.IdentityFile != "key" || s.ProxyJump != "jump" {
				t.Fatal("connection fields not updated")
			}
		} else if s.Host != "old" {
			t.Fatal("skip changed existing entry")
		}
		if s.ID != old.ID || s.Name != old.Name || s.Note != old.Note || s.Favorite != old.Favorite || s.Group != old.Group || !s.LastUsed.Equal(old.LastUsed) || !reflect.DeepEqual(s.Tags, old.Tags) || !reflect.DeepEqual(s.ExtraArgs, old.ExtraArgs) {
			t.Fatal("metadata lost")
		}
	}
	if c.Servers[0].Host != "old" {
		t.Fatal("Merge mutated caller")
	}
}
