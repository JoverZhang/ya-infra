// Package host shares file and process operations used by the CLI and app hooks.
package host

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Decode(data []byte, out any) error {
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(out); err != nil {
		return errors.New("invalid YAML fields or types; inspect the encrypted input with sops")
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("configuration must contain one YAML document")
	}
	return nil
}

func Decrypt(path string, out any) error {
	data, err := Plaintext(path)
	if err != nil {
		return err
	}
	defer clear(data)
	return Decode(data, out)
}

func Plaintext(path string) ([]byte, error) {
	data, err := exec.Command("sops", "decrypt", "--output-type", "yaml", path).Output()
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt %s; check the file and SOPS key access", filepath.Base(path))
	}
	return data, nil
}

func XDG(variable, fallback string) string {
	value := os.Getenv(variable)
	if !filepath.IsAbs(value) {
		home, _ := os.UserHomeDir()
		value = filepath.Join(home, fallback)
	}
	return value
}

// WritePrivate replaces a file atomically and leaves identical content untouched.
func WritePrivate(path string, data []byte) error {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errors.New("private output must be a regular file")
	}
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
		return os.Chmod(path, 0600)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
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

func Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
