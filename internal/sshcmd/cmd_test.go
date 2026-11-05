package sshcmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"addresspad/internal/model"
)

func TestBuild(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		server  model.Server
		want    []string
		invalid bool
	}{
		{"default", model.Server{Name: "x", Host: "example.com"}, []string{"--", "example.com"}, false},
		{"full", model.Server{Name: "x", Host: "::1", User: "deploy", Port: 2222, IdentityFile: "~/.ssh/my key", ProxyJump: "ops@jump:22", ExtraArgs: []string{"-o", "ForwardAgent=yes"}}, []string{"-p", "2222", "-i", filepath.Join(home, ".ssh", "my key"), "-J", "ops@jump:22", "-o", "ForwardAgent=yes", "--", "deploy@::1"}, false},
		{"literal", model.Server{Name: "x", Host: "host;$(command)", ExtraArgs: []string{"-o", "ProxyCommand=echo 'hello'"}}, []string{"-o", "ProxyCommand=echo 'hello'", "--", "host;$(command)"}, false},
		{"option host", model.Server{Name: "x", Host: "-bad"}, nil, true},
		{"space host", model.Server{Name: "x", Host: "bad host"}, nil, true},
		{"bad port", model.Server{Name: "x", Host: "localhost", Port: 65536}, nil, true},
		{"nul", model.Server{Name: "x", Host: "localhost", ExtraArgs: []string{"bad\x00arg"}}, nil, true},
		{"newline", model.Server{Name: "x", Host: "localhost", User: "bad\nuser"}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Build(tc.server)
			if (err != nil) != tc.invalid {
				t.Fatalf("Build() error = %v", err)
			}
			if !tc.invalid && !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	args := []string{"-i", "my key's $HOME; `echo bad`", "--", "user@host"}
	if got := Format(args, "linux"); got != "ssh '-i' 'my key'\"'\"'s $HOME; `echo bad`' '--' 'user@host'" {
		t.Fatalf("POSIX = %s", got)
	}
	if got := Format(args, "windows"); got != "& ssh '-i' 'my key''s $HOME; `echo bad`' '--' 'user@host'" {
		t.Fatalf("PowerShell = %s", got)
	}
}

func TestEnableAskpass(t *testing.T) {
	cmd := exec.Command("ssh")
	cmd.Env = []string{
		"HOME=/tmp/home",
		"SSH_ASKPASS=/old/helper",
		"ssh_askpass_require=prefer",
		AskpassServerEnv + "=old-server",
	}
	if err := EnableAskpass(cmd, "/opt/apad", "server-1"); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	counts := map[string]int{}
	for _, entry := range cmd.Env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[strings.ToUpper(key)] = value
			counts[strings.ToUpper(key)]++
		}
	}
	if values["SSH_ASKPASS"] != "/opt/apad" || values["SSH_ASKPASS_REQUIRE"] != "force" || values[AskpassServerEnv] != "server-1" {
		t.Fatalf("env = %v", values)
	}
	for _, key := range []string{"SSH_ASKPASS", "SSH_ASKPASS_REQUIRE", AskpassServerEnv} {
		if counts[key] != 1 {
			t.Fatalf("%s appears %d times in %v", key, counts[key], cmd.Env)
		}
	}
	if values["HOME"] != "/tmp/home" {
		t.Fatal("existing environment was lost")
	}
	if strings.Contains(strings.Join(cmd.Env, "\n"), "secret") {
		t.Fatal("environment contains a password")
	}
	if err := EnableAskpass(nil, "/opt/apad", "id"); err == nil {
		t.Fatal("nil command accepted")
	}
	if err := EnableAskpass(cmd, "", "id"); err == nil {
		t.Fatal("empty executable accepted")
	}
}

func TestCommandMissingSSH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := Command(model.Server{Name: "x", Host: "localhost"})
	if err == nil || !strings.Contains(err.Error(), "install ssh") {
		t.Fatalf("error = %v", err)
	}
}

func TestCommandAndExitCode(t *testing.T) {
	if os.Getenv("ADDRESSPAD_SSHCMD_HELPER") == "1" {
		os.Exit(23)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestCommandAndExitCode$")
	cmd.Env = append(os.Environ(), "ADDRESSPAD_SSHCMD_HELPER=1")
	if code := ExitCode(cmd.Run()); code != 23 {
		t.Fatalf("exit code = %d", code)
	}
	if ExitCode(nil) != 0 || ExitCode(errors.New("cannot start")) != -1 {
		t.Fatal("incorrect exit code")
	}
}

func TestClipboardFallback(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		var tried []string
		err := copyWith("中文 command", platform, func(tool clipboardTool, text string) error {
			if text != "中文 command" {
				t.Fatal("text changed")
			}
			tried = append(tried, tool.name)
			return errors.New("not available")
		})
		if err == nil || len(tried) != len(clipboardTools(platform)) {
			t.Fatalf("fallback failed: %v %v", tried, err)
		}
	}
	var tried []string
	err := copyWith("text", "linux", func(tool clipboardTool, _ string) error {
		tried = append(tried, tool.name)
		if tool.name == "xclip" {
			return nil
		}
		return errors.New("unavailable")
	})
	if err != nil || !reflect.DeepEqual(tried, []string{"wl-copy", "xclip"}) {
		t.Fatalf("fallback order %v, %v", tried, err)
	}
}
