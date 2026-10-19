package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClientSetupPreservesUnownedFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "client.yaml")
	state := filepath.Join(dir, "state.json")
	os.WriteFile(target, []byte("existing"), 0600)
	if err := installClient(target, []byte("replacement"), state); err == nil {
		t.Fatal("replaced unowned client configuration")
	}
	if err := installClient(target, []byte("existing"), state); err != nil {
		t.Fatal(err)
	}
	if err := installClient(target, []byte("changed"), state); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(target, []byte("external edit"), 0600)
	if err := installClient(target, []byte("replacement"), state); err == nil {
		t.Fatal("lost external edit")
	}
}
