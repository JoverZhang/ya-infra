package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestClientOwnershipDecodeCause(t *testing.T) {
	directory := t.TempDir()
	statePath := filepath.Join(directory, "state.json")
	if err := os.WriteFile(statePath, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	err := installClient(filepath.Join(directory, "client.yaml"), []byte("fixture"), statePath)
	var syntax *json.SyntaxError
	if !errors.As(err, &syntax) || !strings.Contains(err.Error(), statePath) {
		t.Fatalf("missing ownership file or decode cause: %v", err)
	}
}
