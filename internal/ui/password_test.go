package ui

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"addresspad/internal/model"
	"addresspad/internal/secret"
	tea "github.com/charmbracelet/bubbletea"
)

type fakeSecrets struct {
	values    map[string]string
	setErr    error
	getErr    error
	deleteErr error
	sets      int
	deletes   int
}

func newFakeSecrets() *fakeSecrets {
	return &fakeSecrets{values: map[string]string{}}
}

func (f *fakeSecrets) Get(id string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	value, ok := f.values[id]
	if !ok {
		return "", secret.ErrNotFound
	}
	return value, nil
}

func (f *fakeSecrets) Set(id, password string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.sets++
	f.values[id] = password
	return nil
}

func (f *fakeSecrets) Delete(id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.values[id]; !ok {
		return secret.ErrNotFound
	}
	f.deletes++
	delete(f.values, id)
	return nil
}

func fillRequired(m Model, name, host string) Model {
	m.form.inputs[fieldName].SetValue(name)
	m.form.inputs[fieldHost].SetValue(host)
	return m
}

func TestFormSavesPasswordToKeychain(t *testing.T) {
	m := formModel(t)
	secrets := newFakeSecrets()
	m.secrets = secrets
	m = press(m, "a")
	m = fillRequired(m, "secret-host", "example.com")
	m.form.inputs[fieldPassword].SetValue("hunter2")
	m = formKey(m, tea.KeyCtrlS)
	if m.mode != modeList {
		t.Fatalf("save failed: %v %s", m.form.errors, m.form.saveError)
	}
	saved := m.visible[m.cursor]
	if !saved.PasswordSaved || secrets.values[saved.ID] != "hunter2" {
		t.Fatalf("saved = %+v secrets = %v", saved, secrets.values)
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "hunter2") || strings.Contains(string(data), "password:") {
		t.Fatalf("password leaked into YAML: %s", data)
	}
	if !strings.Contains(string(data), "password_saved: true") {
		t.Fatalf("metadata missing: %s", data)
	}
	if m.form.inputs[fieldPassword].Value() != "" {
		t.Fatal("password remained in the form")
	}
}

func TestFormKeepsAndRemovesPassword(t *testing.T) {
	m := formModel(t)
	secrets := newFakeSecrets()
	m.secrets = secrets
	saved := m.visible[0]
	saved.PasswordSaved = true
	if err := m.store.Update(saved.ID, saved); err != nil {
		t.Fatal(err)
	}
	secrets.values[saved.ID] = "old-secret"
	m.servers = sortedServers(m.store.Config().Servers)
	m.refresh(saved.ID)

	m = press(m, "e")
	m = formKey(m, tea.KeyCtrlS)
	if m.mode != modeList || secrets.values[saved.ID] != "old-secret" || !m.visible[m.cursor].PasswordSaved {
		t.Fatalf("keep failed: %+v %v", m.visible[m.cursor], secrets.values)
	}

	m = press(m, "e")
	m.form.setFocus(fieldSavePassword)
	m = press(m, " ")
	if m.form.savePassword {
		t.Fatal("space did not uncheck save password")
	}
	m = formKey(m, tea.KeyCtrlS)
	if m.mode != modeList || m.visible[m.cursor].PasswordSaved {
		t.Fatalf("remove failed: %+v", m.visible[m.cursor])
	}
	if _, err := secrets.Get(saved.ID); !errors.Is(err, secret.ErrNotFound) {
		t.Fatalf("secret still present: %v", err)
	}
}

func TestFormReplacesPassword(t *testing.T) {
	m := formModel(t)
	secrets := newFakeSecrets()
	m.secrets = secrets
	saved := m.visible[0]
	saved.PasswordSaved = true
	if err := m.store.Update(saved.ID, saved); err != nil {
		t.Fatal(err)
	}
	secrets.values[saved.ID] = "old"
	m.servers = sortedServers(m.store.Config().Servers)
	m.refresh(saved.ID)

	m = press(m, "e")
	m.form.inputs[fieldPassword].SetValue("new-secret")
	m = formKey(m, tea.KeyCtrlS)
	if m.mode != modeList || secrets.values[saved.ID] != "new-secret" {
		t.Fatalf("replace failed: %v %+v", secrets.values, m.form.errors)
	}
}

func TestFormRollsBackPasswordWhenSaveFails(t *testing.T) {
	m := formModel(t)
	secrets := newFakeSecrets()
	m.secrets = secrets
	saved := m.visible[0]
	saved.PasswordSaved = true
	if err := m.store.Update(saved.ID, saved); err != nil {
		t.Fatal(err)
	}
	secrets.values[saved.ID] = "old-secret"
	m.servers = sortedServers(m.store.Config().Servers)
	m.refresh(saved.ID)

	m = press(m, "e")
	m.form.inputs[fieldPassword].SetValue("new-secret")
	// Force backup failure without changing the original data file.
	if err := os.Remove(m.path + ".bak"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(m.path+".bak", 0700); err != nil {
		t.Fatal(err)
	}
	m = formKey(m, tea.KeyCtrlS)
	if m.mode != modeForm || !strings.Contains(m.form.saveError, "save failed") {
		t.Fatalf("save error = %q", m.form.saveError)
	}
	if secrets.values[saved.ID] != "old-secret" {
		t.Fatalf("password was not rolled back: %q", secrets.values[saved.ID])
	}
}

func TestFormPasswordErrorsRetainInput(t *testing.T) {
	t.Run("save unchecked", func(t *testing.T) {
		m := formModel(t)
		m.secrets = newFakeSecrets()
		m = press(m, "a")
		m = fillRequired(m, "secret-host", "example.com")
		m.form.inputs[fieldPassword].SetValue("secret")
		m.form.savePassword = false
		m = formKey(m, tea.KeyCtrlS)
		if m.mode != modeForm || m.form.focus != fieldSavePassword || m.form.errors[fieldSavePassword] == "" {
			t.Fatalf("validation = %v focus = %d", m.form.errors, m.form.focus)
		}
		if len(m.store.Config().Servers) != 3 {
			t.Fatal("validation saved a server")
		}
	})

	t.Run("keychain write", func(t *testing.T) {
		m := formModel(t)
		m.secrets = &fakeSecrets{setErr: errors.New("keychain locked")}
		m = press(m, "a")
		m = fillRequired(m, "secret-host", "example.com")
		m.form.inputs[fieldPassword].SetValue("secret")
		m = formKey(m, tea.KeyCtrlS)
		if m.mode != modeForm || !strings.Contains(m.form.saveError, "save password failed") || m.form.inputs[fieldPassword].Value() != "secret" {
			t.Fatalf("save error = %q", m.form.saveError)
		}
		if len(m.store.Config().Servers) != 3 {
			t.Fatal("failed keychain write saved a server")
		}
	})

	t.Run("keychain delete", func(t *testing.T) {
		m := formModel(t)
		secrets := &fakeSecrets{deleteErr: errors.New("keychain locked")}
		m.secrets = secrets
		saved := m.visible[0]
		saved.PasswordSaved = true
		if err := m.store.Update(saved.ID, saved); err != nil {
			t.Fatal(err)
		}
		m.servers = sortedServers(m.store.Config().Servers)
		m.refresh(saved.ID)
		m = press(m, "e")
		m.form.setFocus(fieldSavePassword)
		m = press(m, " ")
		m = formKey(m, tea.KeyCtrlS)
		if m.mode != modeForm || !strings.Contains(m.form.saveError, "remove password failed") {
			t.Fatalf("delete error = %q", m.form.saveError)
		}
		if !m.store.Config().Servers[0].PasswordSaved {
			t.Fatal("failed delete cleared metadata")
		}
	})
}

func TestDeleteServerRemovesPassword(t *testing.T) {
	m := formModel(t)
	secrets := newFakeSecrets()
	m.secrets = secrets
	saved := m.visible[0]
	saved.PasswordSaved = true
	if err := m.store.Update(saved.ID, saved); err != nil {
		t.Fatal(err)
	}
	secrets.values[saved.ID] = "secret"
	m.servers = sortedServers(m.store.Config().Servers)
	m.refresh(saved.ID)
	m = press(m, "d")
	m = press(m, "y")
	if m.mode != modeList {
		t.Fatal("delete failed")
	}
	if _, err := secrets.Get(saved.ID); !errors.Is(err, secret.ErrNotFound) {
		t.Fatalf("password not deleted: %v", err)
	}
}

func TestConnectConfiguresAskpass(t *testing.T) {
	m := testModel(t)
	m.secrets = newFakeSecrets()
	m.visible[0].PasswordSaved = true
	m.executable = func() (string, error) { return "/opt/apad", nil }
	var captured *exec.Cmd
	m.command = func(model.Server) (*exec.Cmd, error) {
		captured = exec.Command("ssh")
		return captured, nil
	}
	m = press(m, "enter")
	if m.mode != modeConnecting {
		t.Fatal("connection did not start")
	}
	values := envValues(captured.Env)
	if values["SSH_ASKPASS"] != "/opt/apad" || values["SSH_ASKPASS_REQUIRE"] != "force" || values["ADDRESSPAD_ASKPASS_SERVER"] != m.visible[0].ID {
		t.Fatalf("env = %v", values)
	}
}

func TestConnectFallsBackWhenAskpassUnavailable(t *testing.T) {
	m := testModel(t)
	m.secrets = newFakeSecrets()
	m.visible[0].PasswordSaved = true
	m.executable = func() (string, error) { return "", errors.New("no executable") }
	var captured *exec.Cmd
	m.command = func(model.Server) (*exec.Cmd, error) {
		captured = exec.Command("ssh")
		return captured, nil
	}
	m = press(m, "enter")
	if m.mode != modeConnecting || !strings.Contains(m.pendingWarning, "saved password unavailable") {
		t.Fatalf("warning = %q mode = %d", m.pendingWarning, m.mode)
	}
	if values := envValues(captured.Env); values["SSH_ASKPASS"] != "" {
		t.Fatalf("askpass configured: %v", values)
	}
}

func envValues(env []string) map[string]string {
	values := map[string]string{}
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func TestPasswordPlaceholder(t *testing.T) {
	m := New(nil, filepath.Join(t.TempDir(), "servers.yaml"), nil)
	server := model.Server{Name: "x", Host: "localhost", PasswordSaved: true}
	form := newForm(server, true, m.styles)
	if form.inputs[fieldPassword].Placeholder == "" || !form.savePassword {
		t.Fatalf("form = %+v", form.inputs[fieldPassword])
	}
}
