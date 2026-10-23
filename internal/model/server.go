// Package model defines configuration data independently of storage and UI.
package model

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const CurrentVersion = 1

type Server struct {
	ID           string    `yaml:"id" json:"id"`
	Name         string    `yaml:"name" json:"name"`
	Host         string    `yaml:"host" json:"host"`
	User         string    `yaml:"user,omitempty" json:"user"`
	Port         int       `yaml:"port" json:"port"`
	IdentityFile string    `yaml:"identity_file,omitempty" json:"identity_file"`
	ProxyJump    string    `yaml:"proxy_jump,omitempty" json:"proxy_jump"`
	Group        string    `yaml:"group,omitempty" json:"group"`
	Tags         []string  `yaml:"tags,omitempty" json:"tags"`
	Favorite     bool      `yaml:"favorite" json:"favorite"`
	Note         string    `yaml:"note,omitempty" json:"note"`
	ExtraArgs    []string  `yaml:"extra_args,omitempty" json:"extra_args"`
	LastUsed     time.Time `yaml:"last_used,omitempty" json:"last_used"`
}

type Config struct {
	Version int      `yaml:"version" json:"version"`
	Servers []Server `yaml:"servers" json:"servers"`
}

// Normalize supplies missing defaults without expanding stored identity paths.
func (s *Server) Normalize() {
	s.Name = strings.TrimSpace(s.Name)
	if s.ID == "" {
		b := make([]byte, 16)
		// crypto/rand.Read never returns an error on supported Go versions.
		_, _ = rand.Read(b)
		s.ID = hex.EncodeToString(b)
	}
	if s.Port == 0 {
		s.Port = 22
	}
	tags := make([]string, 0, len(s.Tags))
	seen := make(map[string]bool)
	for _, tag := range s.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" && !seen[tag] {
			tags = append(tags, tag)
			seen[tag] = true
		}
	}
	s.Tags = tags
	if s.ExtraArgs == nil {
		s.ExtraArgs = []string{}
	}
}

func (s Server) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("name must not be empty")
	}
	if s.Host == "" || strings.HasPrefix(s.Host, "-") || strings.ContainsFunc(s.Host, unicode.IsSpace) {
		return fmt.Errorf("host must not be empty, start with '-' or contain whitespace")
	}
	if s.Port < 1 || s.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func (c *Config) Normalize() {
	if c.Version == 0 {
		c.Version = CurrentVersion
	}
	if c.Servers == nil {
		c.Servers = []Server{}
	}
	for i := range c.Servers {
		c.Servers[i].Normalize()
	}
}

func (c Config) Validate() error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("unsupported configuration version %d (supported: %d)", c.Version, CurrentVersion)
	}
	names, ids := map[string]bool{}, map[string]bool{}
	for i, s := range c.Servers {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("server %d (%q): %w", i+1, s.Name, err)
		}
		name := strings.ToLower(strings.TrimSpace(s.Name))
		if names[name] {
			return fmt.Errorf("duplicate server name %q", s.Name)
		}
		if s.ID == "" || ids[s.ID] {
			return fmt.Errorf("missing or duplicate server id %q", s.ID)
		}
		names[name], ids[s.ID] = true, true
	}
	return nil
}

// Clone prevents failed edits from changing the caller's in-memory data.
func (c Config) Clone() Config {
	copy := c
	copy.Servers = append([]Server{}, c.Servers...)
	for i := range copy.Servers {
		copy.Servers[i].Tags = append([]string{}, c.Servers[i].Tags...)
		copy.Servers[i].ExtraArgs = append([]string{}, c.Servers[i].ExtraArgs...)
	}
	return copy
}

// ExpandHome expands only ~ and ~/ (or ~\ on Windows), not ~otheruser.
func ExpandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") && !(os.PathSeparator == '\\' && strings.HasPrefix(path, "~\\")) {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
