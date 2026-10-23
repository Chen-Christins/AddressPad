package model

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalize(t *testing.T) {
	s := Server{Name: " 生产 ", Host: "example.com", IdentityFile: "~/.ssh/key", Tags: []string{" web ", "", "web", "数据库"}}
	s.Normalize()
	if s.Name != "生产" || s.Port != 22 || s.ID == "" || s.IdentityFile != "~/.ssh/key" {
		t.Fatalf("defaults: %+v", s)
	}
	if !reflect.DeepEqual(s.Tags, []string{"web", "数据库"}) || s.ExtraArgs == nil {
		t.Fatalf("arrays: %+v", s)
	}
	id := s.ID
	s.Normalize()
	if s.ID != id {
		t.Fatal("normalization changed stable id")
	}
}

func TestServerValidate(t *testing.T) {
	for _, tc := range []struct {
		name, host string
		port       int
		valid      bool
	}{
		{"valid", "example.com", 22, true},
		{"IPv6", "::1", 65535, true},
		{"", "localhost", 22, false},
		{"space", "host name", 22, false},
		{"unicode space", "host\u3000name", 22, false},
		{"option", "-oProxyCommand=bad", 22, false},
		{"empty", "", 22, false},
		{"low", "localhost", 0, false},
		{"negative", "localhost", -1, false},
		{"high", "localhost", 65536, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (Server{Name: tc.name, Host: tc.host, Port: tc.port}).Validate()
			if (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v, valid = %v", err, tc.valid)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config Config
		valid  bool
	}{
		{"empty", Config{}, true},
		{"future", Config{Version: 2}, false},
		{"negative", Config{Version: -1}, false},
		{"duplicate name", Config{Servers: []Server{{Name: "Prod", Host: "a"}, {Name: "prod", Host: "b"}}}, false},
		{"duplicate id", Config{Servers: []Server{{ID: "same", Name: "a", Host: "a"}, {ID: "same", Name: "b", Host: "b"}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.config.Normalize()
			if err := tc.config.Validate(); (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v", err)
			}
		})
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ input, want string }{
		{"~", home}, {"~/.ssh/key", filepath.Join(home, ".ssh", "key")},
		{"~someone/key", "~someone/key"}, {"relative/key", "relative/key"}, {"", ""},
	} {
		got, err := ExpandHome(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("ExpandHome(%q) = %q, %v", tc.input, got, err)
		}
	}
}
