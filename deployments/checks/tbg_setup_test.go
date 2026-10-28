//go:build integration

package checks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestTBGSetup(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "tbg-setup")
	if out, err := exec.Command("go", "build", "-o", binary, "../apps/tbg").CombinedOutput(); err != nil {
		t.Fatalf("build setup: %v %s", err, out)
	}
	key, policy := sopsFixture(t, dir)
	plain := "listen: 127.0.0.1:18474\ndata_dir: /var/lib/tbg\ntelegram:\n  bot_token: synthetic-fixture-token\nadmins: []\n"
	cmd := exec.Command("sops", "--config", policy, "encrypt", "--filename-override", "config.sops.yaml", "--input-type", "yaml", "--output-type", "yaml", "/dev/stdin")
	cmd.Stdin = strings.NewReader(plain)
	encrypted, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.sops.yaml"), encrypted, 0600); err != nil {
		t.Fatal(err)
	}
	secret := fmt.Sprintf("ya-infra-tbg-check-%x", time.Now().UnixNano())
	defer func() {
		if out, err := exec.Command("podman", "secret", "rm", "--ignore", secret).CombinedOutput(); err != nil {
			t.Errorf("remove fixture secret: %v %s", err, out)
		}
	}()
	env := append(os.Environ(), "SOPS_AGE_KEY_FILE="+key, "SOPS_AGE_KEY=", "XDG_CONFIG_HOME="+filepath.Join(dir, "config"), "XDG_STATE_HOME="+filepath.Join(dir, "state"))
	run := func(success bool) {
		t.Helper()
		cmd := exec.Command(binary, "setup", "--secret-name", secret)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		if (err == nil) != success || strings.Contains(string(out), "synthetic-fixture-token") {
			t.Fatalf("setup success=%t, expected=%t (output withheld)", err == nil, success)
		}
	}
	run(true)
	inspect := func() string {
		t.Helper()
		data, err := exec.Command("podman", "secret", "inspect", "--showsecret", secret).Output()
		if err != nil {
			t.Fatal(err)
		}
		var records []struct{ ID, SecretData string }
		if json.Unmarshal(data, &records) != nil || len(records) != 1 {
			t.Fatal("invalid secret observation")
		}
		var want, got any
		if yaml.Unmarshal([]byte(plain), &want) != nil || yaml.Unmarshal([]byte(records[0].SecretData), &got) != nil || !reflect.DeepEqual(want, got) {
			t.Fatal("server configuration was not preserved")
		}
		return records[0].ID
	}
	first := inspect()
	client := filepath.Join(dir, "config/tbg/client.yaml")
	data, err := os.ReadFile(client)
	if err != nil || !strings.Contains(string(data), "18474") || !strings.Contains(string(data), "127.0.0.1") {
		t.Fatal("host client does not match the server endpoint")
	}
	run(true)
	if inspect() != first {
		t.Fatal("repeated setup replaced an unchanged secret")
	}
	if err := os.WriteFile(client, []byte("host: 127.0.0.1\nport: 19999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(false)
	if inspect() != first {
		t.Fatal("conflicting host client changed the runtime secret")
	}
	info, err := os.Stat(client)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("host client is not private")
	}
	t.Log(`{"server_config":"preserved in Podman secret","client":"derived from server endpoint","repeat_setup":"same secret ID","external_edit":"refused","telegram_messages":"none"}`)
}
