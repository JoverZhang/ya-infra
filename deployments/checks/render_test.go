//go:build integration

package checks

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func command(t *testing.T, env []string, name string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			t.Fatalf("%s failed: %s", name, e.Stderr)
		}
		t.Fatal(err)
	}
	return out
}
func TestEncryptedRendering(t *testing.T) {
	for _, name := range []string{"sops", "age-keygen", "ksops", "kustomize"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatalf("required integration tool: %s", name)
		}
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	source := filepath.Join(temp, "source")
	err = filepath.WalkDir(root, func(path string, item fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		if item.IsDir() {
			if item.Name() == ".bin" || item.Name() == ".infra" || item.Name() == "services" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".sops.yaml") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(source, relative)
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(temp, "age.key")
	command(t, os.Environ(), "age-keygen", "-o", key)
	recipient := strings.TrimSpace(string(command(t, os.Environ(), "age-keygen", "-y", key)))
	policy := filepath.Join(temp, "policy.yaml")
	os.WriteFile(policy, []byte("creation_rules:\n  - age: "+recipient+"\n"), 0600)
	env := append(os.Environ(), "SOPS_AGE_KEY_FILE="+key)
	encrypt := func(dir, input, output string) {
		t.Helper()
		data := command(t, env, "sops", "--config", policy, "encrypt", "--age", recipient, filepath.Join(source, dir, input))
		if err := os.WriteFile(filepath.Join(source, dir, output), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	encrypt("gateway/connector", "secret.example.yaml", "secret.sops.yaml")
	encrypt("apps/paseo/kubernetes", "secret.example.yaml", "secret.sops.yaml")
	encrypt("apps/tbg/kubernetes", "config.example.yaml", "config.sops.yaml")
	render := func(dir string) []byte {
		t.Helper()
		out := command(t, env, "kustomize", "build", "--enable-alpha-plugins", "--enable-exec", filepath.Join(source, dir))
		if bytes.Contains(out, []byte("ENC[")) || bytes.Contains(out, []byte("kind: ksops")) || bytes.Contains(out, []byte("sops:")) {
			t.Fatal("renderer leaked local encryption metadata")
		}
		return out
	}
	for _, dir := range []string{"examples/app/kubernetes", "kubernetes/cluster", "apps/paseo/kubernetes", "apps/tbg/kubernetes", "apps/browserlab/kubernetes"} {
		resources := render(dir)
		if dir == "apps/paseo/kubernetes" {
			checkWebEgress(t, resources)
		}
	}
	before := render("apps/tbg/kubernetes")
	encrypt("apps/tbg/kubernetes", "config.example.yaml", "config.sops.yaml")
	same := render("apps/tbg/kubernetes")
	if !bytes.Equal(before, same) {
		t.Fatal("re-encryption changed effective resources")
	}
	changed := filepath.Join(source, "apps/tbg/kubernetes/config.example.yaml")
	data, _ := os.ReadFile(changed)
	data = bytes.ReplaceAll(data, []byte("18473"), []byte("18474"))
	os.WriteFile(changed, data, 0600)
	encrypt("apps/tbg/kubernetes", "config.example.yaml", "config.sops.yaml")
	after := render("apps/tbg/kubernetes")
	oldSecret, oldReference := secretIdentity(t, before)
	newSecret, newReference := secretIdentity(t, after)
	if oldSecret == newSecret || oldSecret != oldReference || newSecret != newReference {
		t.Fatalf("Secret hash and Pod reference did not change together: %q %q %q %q", oldSecret, oldReference, newSecret, newReference)
	}
	command(t, append(env, "SOPS_AGE_KEY_FILE="+filepath.Join(temp, "missing.key")), "kustomize", "version")
	failed := exec.Command("kustomize", "build", "--enable-alpha-plugins", "--enable-exec", filepath.Join(source, "apps/tbg/kubernetes"))
	failed.Env = append(os.Environ(), "SOPS_AGE_KEY_FILE="+filepath.Join(temp, "missing.key"), "SOPS_AGE_KEY=", "XDG_CONFIG_HOME="+filepath.Join(temp, "no-keys"))
	if err := failed.Run(); err == nil {
		t.Fatal("missing key did not stop rendering")
	}
	evidence, _ := json.Marshal(map[string]any{"public_and_encrypted_apps": "rendered", "reencryption": "unchanged", "config_update": "Secret and workload reference changed", "missing_key": "failed before cluster access"})
	t.Log(string(evidence))
}

func checkWebEgress(t *testing.T, data []byte) {
	t.Helper()
	type resource struct {
		Kind     string
		Metadata struct{ Name string }
		Spec     struct {
			Template struct {
				Metadata struct{ Labels map[string]string }
			}
			PodSelector struct {
				MatchLabels map[string]string `yaml:"matchLabels"`
			} `yaml:"podSelector"`
			PolicyTypes []string `yaml:"policyTypes"`
			Egress      []any
		}
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var webLabels map[string]string
	var policies []resource
	for {
		var item resource
		if err := decoder.Decode(&item); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if item.Kind == "Deployment" && item.Metadata.Name == "paseo-web" {
			webLabels = item.Spec.Template.Metadata.Labels
		}
		if item.Kind == "NetworkPolicy" {
			policies = append(policies, item)
		}
	}
	if len(webLabels) == 0 {
		t.Fatal("rendered Paseo Web Pod labels are missing")
	}
	isolated := false
	for _, policy := range policies {
		matches := true
		for name, value := range policy.Spec.PodSelector.MatchLabels {
			matches = matches && webLabels[name] == value
		}
		if !matches {
			continue
		}
		for _, kind := range policy.Spec.PolicyTypes {
			if kind == "Egress" {
				isolated = true
				if len(policy.Spec.Egress) != 0 {
					t.Fatal("rendered policy permits Web outbound connections", policy.Metadata.Name)
				}
			}
		}
	}
	if !isolated {
		t.Fatal("rendered Paseo Web has no egress isolation")
	}
}
func secretIdentity(t *testing.T, data []byte) (string, string) {
	t.Helper()
	var name, reference string
	// Read only these fields; this view is never written back to the cluster.
	type secretResource struct {
		Kind     string
		Metadata objectMetadata
		Spec     deploymentSpec
	}
	for _, resource := range decodeDocuments[secretResource](t, data) {
		if resource.Kind == "Secret" {
			name = resource.Metadata.Name
		}
		if resource.Kind == "Deployment" {
			for _, volume := range resource.Spec.Template.Spec.Volumes {
				if volume.Secret != nil {
					reference = volume.Secret.SecretName
				}
			}
		}
	}
	return name, reference
}
