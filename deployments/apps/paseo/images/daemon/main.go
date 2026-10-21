// This binary runs only in the Pod's init container.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	fresh := flag.Bool("new", false, "initialize explicitly new volumes")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(1)
	}
	if err := initialize("/home/paseo", "/workspace", *fresh); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func initialize(home, workspace string, fresh bool) error {
	for _, root := range []string{home, workspace} {
		if !fresh {
			if _, err := os.Stat(filepath.Join(root, ".ya-infra-restored")); err != nil {
				if _, err = os.Stat(filepath.Join(root, ".ya-infra-initialized")); err != nil {
					return errors.New("volume has no verified restore or initialization marker; restore data before startup")
				}
			}
		}
	}
	for _, dir := range []string{home, workspace, filepath.Join(home, ".codex"), filepath.Join(home, ".paseo")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		if err := os.Chown(dir, 1000, 1000); err != nil {
			return err
		}
	}
	if err := os.Chmod(home, 0700); err != nil {
		return err
	}
	if err := filepath.WalkDir(filepath.Join(home, ".codex"), func(path string, item os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(path, 1000, 1000)
	}); err != nil {
		return err
	}
	config := filepath.Join(home, ".codex/config.toml")
	if _, err := os.Stat(config); os.IsNotExist(err) {
		data, err := os.ReadFile("/opt/infra/codex.toml")
		if err != nil {
			return err
		}
		if err = os.WriteFile(config, data, 0600); err != nil {
			return err
		}
		if err = os.Chown(config, 1000, 1000); err != nil {
			return err
		}
	}
	if fresh {
		for _, root := range []string{home, workspace} {
			if err := os.WriteFile(filepath.Join(root, ".ya-infra-initialized"), []byte("new\n"), 0600); err != nil {
				return err
			}
		}
	}
	return nil
}
