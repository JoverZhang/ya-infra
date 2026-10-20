package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicArtifactCopy(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "copy")
	os.WriteFile(filepath.Join(source, "program"), []byte("public"), 0755)
	os.WriteFile(filepath.Join(source, "auth.json"), []byte("private"), 0600)
	os.WriteFile(filepath.Join(source, ".env.local"), []byte("private"), 0600)
	if err := copyPublic(source, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "auth.json")); !os.IsNotExist(err) {
		t.Fatal("copied credentials")
	}
	os.Symlink("/etc/passwd", filepath.Join(source, "escape"))
	if err := copyPublic(source, filepath.Join(t.TempDir(), "copy")); err == nil {
		t.Fatal("accepted external symlink")
	}
}

func TestVerifyRefusesRedirectedCredentials(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("followed credential redirect") }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer source.Close()
	if err := verifyAPI(source.URL, "private-value"); err == nil || strings.Contains(err.Error(), "private-value") {
		t.Fatalf("unsafe verify: %v", err)
	}
}

func TestReusePublicSnapshot(t *testing.T) {
	manifest, err := os.ReadFile("testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		input []byte
		valid bool
	}{
		{"matching versions", manifest, true},
		{"different version", []byte(strings.Replace(string(manifest), paseoVersion, "0.0.0", 1)), false},
		{"invalid version type", []byte(strings.Replace(string(manifest), `"`+paseoVersion+`"`, "123", 1)), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			if err := os.WriteFile(filepath.Join(directory, "manifest.json"), test.input, 0600); err != nil {
				t.Fatal(err)
			}
			err := prepareArtifacts(directory)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, got %v", test.valid, err)
			}
		})
	}
}
