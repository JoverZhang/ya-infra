//go:build integration

package checks

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

// This check uses real SOPS and Compose, synthetic credentials, and an external
// volume created only for this run. It never reads application configuration.
func TestComposeLifecycle(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	app := filepath.Join(dir, "deployments/apps/check")
	if err := os.MkdirAll(app, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"justfile", "deployments/justfile"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	key, policy := sopsFixture(t, dir)
	project := fmt.Sprintf("ya-infra-check-%x", time.Now().UnixNano())
	volume := project + "-data"
	token := "fixture-$HOME-$(touch should-not-exist)"
	plain := "COMPOSE_PROJECT_NAME=" + project + "\nDATA_VOLUME=" + volume + "\nTOKEN=" + token + "\n"
	cmd := exec.Command("sops", "--config", policy, "encrypt", "--filename-override", "config.sops.env", "--input-type", "dotenv", "--output-type", "dotenv", "/dev/stdin")
	cmd.Stdin = strings.NewReader(plain)
	encrypted, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "config.sops.env"), encrypted, 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("testdata/compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "compose.yaml"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "asset"), []byte("read-only"), 0600); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "SOPS_AGE_KEY_FILE="+key, "SOPS_AGE_KEY=", "XDG_CONFIG_HOME="+filepath.Join(dir, "config"))
	run := func(wantSuccess bool, args ...string) []byte {
		t.Helper()
		cmd := exec.Command("just", append([]string{"--justfile", filepath.Join(dir, "justfile"), "compose", "check"}, args...)...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		if (err == nil) != wantSuccess || bytes.Contains(out, []byte(token)) {
			t.Fatalf("Compose result for %q: success=%t, expected=%t (output withheld)", args, err == nil, wantSuccess)
		}
		return out
	}
	defer func() {
		if out, err := exec.Command("podman", "rm", "--force", "--ignore", "--filter", "label=com.docker.compose.project="+project).CombinedOutput(); err != nil {
			t.Errorf("remove fixture containers: %v %s", err, out)
		}
		if exec.Command("podman", "volume", "exists", volume).Run() == nil {
			if out, err := exec.Command("podman", "volume", "rm", volume).CombinedOutput(); err != nil {
				t.Errorf("remove fixture volume: %v %s", err, out)
			}
		}
	}()
	run(false, "up", "-d")
	if err := exec.Command("podman", "volume", "exists", volume).Run(); err == nil {
		t.Fatal("missing external volume was created implicitly")
	}
	if out, err := exec.Command("podman", "volume", "create", volume).CombinedOutput(); err != nil {
		t.Fatalf("create fixture volume: %v %s", err, out)
	}
	env = append(env, "SOPS_AGE_KEY_FILE="+filepath.Join(dir, "missing.key"))
	run(false, "up", "-d")
	env = append(env, "SOPS_AGE_KEY_FILE="+key)
	run(true, "up", "-d")
	container := func() string {
		t.Helper()
		out, err := exec.Command("podman", "ps", "-aq", "--filter", "label=com.docker.compose.project="+project).Output()
		if err != nil || strings.TrimSpace(string(out)) == "" {
			t.Fatal("fixture container is missing")
		}
		return strings.TrimSpace(string(out))
	}
	first := container()
	run(true, "up", "-d")
	if container() != first {
		t.Fatal("unchanged native up replaced the container")
	}
	probe := `const fs=require('fs'); if(process.env.TOKEN!==process.argv[1] || fs.readFileSync('/data/retained','utf8')!=='keep') process.exit(1); try {fs.writeFileSync('/asset','bad');process.exit(2)} catch(e) {if(e.code!=='EROFS')throw e}`
	// Pass the synthetic expected value only to this disposable process, never a
	// production credential. Shell metacharacters must remain literal arguments.
	run(true, "exec", "-T", "check", "node", "-e", probe, "--", token)
	argument := "'quoted'\n$HOME; $(touch should-not-exist)"
	literal, err := json.Marshal(argument)
	if err != nil {
		t.Fatal(err)
	}
	run(true, "exec", "-T", "check", "node", "-e", "if(process.argv[1]!=="+string(literal)+")process.exit(1)", "--", argument)
	cmd = exec.Command("just", "--justfile", filepath.Join(dir, "justfile"), "compose", "check", "exec", "-T", "check", "node", "-e", "process.exit(37)")
	cmd.Dir, cmd.Env = dir, env
	if err := cmd.Run(); err == nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 37 {
		t.Fatal("native command exit status was not preserved")
	}
	if _, err := os.Stat(filepath.Join(app, "should-not-exist")); !os.IsNotExist(err) {
		t.Fatal("forwarded argument was executed as shell code")
	}
	run(true, "exec", "-T", "check", "node", "-e", "require('fs').writeFileSync('/data/retained','persisted')")
	run(true, "down", "-v")
	if err := exec.Command("podman", "volume", "exists", volume).Run(); err != nil {
		t.Fatal("native down -v deleted the external volume")
	}
	run(true, "up", "-d")
	run(true, "exec", "-T", "check", "node", "-e", "if(require('fs').readFileSync('/data/retained','utf8')!=='persisted')process.exit(1)")
	run(true, "down")
	if err := os.Remove(filepath.Join(app, "asset")); err != nil {
		t.Fatal(err)
	}
	run(false, "up", "-d")
	if _, err := os.Stat(filepath.Join(app, "asset")); !os.IsNotExist(err) {
		t.Fatal("missing bind source was created implicitly")
	}
	if err := os.WriteFile(filepath.Join(app, "asset"), []byte("read-only"), 0600); err != nil {
		t.Fatal(err)
	}
	evidence, err := json.Marshal(map[string]string{"input": "SOPS exec-env", "arguments": "literal", "exit_status": "preserved", "repeat_up": "same container", "down_v": "external data retained", "missing_key_or_source": "refused", "workspace": "read-only"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(evidence))
}
