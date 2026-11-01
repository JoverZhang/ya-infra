package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"
)

const sourceLink = "/repo/current"
const workspace = "/home/node/site"

func main() {
	if err := serve(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		// Avoid a tight restart loop when installation or startup fails.
		time.Sleep(5 * time.Second)
		os.Exit(1)
	}
}

func serve() error {
	var source string
	for {
		var err error
		source, err = filepath.EvalSymlinks(sourceLink)
		if err == nil {
			break
		}
		fmt.Fprintln(os.Stderr, "waiting for a synced checkout")
		time.Sleep(2 * time.Second)
	}
	// Native init/restart must end the entire container, including npm's children.
	// This also interrupts installation if a newer checkout arrives.
	go func() {
		for range time.NewTicker(time.Second).C {
			current, err := filepath.EvalSymlinks(sourceLink)
			if err != nil || current != source {
				fmt.Fprintln(os.Stderr, "checkout changed or disappeared; restarting development server")
				os.Exit(1)
			}
		}
	}()
	if err := os.RemoveAll(workspace); err != nil {
		return fmt.Errorf("clear development workspace: %w", err)
	}
	if err := os.CopyFS(workspace, sourceTree{os.DirFS(source)}); err != nil {
		return fmt.Errorf("copy development source: %w", err)
	}
	install := exec.Command("npm", "install")
	install.Dir, install.Stdout, install.Stderr = workspace, os.Stdout, os.Stderr
	if err := install.Run(); err != nil {
		return fmt.Errorf("install development dependencies: %w", err)
	}
	base := os.Getenv("VITE_BASE_PATH")
	if base == "" {
		base = "/"
	}
	dev := exec.Command("npm", "run", "dev", "--", "--host", "0.0.0.0", "--port", "8080", "--strictPort", "--base", base)
	dev.Dir, dev.Stdout, dev.Stderr = workspace, os.Stdout, os.Stderr
	if err := dev.Run(); err != nil {
		return fmt.Errorf("run development server: %w", err)
	}
	return fmt.Errorf("development server exited")
}

// Copy source without Git metadata, old static output or dependencies from another runtime.
type sourceTree struct{ fs.FS }

func (s sourceTree) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(s.FS, name)
	if err != nil {
		return nil, err
	}
	if name == "." {
		entries = slices.DeleteFunc(entries, func(entry fs.DirEntry) bool {
			return entry.Name() == ".git" || entry.Name() == "dist" || entry.Name() == "node_modules"
		})
	}
	return entries, nil
}
