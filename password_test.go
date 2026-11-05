package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"addresspad/internal/model"
	"addresspad/internal/secret"
	"addresspad/internal/store"
)

type cliSecrets struct {
	values    map[string]string
	setErr    error
	deleteErr error
}

func newCLISecrets() *cliSecrets { return &cliSecrets{values: map[string]string{}} }

func (f *cliSecrets) Get(id string) (string, error) {
	value, ok := f.values[id]
	if !ok {
		return "", secret.ErrNotFound
	}
	return value, nil
}

func (f *cliSecrets) Set(id, password string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.values[id] = password
	return nil
}

func (f *cliSecrets) Delete(id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.values[id]; !ok {
		return secret.ErrNotFound
	}
	delete(f.values, id)
	return nil
}

func cliEnv(env []string) map[string]string {
	values := map[string]string{}
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func passwordCLI(t *testing.T, path string, secrets secret.Store, input string) (cli, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, stderr bytes.Buffer
	return cli{
		path: path, in: strings.NewReader(input), out: &out, stderr: &stderr,
		secrets: secrets, executable: func() (string, error) { return "/opt/apad", nil },
	}, &out, &stderr
}

func TestPasswordCLI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.yaml")
	config := model.Config{}
	config.Normalize()
	config.Servers = append(config.Servers, model.Server{Name: "prod", Host: "example.com"})
	config.Servers[0].Normalize()
	if err := store.Save(path, config); err != nil {
		t.Fatal(err)
	}
	secrets := newCLISecrets()
	id := config.Servers[0].ID

	c, out, _ := passwordCLI(t, path, secrets, "s3cret\nignored\n")
	if code := c.password([]string{"prod"}); code != 0 || !strings.Contains(out.String(), "saved password for prod") {
		t.Fatalf("save: %d %s", code, out.String())
	}
	if secrets.values[id] != "s3cret" {
		t.Fatalf("secret = %q", secrets.values[id])
	}
	loaded, err := store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Servers[0].PasswordSaved {
		t.Fatal("metadata not saved")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "s3cret") {
		t.Fatal("password leaked into YAML")
	}

	c, out, _ = passwordCLI(t, path, secrets, "")
	if code := c.password([]string{"prod", "--delete"}); code != 0 || !strings.Contains(out.String(), "removed password for prod") {
		t.Fatalf("delete: %d %s", code, out.String())
	}
	if _, ok := secrets.values[id]; ok {
		t.Fatal("secret not deleted")
	}
	loaded, err = store.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Servers[0].PasswordSaved {
		t.Fatal("metadata not cleared")
	}
	c, _, _ = passwordCLI(t, path, secrets, "")
	if code := c.password([]string{"prod", "--delete"}); code != 0 {
		t.Fatal("repeat delete should be idempotent")
	}
}

func TestDirectConnectUsesSavedPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.yaml")
	config := model.Config{}
	config.Normalize()
	server := model.Server{Name: "prod", Host: "localhost", PasswordSaved: true}
	server.Normalize()
	config.Servers = append(config.Servers, server)
	if err := store.Save(path, config); err != nil {
		t.Fatal(err)
	}
	var captured *exec.Cmd
	var out, stderr bytes.Buffer
	c := cli{
		path: path, in: strings.NewReader(""), out: &out, stderr: &stderr,
		secrets: newCLISecrets(), executable: func() (string, error) { return "/opt/apad", nil },
		command: func(model.Server) (*exec.Cmd, error) {
			captured = exec.Command(os.Args[0], "-test.run=^TestDirectConnectProcess$")
			captured.Env = append(os.Environ(), "ADDRESSPAD_CLI_HELPER=1")
			return captured, nil
		},
	}
	if code := c.connect("prod"); code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	values := cliEnv(captured.Env)
	if values["SSH_ASKPASS"] != "/opt/apad" || values["SSH_ASKPASS_REQUIRE"] != "force" || values["ADDRESSPAD_ASKPASS_SERVER"] != server.ID {
		t.Fatalf("env = %v", values)
	}
}

func TestPasswordCLIRollsBackKeychain(t *testing.T) {
	setup := func(t *testing.T, passwordSaved bool) (string, string, *cliSecrets) {
		t.Helper()
		path := filepath.Join(t.TempDir(), "servers.yaml")
		config := model.Config{}
		config.Normalize()
		config.Servers = append(config.Servers, model.Server{Name: "prod", Host: "example.com", PasswordSaved: passwordSaved})
		config.Servers[0].Normalize()
		if err := store.Save(path, config); err != nil {
			t.Fatal(err)
		}
		secrets := newCLISecrets()
		secrets.values[config.Servers[0].ID] = "old-secret"
		// Force backup failure without changing the original data file.
		if err := os.RemoveAll(path + ".bak"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path+".bak", 0700); err != nil {
			t.Fatal(err)
		}
		return path, config.Servers[0].ID, secrets
	}

	t.Run("save", func(t *testing.T) {
		path, id, secrets := setup(t, false)
		c, _, _ := passwordCLI(t, path, secrets, "new-secret\n")
		if code := c.password([]string{"prod"}); code != 1 {
			t.Fatalf("code = %d", code)
		}
		if secrets.values[id] != "old-secret" {
			t.Fatalf("password was not rolled back: %q", secrets.values[id])
		}
	})

	t.Run("delete", func(t *testing.T) {
		path, id, secrets := setup(t, true)
		c, _, _ := passwordCLI(t, path, secrets, "")
		if code := c.password([]string{"prod", "--delete"}); code != 1 {
			t.Fatalf("code = %d", code)
		}
		if secrets.values[id] != "old-secret" {
			t.Fatalf("password was not restored: %q", secrets.values[id])
		}
	})
}

func TestPasswordCLIErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.yaml")
	config := model.Config{}
	config.Normalize()
	config.Servers = append(config.Servers, model.Server{Name: "prod", Host: "example.com"})
	config.Servers[0].Normalize()
	if err := store.Save(path, config); err != nil {
		t.Fatal(err)
	}
	id := config.Servers[0].ID

	t.Run("empty input", func(t *testing.T) {
		c, _, _ := passwordCLI(t, path, newCLISecrets(), "\n")
		if code := c.password([]string{"prod"}); code != 2 {
			t.Fatalf("code = %d", code)
		}
	})
	t.Run("carriage return input", func(t *testing.T) {
		c, _, _ := passwordCLI(t, path, newCLISecrets(), "bad\rline")
		if code := c.password([]string{"prod"}); code != 2 {
			t.Fatalf("code = %d", code)
		}
	})
	t.Run("keychain write", func(t *testing.T) {
		secrets := newCLISecrets()
		secrets.setErr = errors.New("keychain locked")
		c, _, _ := passwordCLI(t, path, secrets, "secret\n")
		if code := c.password([]string{"prod"}); code != 1 {
			t.Fatalf("code = %d", code)
		}
		loaded, err := store.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Servers[0].PasswordSaved {
			t.Fatal("failed write changed metadata")
		}
		if _, ok := secrets.values[id]; ok {
			t.Fatal("failed write stored a value")
		}
	})
	t.Run("keychain delete", func(t *testing.T) {
		secrets := newCLISecrets()
		secrets.values[id] = "old"
		secrets.deleteErr = errors.New("keychain locked")
		c, _, _ := passwordCLI(t, path, secrets, "")
		if code := c.password([]string{"prod", "--delete"}); code != 1 {
			t.Fatalf("code = %d", code)
		}
	})
	t.Run("usage", func(t *testing.T) {
		c, _, _ := passwordCLI(t, path, newCLISecrets(), "")
		if code := c.password(nil); code != 2 {
			t.Fatalf("code = %d", code)
		}
		if code := c.password([]string{"a", "b"}); code != 2 {
			t.Fatalf("code = %d", code)
		}
	})
	t.Run("unknown server", func(t *testing.T) {
		c, _, _ := passwordCLI(t, path, newCLISecrets(), "secret\n")
		if code := c.password([]string{"missing"}); code != 1 {
			t.Fatalf("code = %d", code)
		}
	})
}
