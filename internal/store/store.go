// Package store implements validated YAML persistence and transactional edits.
package store

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"addresspad/internal/model"
	"gopkg.in/yaml.v3"
)

func ResolvePath(explicit string) (string, error) {
	path := explicit
	if path == "" {
		path = os.Getenv("ADDRESSPAD_CONFIG")
	}
	if path == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("configuration directory: %w", err)
		}
		path = filepath.Join(dir, "addresspad", "servers.yaml")
	}
	path, err := model.ExpandHome(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

func decode(data []byte) (model.Config, error) {
	var c model.Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&c); err != nil {
		return c, fmt.Errorf("parse YAML: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return c, errors.New("expected a single YAML document")
		}
		return c, fmt.Errorf("parse YAML: %w", err)
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		return c, err
	}
	return c, nil
}

// Load initializes missing files, but never repairs or overwrites invalid files.
func Load(path string) (model.Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		c := model.Config{}
		c.Normalize()
		err = Save(path, c)
		return c, err
	}
	if err != nil {
		return model.Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	c, err := decode(data)
	if err != nil {
		return model.Config{}, fmt.Errorf("load %s: %w", path, err)
	}
	return c, nil
}

// Read loads without creating files, for read-only commands and import previews.
func Read(path string) (model.Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		c := model.Config{}
		c.Normalize()
		return c, nil
	}
	if err != nil {
		return model.Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	c, err := decode(data)
	if err != nil {
		return model.Config{}, fmt.Errorf("load %s: %w", path, err)
	}
	return c, nil
}

// writeAtomic writes and syncs a private temporary file before replacing path.
// No fallible operations follow rename, so an error leaves the target unchanged.
func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}

// Save refuses to overwrite corrupt or future-version data, even if called
// independently of Load. Normalization takes place on a detached copy.
func Save(path string, config model.Config) error {
	c := config.Clone()
	c.Normalize()
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode configuration: %w", err)
	}
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s before save: %w", path, err)
	}
	exists := err == nil
	if exists {
		if _, err := decode(old); err != nil {
			return fmt.Errorf("refusing to overwrite invalid configuration %s: %w", path, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if exists {
		if err := writeAtomic(path+".bak", old); err != nil {
			return fmt.Errorf("backup %s: %w", path, err)
		}
	}
	if err := writeAtomic(path, data); err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	return nil
}
