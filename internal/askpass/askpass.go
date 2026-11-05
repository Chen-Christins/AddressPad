// Package askpass answers OpenSSH prompts from the system keychain.
//
// OpenSSH runs this helper when SSH_ASKPASS is set. Passwords are written only
// to stdout; prompts and diagnostics go to stderr so they remain visible to the
// user. Prompts that are not simple password requests fall back to a terminal,
// preserving host-key confirmation, key passphrases and OTP input.
package askpass

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"addresspad/internal/secret"
	"golang.org/x/term"
)

const (
	ServerEnv = "ADDRESSPAD_ASKPASS_SERVER"
	PromptEnv = "SSH_ASKPASS_PROMPT"
)

// Prompter lets tests replace terminal input.
type Prompter interface {
	Prompt(prompt string, hidden bool) (string, error)
}

type Terminal struct {
	In  *os.File
	Out io.Writer
}

func NewTerminal() Prompter { return Terminal{In: os.Stdin, Out: os.Stderr} }

func (t Terminal) Prompt(prompt string, hidden bool) (string, error) {
	if t.In == nil || !term.IsTerminal(int(t.In.Fd())) {
		return "", errors.New("no terminal available for SSH prompt")
	}
	if _, err := io.WriteString(t.Out, promptWithSpace(prompt)); err != nil {
		return "", err
	}
	if hidden {
		b, err := term.ReadPassword(int(t.In.Fd()))
		_, _ = fmt.Fprintln(t.Out)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	line, err := bufio.NewReader(t.In).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func promptWithSpace(prompt string) string {
	if prompt == "" || strings.HasSuffix(prompt, " ") {
		return prompt
	}
	return prompt + " "
}

// Run returns the process exit status expected by OpenSSH. A non-zero status
// makes ssh treat the prompt as unanswered.
func Run(args []string, store secret.Store, getenv func(string) string, stdout, stderr io.Writer, terminal Prompter) int {
	prompt := strings.Join(args, " ")
	if getenv(PromptEnv) == "none" {
		return 0
	}
	if strings.TrimSpace(prompt) == "" {
		fmt.Fprintln(stderr, "AddressPad: ssh provided an empty prompt")
		return 1
	}
	lower := strings.ToLower(prompt)
	if getenv(PromptEnv) == "confirm" || isConfirmation(lower) {
		return interact(prompt, false, stdout, stderr, terminal)
	}
	if isPasswordPrompt(lower) {
		if password, ok := storedPassword(store, getenv(ServerEnv), stderr); ok {
			if _, err := fmt.Fprintln(stdout, password); err != nil {
				return 1
			}
			return 0
		}
		return interact(prompt, true, stdout, stderr, terminal)
	}
	return interact(prompt, isSensitive(lower), stdout, stderr, terminal)
}

func storedPassword(store secret.Store, serverID string, stderr io.Writer) (string, bool) {
	if store == nil || serverID == "" {
		return "", false
	}
	password, err := store.Get(serverID)
	if err != nil {
		if !errors.Is(err, secret.ErrNotFound) {
			fmt.Fprintf(stderr, "AddressPad: %v\n", err)
		}
		return "", false
	}
	if secret.ValidatePassword(password) != nil {
		fmt.Fprintln(stderr, "AddressPad: saved password is not valid for SSH askpass")
		return "", false
	}
	return password, true
}

func interact(prompt string, hidden bool, stdout, stderr io.Writer, terminal Prompter) int {
	if terminal == nil {
		fmt.Fprintln(stderr, "AddressPad: no terminal available for SSH prompt")
		return 1
	}
	answer, err := terminal.Prompt(prompt, hidden)
	if err != nil {
		fmt.Fprintf(stderr, "AddressPad: %v\n", err)
		return 1
	}
	if _, err := fmt.Fprintln(stdout, answer); err != nil {
		return 1
	}
	return 0
}

func isConfirmation(prompt string) bool {
	return strings.Contains(prompt, "are you sure you want to continue connecting") ||
		strings.Contains(prompt, "(yes/no") ||
		strings.Contains(prompt, "yes/no/[fingerprint]")
}

func isPasswordPrompt(prompt string) bool {
	return strings.Contains(prompt, "password") && !strings.Contains(prompt, "passphrase") && !isChallenge(prompt)
}

func isChallenge(prompt string) bool {
	for _, word := range []string{"passcode", "verification code", "one-time", "otp"} {
		if strings.Contains(prompt, word) {
			return true
		}
	}
	return false
}

func isSensitive(prompt string) bool {
	return isChallenge(prompt) || strings.Contains(prompt, "passphrase") ||
		strings.Contains(prompt, "password") || strings.Contains(prompt, "pin")
}
