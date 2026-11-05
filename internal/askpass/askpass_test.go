package askpass

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"addresspad/internal/secret"
)

type fakeStore struct {
	password string
	err      error
}

func (f fakeStore) Get(string) (string, error) { return f.password, f.err }
func (f fakeStore) Set(string, string) error   { return nil }
func (f fakeStore) Delete(string) error        { return nil }

type fakePrompter struct {
	answer string
	err    error
	called bool
	prompt string
	hidden bool
}

func (f *fakePrompter) Prompt(prompt string, hidden bool) (string, error) {
	f.called, f.prompt, f.hidden = true, prompt, hidden
	return f.answer, f.err
}

func run(t *testing.T, prompt string, store secret.Store, env map[string]string, tty Prompter) (int, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	getenv := func(key string) string { return env[key] }
	code := Run([]string{prompt}, store, getenv, &out, &stderr, tty)
	return code, out.String()
}

func TestStoredPasswordPrompt(t *testing.T) {
	tty := &fakePrompter{answer: "typed"}
	code, out := run(t, "deploy@example.com's password: ", fakeStore{password: "stored secret"}, map[string]string{ServerEnv: "server-1"}, tty)
	if code != 0 || out != "stored secret\n" {
		t.Fatalf("code=%d out=%q", code, out)
	}
	if tty.called {
		t.Fatal("terminal was used for a stored password")
	}
}

func TestPasswordFallbacks(t *testing.T) {
	tests := []struct {
		name  string
		store secret.Store
		env   map[string]string
	}{
		{"missing secret", fakeStore{err: secret.ErrNotFound}, map[string]string{ServerEnv: "server-1"}},
		{"keychain error", fakeStore{err: errors.New("keychain locked")}, map[string]string{ServerEnv: "server-1"}},
		{"missing server id", fakeStore{password: "stored"}, nil},
		{"invalid stored secret", fakeStore{password: "line\nbreak"}, map[string]string{ServerEnv: "server-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tty := &fakePrompter{answer: "typed"}
			code, out := run(t, "Password:", tc.store, tc.env, tty)
			if code != 0 || out != "typed\n" || !tty.called || !tty.hidden {
				t.Fatalf("code=%d out=%q tty=%+v", code, out, tty)
			}
		})
	}
}

func TestInteractivePrompts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prompt string
		env    map[string]string
		hidden bool
		answer string
	}{
		{"host key", "Are you sure you want to continue connecting (yes/no/[fingerprint])? ", nil, false, "yes"},
		{"permission", "Allow use of key? (yes/no) ", map[string]string{PromptEnv: "confirm"}, false, "yes"},
		{"passphrase", "Enter passphrase for key '/home/me/.ssh/id_ed25519': ", nil, true, "secret"},
		{"otp", "Verification code: ", nil, true, "123456"},
		{"one-time password", "One-time password: ", nil, true, "123456"},
		{"custom", "Enter response: ", nil, false, "value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tty := &fakePrompter{answer: tc.answer}
			env := map[string]string{ServerEnv: "server-1"}
			for key, value := range tc.env {
				env[key] = value
			}
			code, out := run(t, tc.prompt, fakeStore{password: "stored"}, env, tty)
			if code != 0 || out != tc.answer+"\n" || !tty.called || tty.hidden != tc.hidden {
				t.Fatalf("code=%d out=%q tty=%+v", code, out, tty)
			}
		})
	}
}

func TestNotificationsAndErrors(t *testing.T) {
	code, out := run(t, "anything", fakeStore{}, map[string]string{PromptEnv: "none"}, &fakePrompter{})
	if code != 0 || out != "" {
		t.Fatalf("notification: code=%d out=%q", code, out)
	}
	tty := &fakePrompter{err: errors.New("no terminal")}
	code, _ = run(t, "Password:", fakeStore{err: secret.ErrNotFound}, nil, tty)
	if code != 1 {
		t.Fatalf("error code = %d", code)
	}
	code, _ = run(t, "", fakeStore{}, nil, tty)
	if code != 1 {
		t.Fatalf("empty prompt code = %d", code)
	}
}

func TestPromptFormatting(t *testing.T) {
	if got := promptWithSpace("Password:"); got != "Password: " {
		t.Fatalf("got %q", got)
	}
	if got := promptWithSpace("Password: "); got != "Password: " {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(promptWithSpace("Password:"), "\n") {
		t.Fatal("added newline")
	}
}
