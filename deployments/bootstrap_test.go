package main

import (
	"github.com/JoverZhang/ya-infra/deployments/internal/host"
	"os"
	"path/filepath"
	"testing"
)

func TestXDGDataFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, value := range []string{"", "relative"} {
		t.Setenv("XDG_DATA_HOME", value)
		if got := host.XDG("XDG_DATA_HOME", ".local/share"); got != filepath.Join(home, ".local/share") {
			t.Fatal(got)
		}
	}
}

func TestPrivateWriteIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private")
	if err := host.WritePrivate(path, []byte("same")); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := host.WritePrivate(path, []byte("same")); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) || after.Mode().Perm() != 0600 {
		t.Fatal("unchanged private file was rewritten")
	}
}
