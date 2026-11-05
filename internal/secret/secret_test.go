package secret

import (
	"errors"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

func TestKeyringRoundTrip(t *testing.T) {
	keyring.MockInit()
	store := Default()
	if _, err := store.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
	if err := store.Set("server-1", "p@ss word"); err != nil {
		t.Fatal(err)
	}
	password, err := store.Get("server-1")
	if err != nil || password != "p@ss word" {
		t.Fatalf("get = %q, %v", password, err)
	}
	if err := store.Delete("server-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("server-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete = %v", err)
	}
	if err := store.Delete("server-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v", err)
	}
}

func TestKeyringErrorsAndValidation(t *testing.T) {
	keyring.MockInitWithError(errors.New("keychain locked"))
	if _, err := (Keyring{}).Get("id"); err == nil || !strings.Contains(err.Error(), "keychain locked") {
		t.Fatalf("get error = %v", err)
	}
	if err := (Keyring{}).Set("id", "secret"); err == nil || !strings.Contains(err.Error(), "keychain locked") {
		t.Fatalf("set error = %v", err)
	}
	if err := (Keyring{}).Delete("id"); err == nil || !strings.Contains(err.Error(), "keychain locked") {
		t.Fatalf("delete error = %v", err)
	}
	for _, password := range []string{"", "with\nnewline", "with\rcarriage", "with\x00nul"} {
		if err := ValidatePassword(password); err == nil {
			t.Fatalf("accepted invalid password %q", password)
		}
	}
	if err := ValidatePassword("correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
}
