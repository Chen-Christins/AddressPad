// Package sshconfig imports a deliberately limited subset of OpenSSH config.
// Wildcard Host and Match blocks are skipped, not evaluated.
package sshconfig

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"addresspad/internal/model"
)

const MaxDepth = 8

type Result struct {
	Servers  []model.Server
	Skipped  []string
	Includes []string
}

type parser struct {
	result  Result
	active  map[string]bool
	aliases []string
	options map[string]string
	base    string
	inMatch bool
}

func Parse(path string) (Result, error) {
	path, err := model.ExpandHome(path)
	if err != nil {
		return Result{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Result{}, err
	}
	p := parser{active: map[string]bool{}, base: filepath.Join(home, ".ssh")}
	if err := p.file(path, 0); err != nil {
		return Result{}, err
	}
	if err := p.flush(); err != nil {
		return Result{}, err
	}
	// OpenSSH's first value wins, including repeated Host sections.
	merged := []model.Server{}
	indices := map[string]int{}
	for _, s := range p.result.Servers {
		key := strings.ToLower(s.Name)
		if _, exists := indices[key]; exists {
			p.result.Skipped = append(p.result.Skipped, s.Name+": repeated alias (first entry retained)")
			continue
		}
		indices[key] = len(merged)
		merged = append(merged, s)
	}
	p.result.Servers = merged
	return p.result, nil
}

func (p *parser) flush() error {
	for _, alias := range p.aliases {
		s := model.Server{Name: alias, Host: alias, User: p.options["user"], IdentityFile: p.options["identityfile"], ProxyJump: p.options["proxyjump"]}
		if host := p.options["hostname"]; host != "" {
			s.Host = strings.ReplaceAll(host, "%h", alias)
		}
		if port, ok := p.options["port"]; ok {
			value, err := strconv.Atoi(port)
			if err != nil || value < 1 || value > 65535 {
				return fmt.Errorf("Host %s: invalid port %q", alias, port)
			}
			s.Port = value
		}
		s.Normalize()
		if err := s.Validate(); err != nil {
			return fmt.Errorf("Host %s: %w", alias, err)
		}
		p.result.Servers = append(p.result.Servers, s)
	}
	p.aliases = nil
	p.options = map[string]string{}
	return nil
}

func (p *parser) file(path string, depth int) error {
	if depth > MaxDepth {
		return fmt.Errorf("Include depth exceeds %d: %s", MaxDepth, path)
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("read SSH config %s: %w", path, err)
	}
	real, err = filepath.Abs(real)
	if err != nil {
		return err
	}
	key := real
	if runtime.GOOS == "windows" {
		key = strings.ToLower(real)
	}
	if p.active[key] {
		return fmt.Errorf("Include cycle: %s", path)
	}
	p.active[key] = true
	defer delete(p.active, key)
	f, err := os.Open(real)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		if err := p.line(scanner.Text(), depth); err != nil {
			return fmt.Errorf("%s:%d: %w", real, line, err)
		}
	}
	return scanner.Err()
}

func (p *parser) line(line string, depth int) error {
	words, err := split(line)
	if err != nil {
		return err
	}
	if len(words) == 0 {
		return nil
	}
	keyword := strings.ToLower(words[0])
	values := words[1:]
	if len(values) == 0 {
		return fmt.Errorf("missing value for %s", words[0])
	}
	switch keyword {
	case "host":
		p.inMatch = false
		if err := p.flush(); err != nil {
			return err
		}
		for _, alias := range values {
			if strings.ContainsAny(alias, "*?![") {
				p.result.Skipped = append(p.result.Skipped, alias+": wildcard/negated host")
				continue
			}
			p.aliases = append(p.aliases, alias)
		}
	case "match":
		if err := p.flush(); err != nil {
			return err
		}
		p.result.Skipped = append(p.result.Skipped, "Match "+strings.Join(values, " ")+": conditional block")
		p.inMatch = true
	case "include":
		if p.inMatch {
			return nil
		}
		for _, pattern := range values {
			pattern, err = model.ExpandHome(pattern)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(p.base, pattern)
			}
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("Include %q: %w", pattern, err)
			}
			// Unmatched Include patterns are optional, as in OpenSSH.
			for _, match := range matches {
				p.result.Includes = append(p.result.Includes, fmt.Sprintf("%s (depth %d)", match, depth+1))
				if err := p.file(match, depth+1); err != nil {
					return err
				}
			}
		}
	case "hostname", "user", "port", "identityfile", "proxyjump":
		if len(values) != 1 {
			return fmt.Errorf("%s expects one value (quote paths containing spaces)", words[0])
		}
		if len(p.aliases) == 0 {
			return nil
		}
		if _, exists := p.options[keyword]; !exists {
			p.options[keyword] = values[0]
		}
	}
	return nil
}

// split handles Key=value, quotes and inline comments. Backslashes in Windows
// paths are preserved unless escaping whitespace, a quote, '#' or backslash.
func split(line string) ([]string, error) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "#") {
		return []string{}, nil
	}
	// '=' separates only the keyword from its value, never a value itself.
	for i, r := range line {
		if unicode.IsSpace(r) {
			rest := strings.TrimSpace(line[i:])
			if strings.HasPrefix(rest, "=") {
				line = line[:i] + " " + strings.TrimSpace(rest[1:])
			}
			break
		}
		if r == '=' {
			line = line[:i] + " " + line[i+1:]
			break
		}
	}
	words := []string{}
	var word strings.Builder
	var quote rune
	started := false
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' && i+1 < len(runes) {
			next := runes[i+1]
			if next == '\\' || next == '"' || next == '\'' || next == '#' || unicode.IsSpace(next) {
				word.WriteRune(next)
				started = true
				i++
				continue
			}
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
			continue
		}
		if r == '#' {
			break
		}
		if r == '"' || r == '\'' {
			quote = r
			started = true
			continue
		}
		if unicode.IsSpace(r) {
			if started {
				words = append(words, word.String())
				word.Reset()
				started = false
			}
			continue
		}
		word.WriteRune(r)
		started = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	if started {
		words = append(words, word.String())
	}
	if len(words) > 1 && words[1] == "=" {
		words = append(words[:1], words[2:]...)
	}
	return words, nil
}
