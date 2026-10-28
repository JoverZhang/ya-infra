//go:build integration

package checks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sopsFixture(t *testing.T, dir string) (string, string) {
	t.Helper()
	key := filepath.Join(dir, "age.key")
	if out, err := exec.Command("age-keygen", "-o", key).CombinedOutput(); err != nil {
		t.Fatalf("fixture key: %v %s", err, out)
	}
	recipient, err := exec.Command("age-keygen", "-y", key).Output()
	if err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(dir, ".sops.yaml")
	data, err := os.ReadFile("../.sops.yaml")
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), "REPLACE_WITH_YOUR_PUBLIC_AGE_RECIPIENT", strings.TrimSpace(string(recipient))))
	if err := os.WriteFile(policy, data, 0600); err != nil {
		t.Fatal(err)
	}
	return key, policy
}
