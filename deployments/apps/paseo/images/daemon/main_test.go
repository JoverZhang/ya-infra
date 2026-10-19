package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMissingRestoreFailsBeforeInitialization(t *testing.T) {
	home, workspace := t.TempDir(), t.TempDir()
	if err := initialize(home, workspace, false); err == nil {
		t.Fatal("empty migration accepted")
	}
	if _, err := os.Stat(filepath.Join(home, ".codex")); !os.IsNotExist(err) {
		t.Fatal("initialized missing restore")
	}
}
