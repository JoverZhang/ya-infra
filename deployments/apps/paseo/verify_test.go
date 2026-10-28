package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == "sops" {
		data, err := os.ReadFile(os.Args[len(os.Args)-1])
		if err != nil {
			os.Exit(1)
		}
		os.Stdout.Write(data)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestVerifyLeavesHostUnchanged(t *testing.T) {
	if _, err := exec.LookPath("paseo"); err != nil {
		t.Skip("native Paseo CLI is not installed")
	}
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(self, filepath.Join(bin, "sops")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("HOME", root)
	t.Setenv("PASEO_HOME", "")
	t.Setenv("PASEO_HOST", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/healthz":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/status" && r.Header.Get("Authorization") == "Bearer disposable-password":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/status":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	secret := filepath.Join(root, "secret.yaml")
	data := "PASEO_PASSWORD: disposable-password\nPASEO_HOSTNAMES: 127.0.0.1\n"
	if err := os.WriteFile(secret, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	err = run([]string{"verify", "--web-url", server.URL, "--daemon-url", server.URL, "--secret-file", secret})
	if err == nil || !strings.Contains(err.Error(), "WebSocket") {
		t.Fatalf("expected deliberate WebSocket rejection after HTTP checks: %v", err)
	}
	path := filepath.Join(root, ".paseo", "cli-client-id")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("verify created host client state at %s", path)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
