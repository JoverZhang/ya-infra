package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
	"gopkg.in/yaml.v3"
)

type runtimeConfig struct {
	Image   string `yaml:"image"`
	Volumes []struct {
		Type, Source, Target string
		ReadOnly             bool `yaml:"read_only"`
	} `yaml:"volumes"`
}

func main() {
	if err := update(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
func update() error {
	if len(os.Args) != 2 || os.Args[1] != "update" {
		return errors.New("usage: cpe update (run just cpe-update from the repository root)")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root := filepath.Clean(filepath.Join(cwd, "../../.."))
	if err = os.Setenv("PATH", filepath.Join(host.XDG("XDG_DATA_HOME", ".local/share"), "ya-infra/bin")+string(os.PathListSeparator)+os.Getenv("PATH")); err != nil {
		return err
	}
	source, err := filepath.EvalSymlinks(filepath.Join(root, ".sources/cpe"))
	if err != nil {
		return errors.New("create the CPE source worktree at .sources/cpe first")
	}
	top, err := capture(source, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	actual, err := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
	if err != nil || actual != source {
		return errors.New(".sources/cpe must be the root of its own Git worktree")
	}
	status, err := capture(source, "git", "status", "--porcelain")
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(status))) != 0 {
		return errors.New("CPE source has local changes; commit or discard them first")
	}
	// Validate the merged model, including private Compose overrides, before stopping.
	data, err := capture(root, "just", "compose", "cpe", "config")
	if err != nil {
		return err
	}
	var config struct {
		Services struct {
			Daemon runtimeConfig `yaml:"daemon"`
			Web    runtimeConfig `yaml:"web"`
		} `yaml:"services"`
	}
	if yaml.Unmarshal(data, &config) != nil {
		return errors.New("invalid effective CPE Compose configuration; inspect it privately")
	}
	for _, service := range []runtimeConfig{config.Services.Daemon, config.Services.Web} {
		if service.Image != "cpe:local" {
			return errors.New("both CPE apps must select cpe:local")
		}
		matches := 0
		for _, mount := range service.Volumes {
			if mount.Target == "/app/worker_scripts" {
				if mount.Type != "bind" || filepath.Clean(mount.Source) != filepath.Join(source, "worker_scripts") || !mount.ReadOnly {
					return errors.New("both CPE apps must mount .sources/cpe/worker_scripts readonly")
				}
				matches++
			}
		}
		if matches != 1 {
			return errors.New("both CPE apps need exactly one worker_scripts mount")
		}
	}
	if err = execute(source, "git", "fetch", "origin"); err != nil {
		return err
	}
	if err = execute(root, "just", "compose", "cpe", "stop", "daemon", "web"); err != nil {
		return err
	}
	if err = execute(source, "git", "switch", "--detach", "origin/master"); err != nil {
		return err
	}
	if err = execute(source, "just", "image", "build", "cpe:local"); err != nil {
		return fmt.Errorf("applications remain stopped; fix the build error and rerun just cpe-update: %w", err)
	}
	if err = execute(root, "just", "compose", "cpe", "up", "-d", "--no-deps", "--force-recreate", "daemon", "web"); err != nil {
		return err
	}
	if err = execute(root, "just", "compose", "cpe", "exec", "-T", "daemon", "cpe-client", "doctor"); err != nil {
		return err
	}
	return execute(root, "just", "compose", "cpe", "ps")
}
func capture(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w (inspect configuration privately)", args[0], err)
	}
	return data, nil
}
func execute(dir string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(args, " "), err)
	}
	return nil
}
