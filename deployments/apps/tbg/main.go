package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
	"gopkg.in/yaml.v3"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" {
		fmt.Println("Usage: tbg setup | verify [--url URL]\nSetup reconciles the host client file. Verify checks the local protocol without sending messages.")
		return nil
	}
	switch args[0] {
	case "setup":
		if len(args) != 1 {
			return errors.New("setup takes no arguments")
		}
		c, err := host.Load()
		if err != nil {
			return err
		}
		data, err := host.Plaintext(filepath.Join(c.Source, "apps/tbg/host/client.sops.yaml"))
		if err != nil {
			return err
		}
		defer clear(data)
		var client struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
		}
		if err = host.Decode(data, &client); err != nil {
			return err
		}
		if client.Host != "127.0.0.1" || client.Port < 1024 || client.Port > 65535 {
			return errors.New("host client must use an unprivileged loopback endpoint")
		}
		if _, err = exec.LookPath("tbg"); err != nil {
			return err
		}
		stateDir := filepath.Join(host.XDG("XDG_STATE_HOME", ".local/state"), "ya-infra", "tbg")
		if err = os.MkdirAll(stateDir, 0700); err != nil {
			return err
		}
		lock, err := os.OpenFile(filepath.Join(stateDir, "setup.lock"), os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return err
		}
		defer lock.Close()
		if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			return errors.New("another TBG setup holds the lock")
		}
		return installClient(filepath.Join(host.XDG("XDG_CONFIG_HOME", ".config"), "tbg/client.yaml"), data, filepath.Join(stateDir, "client.json"))
	case "verify":
		flags := flag.NewFlagSet("verify", flag.ContinueOnError)
		url := flags.String("url", "http://127.0.0.1:18473/v1/whoami", "local protocol endpoint")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("verify takes no positional arguments")
		}
		return host.Probe(*url, 405)
	default:
		return errors.New("unsupported action; use --help")
	}
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func installClient(target string, data []byte, statePath string) error {
	old, err := os.ReadFile(target)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	var state struct {
		SHA256 string `json:"sha256"`
	}
	saved, err := os.ReadFile(statePath)
	if err == nil {
		if err := json.Unmarshal(saved, &state); err != nil {
			return fmt.Errorf("decode host client ownership state %q: %w", statePath, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	equal := bytes.Equal(old, data)
	if exists && !equal {
		var a, b any
		if yaml.Unmarshal(old, &a) == nil && yaml.Unmarshal(data, &b) == nil {
			equal = reflect.DeepEqual(a, b)
		}
	}
	if exists && !equal && (state.SHA256 == "" || state.SHA256 != digest(old)) {
		return errors.New("client file has unowned changes; reconcile it explicitly before setup")
	}
	if equal {
		data = old
	}
	if err = host.WritePrivate(target, data); err != nil {
		return err
	}
	state.SHA256 = digest(data)
	encoded, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return host.WritePrivate(statePath, encoded)
}
