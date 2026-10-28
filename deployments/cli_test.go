package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var testCLI string

func TestMain(m *testing.M) {
	for _, key := range []string{"GOCACHE", "GOMODCACHE"} {
		value, err := exec.Command("go", "env", key).Output()
		if err != nil {
			panic(err)
		}
		os.Setenv(key, strings.TrimSpace(string(value)))
	}
	dir, err := os.MkdirTemp("", "ya-infra-tests-")
	if err != nil {
		panic(err)
	}
	testCLI = filepath.Join(dir, "ya-infra")
	cmd := exec.Command("go", "build", "-o", testCLI, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestHelpDoesNotNeedConfiguration(t *testing.T) {
	cmd := exec.Command(testCLI, "--help")
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "bootstrap") || strings.Contains(string(out), "svc") {
		t.Fatalf("help: %v %s", err, out)
	}
}
