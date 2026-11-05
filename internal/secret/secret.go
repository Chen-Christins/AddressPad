// Package secret stores per-server passwords in the operating system keychain.
package secret

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const service = "addresspad"

// ErrNotFound reports that no password is stored for the requested server.
var ErrNotFound = errors.New("password not found in system keychain")

// Store hides the platform keychain implementation from the UI and CLI.
type Store interface {
	Get(serverID string) (string, error)
	Set(serverID, password string) error
	Delete(serverID string) error
}

type Keyring struct{}

func Default() Store { return Keyring{} }

func account(serverID string) string { return "server:" + serverID }

func (Keyring) Get(serverID string) (string, error) {
	if serverID == "" {
		return "", ErrNotFound
	}
	password, err := keyring.Get(service, account(serverID))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read system keychain: %w", err)
	}
	if password == "" {
		return "", ErrNotFound
	}
	return password, nil
}

func (Keyring) Set(serverID, password string) error {
	if serverID == "" {
		return errors.New("server id must not be empty")
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	if err := keyring.Set(service, account(serverID), password); err != nil {
		return fmt.Errorf("write system keychain: %w", err)
	}
	return nil
}

func (Keyring) Delete(serverID string) error {
	if serverID == "" {
		return ErrNotFound
	}
	err := keyring.Delete(service, account(serverID))
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete from system keychain: %w", err)
	}
	return nil
}

// ValidatePassword rejects values that cannot round-trip through OpenSSH's
// line-based askpass protocol.
func ValidatePassword(password string) error {
	if password == "" {
		return errors.New("password must not be empty")
	}
	if strings.ContainsAny(password, "\x00\r\n") {
		return errors.New("password must not contain NUL or newlines")
	}
	return nil
}
