package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"addresspad/internal/model"
	"addresspad/internal/sshcmd"
	"addresspad/internal/sshconfig"
	"addresspad/internal/store"
)

var commandHelp = map[string]string{
	"list": "Usage: apad [--config PATH] list [--json]\nList servers by favorite, last use and name.\n",
	"add": `Usage: apad [--config PATH] add --name NAME --host HOST [options]
Options: --user USER --port PORT --identity PATH --proxy-jump JUMP
         --group GROUP --tag TAG (repeatable) --note NOTE --favorite
         --extra-arg ARG (repeatable; use --extra-arg=-o for option values)
`,
	"import":  "Usage: apad [--config PATH] import [--ssh-config PATH] [--dry-run] [--overwrite]\nDefaults to ~/.ssh/config. Dry run never writes files.\n",
	"connect": "Usage: apad [--config PATH] connect NAME\nOr: apad [--config PATH] NAME\nCase-insensitive exact match, then unique substring match.\n",
	"config":  "Usage: apad [--config PATH] config path\nPrint path without creating files.\n",
}

type repeated []string

func (r *repeated) String() string         { return strings.Join(*r, ", ") }
func (r *repeated) Set(value string) error { *r = append(*r, value); return nil }

type cli struct {
	path        string
	in          io.Reader
	out, stderr io.Writer
	command     func(model.Server) (*exec.Cmd, error)
}

func (c cli) fail(code int, err error) int { fmt.Fprintln(c.stderr, err); return code }

func (c cli) flags(name string) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(c.stderr)
	f.Usage = func() { fmt.Fprint(c.stderr, commandHelp[name]) }
	return f
}

func parseFlags(f *flag.FlagSet, args []string) (int, bool) {
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, false
		}
		return 2, false
	}
	if f.NArg() != 0 {
		fmt.Fprintf(f.Output(), "unexpected arguments: %v\n", f.Args())
		return 2, false
	}
	return 0, true
}

func (c cli) list(args []string) int {
	f := c.flags("list")
	asJSON := f.Bool("json", false, "JSON output")
	if code, ok := parseFlags(f, args); !ok {
		return code
	}
	config, err := store.Read(c.path)
	if err != nil {
		return c.fail(1, err)
	}
	config.Servers = model.Sorted(config.Servers)
	if *asJSON {
		encoder := json.NewEncoder(c.out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(config); err != nil {
			return c.fail(1, err)
		}
		return 0
	}
	w := tabwriter.NewWriter(c.out, 0, 4, 2, ' ', 0)
	for _, s := range config.Servers {
		star := " "
		if s.Favorite {
			star = "★"
		}
		host := s.Host
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		if s.User != "" {
			host = s.User + "@" + host
		}
		group := ""
		if s.Group != "" {
			group = "[" + s.Group + "]"
		}
		tags := []string{}
		for _, tag := range s.Tags {
			tags = append(tags, "#"+tag)
		}
		if _, err := fmt.Fprintf(w, "%s %s\t%s:%d\t%s\t%s\t%s\n", star, safeText(s.Name), safeText(host), s.Port, safeText(group), safeText(strings.Join(tags, " ")), safeText(s.Note)); err != nil {
			return c.fail(1, err)
		}
	}
	if err := w.Flush(); err != nil {
		return c.fail(1, err)
	}
	return 0
}

func safeText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

func (c cli) add(args []string) int {
	f := c.flags("add")
	s := model.Server{}
	f.StringVar(&s.Name, "name", "", "unique name")
	f.StringVar(&s.Host, "host", "", "host")
	f.StringVar(&s.User, "user", "", "user")
	f.IntVar(&s.Port, "port", 22, "port")
	f.StringVar(&s.IdentityFile, "identity", "", "identity path")
	f.StringVar(&s.ProxyJump, "proxy-jump", "", "jump host")
	f.StringVar(&s.Group, "group", "", "group")
	f.StringVar(&s.Note, "note", "", "note")
	f.BoolVar(&s.Favorite, "favorite", false, "favorite")
	var tags, extra repeated
	f.Var(&tags, "tag", "tag (repeatable)")
	f.Var(&extra, "extra-arg", "SSH argument (repeatable)")
	if code, ok := parseFlags(f, args); !ok {
		return code
	}
	// Explicit port zero is invalid, not an omitted/default value.
	if s.Port < 1 || s.Port > 65535 {
		return c.fail(2, fmt.Errorf("port must be between 1 and 65535"))
	}
	s.Tags, s.ExtraArgs = []string(tags), []string(extra)
	s.Normalize()
	if err := s.Validate(); err != nil {
		return c.fail(2, err)
	}
	config, err := store.Read(c.path)
	if err != nil {
		return c.fail(1, err)
	}
	config.Servers = append(config.Servers, s)
	if err := config.Validate(); err != nil {
		return c.fail(2, err)
	}
	if err := store.Save(c.path, config); err != nil {
		return c.fail(1, err)
	}
	fmt.Fprintln(c.out, "added "+s.Name)
	return 0
}

func (c cli) importConfig(args []string) int {
	f := c.flags("import")
	source := f.String("ssh-config", "~/.ssh/config", "SSH configuration path")
	dry := f.Bool("dry-run", false, "preview only")
	overwrite := f.Bool("overwrite", false, "update existing connection fields")
	if code, ok := parseFlags(f, args); !ok {
		return code
	}
	result, err := sshconfig.Parse(*source)
	if err != nil {
		return c.fail(1, err)
	}
	config, err := store.Read(c.path)
	if err != nil {
		return c.fail(1, err)
	}
	merged, actions, err := sshconfig.Merge(config, result.Servers, *overwrite)
	if err != nil {
		return c.fail(1, err)
	}
	changed := false
	for _, action := range actions {
		if !strings.HasPrefix(action, "skip ") {
			changed = true
		}
	}
	if !*dry && changed {
		if err := store.Save(c.path, merged); err != nil {
			return c.fail(1, err)
		}
	}
	fmt.Fprintln(c.out, "parsed "+safeText(*source))
	for _, include := range result.Includes {
		fmt.Fprintln(c.out, "include "+safeText(include))
	}
	for _, action := range actions {
		if *dry {
			action = "would " + action
		}
		fmt.Fprintln(c.out, safeText(action))
	}
	for _, skip := range result.Skipped {
		fmt.Fprintln(c.out, "skip "+safeText(skip))
	}
	return 0
}

func (c cli) connect(name string) int {
	config, err := store.Read(c.path)
	if err != nil {
		return c.fail(1, err)
	}
	s, err := resolveServer(config.Servers, name)
	if err != nil {
		return c.fail(1, err)
	}
	cmd, err := c.command(s)
	if err != nil {
		return c.fail(1, err)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.in, c.out, c.stderr
	err = cmd.Run()
	code := sshcmd.ExitCode(err)
	if code < 0 {
		return c.fail(1, err)
	}
	if code != 0 {
		return code
	}
	for i := range config.Servers {
		if config.Servers[i].ID == s.ID {
			config.Servers[i].LastUsed = time.Now()
			break
		}
	}
	// SSH exit status must remain intact even when persisting last_used fails.
	if err := store.Save(c.path, config); err != nil {
		fmt.Fprintln(c.stderr, "warning: save last_used failed:", err)
	}
	return 0
}

func resolveServer(servers []model.Server, name string) (model.Server, error) {
	for _, s := range servers {
		if strings.EqualFold(s.Name, name) {
			return s, nil
		}
	}
	matches := []model.Server{}
	for _, s := range model.Sorted(servers) {
		if strings.Contains(strings.ToLower(s.Name), strings.ToLower(name)) {
			matches = append(matches, s)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		names := []string{}
		for _, s := range matches {
			names = append(names, safeText(s.Name))
		}
		return model.Server{}, fmt.Errorf("ambiguous name %q; candidates: %s", name, strings.Join(names, ", "))
	}
	suggestions := append([]model.Server{}, servers...)
	sort.SliceStable(suggestions, func(i, j int) bool {
		a, b := distance(strings.ToLower(name), strings.ToLower(suggestions[i].Name)), distance(strings.ToLower(name), strings.ToLower(suggestions[j].Name))
		if a != b {
			return a < b
		}
		return suggestions[i].Name < suggestions[j].Name
	})
	names := []string{}
	for _, s := range suggestions[:min(3, len(suggestions))] {
		names = append(names, safeText(s.Name))
	}
	return model.Server{}, fmt.Errorf("no server matches %q; closest names: %s", name, strings.Join(names, ", "))
}

func distance(a, b string) int {
	x, y := []rune(a), []rune(b)
	row := make([]int, len(y)+1)
	for j := range row {
		row[j] = j
	}
	for i, r := range x {
		previous := row[0]
		row[0] = i + 1
		for j, s := range y {
			old := row[j+1]
			cost := 0
			if r != s {
				cost = 1
			}
			row[j+1] = min(row[j+1]+1, row[j]+1, previous+cost)
			previous = old
		}
	}
	return row[len(y)]
}
