//go:build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This check crosses the CLI, real SOPS, and the rootless Podman secret store.
// It does not start a connector or contact Cloudflare.
func TestGatewayConfigAndSecrets(t *testing.T) {
	for _, tool := range []string{"sops", "age-keygen", "podman", "just"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("integration tool required: %s", tool)
		}
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "age.key")
	if out, err := exec.Command("age-keygen", "-o", key).CombinedOutput(); err != nil {
		t.Fatalf("generate fixture key: %v %s", err, out)
	}
	recipient, err := exec.Command("age-keygen", "-y", key).Output()
	if err != nil {
		t.Fatal(err)
	}
	policy := filepath.Join(dir, ".sops.yaml")
	if err := os.WriteFile(policy, []byte("creation_rules:\n  - age: "+strings.TrimSpace(string(recipient))+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("check-%x", time.Now().UnixNano())
	secret := "ya-infra-gateway-" + name
	defer func() {
		if out, err := exec.Command("podman", "secret", "rm", "--ignore", secret).CombinedOutput(); err != nil {
			t.Errorf("remove test secret: %v %s", err, out)
		}
	}()
	management := "private-management-fixture-value"
	token := "private-connector-fixture-value"
	input := func(token string) string {
		return fmt.Sprintf("gateways:\n  %s:\n    zone: example.com\n    account_id: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n    tunnel_id: 00000000-0000-0000-0000-000000000001\n    api_token: %s\n    tunnel_token: %q\nroutes: []\n", name, management, token)
	}
	encrypt := func(content string) {
		t.Helper()
		cmd := exec.Command("sops", "--config", policy, "encrypt", "--input-type", "yaml", "--output-type", "yaml", "/dev/stdin")
		cmd.Stdin = strings.NewReader(content)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("encrypt fixture: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.sops.yaml"), out, 0600); err != nil {
			t.Fatal(err)
		}
	}
	env := append(os.Environ(), "SOPS_AGE_KEY_FILE="+key, "SOPS_AGE_KEY=", "XDG_STATE_HOME="+filepath.Join(dir, "state"))
	run := func(wantSuccess bool, args ...string) {
		t.Helper()
		cmd := exec.Command(testCLI, args...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		if (err == nil) != wantSuccess || bytes.Contains(out, []byte(management)) || bytes.Contains(out, []byte(token)) {
			t.Fatalf("unexpected CLI result or leaked fixture credential: success=%t, want=%t", err == nil, wantSuccess)
		}
	}
	inspect := func() (string, string) {
		t.Helper()
		out, err := exec.Command("podman", "secret", "inspect", "--showsecret", secret).Output()
		if err != nil {
			t.Fatal(err)
		}
		var records []struct{ ID, SecretData string }
		if err := json.Unmarshal(out, &records); err != nil || len(records) != 1 {
			t.Fatal("invalid secret observation")
		}
		return records[0].ID, records[0].SecretData
	}
	encrypt(input(token))
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("just", "--justfile", filepath.Join(root, "..", "justfile"), "gateway", "prepare", name)
	cmd.Dir, cmd.Env = dir, env
	if out, err := cmd.CombinedOutput(); err != nil || bytes.Contains(out, []byte(token)) || bytes.Contains(out, []byte(management)) {
		t.Fatalf("single-file input through root just: %v", err)
	}
	first, value := inspect()
	if value != token {
		t.Fatal("the runtime secret did not contain only the connector token")
	}
	run(true, "gateway", "prepare", name)
	second, _ := inspect()
	if first != second {
		t.Fatal("unchanged preparation replaced the secret")
	}
	encrypt(input(""))
	run(false, "gateway", "prepare", name)
	if id, _ := inspect(); id != first {
		t.Fatal("missing credential changed the existing secret")
	}
	encrypt(input(token))
	env = append(env, "SOPS_AGE_KEY_FILE="+filepath.Join(dir, "missing.key"))
	run(false, "gateway", "prepare", name)
	env = append(env, "SOPS_AGE_KEY_FILE="+key)
	run(false, "gateway", "prepare", "unknown")
	encrypt("source: ./source\nkube_context: private-connector-fixture-value\n")
	run(false, "gateway", "prepare", name)
	token += "-rotated"
	encrypt(input(token))
	run(true, "gateway", "prepare", name)
	rotated, value := inspect()
	if rotated == first || value != token {
		t.Fatal("changed credential was not prepared")
	}
	cmd = exec.Command("podman", "secret", "create", "--replace", secret, "-")
	cmd.Stdin = strings.NewReader("unowned")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare unowned fixture: %v %s", err, out)
	}
	unowned, _ := inspect()
	run(false, "gateway", "prepare", name)
	if id, value := inspect(); id != unowned || value != "unowned" {
		t.Fatal("unowned secret was overwritten")
	}
	t.Log(`{"input":"single encrypted file in caller directory","unchanged":"same secret ID","rotation":"new connector token","unowned":"refused","missing_key":"refused","credentials":"not printed"}`)
}
