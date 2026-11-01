package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const siteDir = "/repo/site"

func main() {
	if err := build(); err != nil {
		// Keep a separate marker: git-sync clears its own error file after a fetch.
		if writeErr := os.WriteFile(siteDir+"/build-error", []byte("site build failed\n"), 0644); writeErr != nil {
			fmt.Fprintln(os.Stderr, "record build failure:", writeErr)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build() error {
	if err := os.MkdirAll(siteDir, 0755); err != nil {
		return fmt.Errorf("create publication directory: %w", err)
	}
	// Hooks run in a synced worktree. Never build into the directory Caddy serves.
	if err := os.RemoveAll("dist"); err != nil {
		return fmt.Errorf("remove stale build output: %w", err)
	}
	for _, args := range [][]string{{"install"}, {"run", "typecheck"}, {"run", "build"}} {
		cmd := exec.Command("npm", args...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("npm %s: %w", strings.Join(args, " "), err)
		}
	}
	index, err := os.Stat("dist/index.html")
	if err != nil {
		return fmt.Errorf("check dist/index.html: %w", err)
	}
	if !index.Mode().IsRegular() || index.Size() == 0 {
		return fmt.Errorf("dist/index.html must be a nonempty regular file")
	}
	release, err := os.MkdirTemp(siteDir, ".release-")
	if err != nil {
		return fmt.Errorf("stage build output: %w", err)
	}
	if err := os.Chmod(release, 0755); err != nil {
		return err
	}
	if err := os.CopyFS(release, os.DirFS("dist")); err != nil {
		return fmt.Errorf("copy build output: %w", err)
	}
	link := release + "-link"
	if err := os.Symlink(filepath.Base(release), link); err != nil {
		return fmt.Errorf("stage publication link: %w", err)
	}
	previous, err := os.Readlink(siteDir + "/current")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read previous publication: %w", err)
	}
	if err := os.Remove(siteDir + "/build-error"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear build failure: %w", err)
	}
	// Keep the previous release; remove abandoned staging directories on success.
	entries, err := os.ReadDir(siteDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".release-") && entry.Name() != filepath.Base(release) && entry.Name() != filepath.Base(link) && entry.Name() != previous {
			if err := os.RemoveAll(filepath.Join(siteDir, entry.Name())); err != nil {
				return fmt.Errorf("remove old publication: %w", err)
			}
		}
	}
	// Publication is the commit point: all fallible preparation is complete.
	if err := os.Rename(link, siteDir+"/current"); err != nil {
		return fmt.Errorf("publish build output: %w", err)
	}
	return nil
}
