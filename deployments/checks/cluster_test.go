//go:build integration

package checks

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

var clusterContext = flag.String("cluster-context", "", "explicit disposable cluster context; enables cluster writes")

func TestClusterApplyAndRetention(t *testing.T) {
	if *clusterContext == "" {
		t.Skip("set -cluster-context to enable disposable cluster checks")
	}
	namespace := fmt.Sprintf("ya-infra-check-%x", time.Now().UnixNano())
	kubectl := func(input []byte, args ...string) ([]byte, error) {
		cmd := exec.Command("kubectl", append([]string{"--context=" + *clusterContext}, args...)...)
		cmd.Stdin = bytes.NewReader(input)
		return cmd.CombinedOutput()
	}
	must := func(input []byte, args ...string) []byte {
		t.Helper()
		out, err := kubectl(input, args...)
		if err != nil {
			t.Fatalf("kubectl %v: %v\n%s", args, err, out)
		}
		return out
	}
	apply := func(value any) {
		t.Helper()
		data, err := yaml.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		must(data, "apply", "--server-side", "-f", "-")
	}
	must(nil, "create", "namespace", namespace)
	defer func() {
		remove := func(kind, name string) {
			out, err := kubectl(nil, "delete", kind, name, "--ignore-not-found", "--wait=true", "--timeout=90s")
			if err != nil {
				t.Errorf("cleanup %s/%s failed; inspect remaining resources: %v\n%s", kind, name, err, out)
			}
		}
		// Release claims before removing retained PVs. The unique class identifies
		// our volumes even if the test failed before reading the bound PV name.
		remove("namespace", namespace)
		selector := `jsonpath={.items[?(@.spec.storageClassName=="` + namespace + `")].metadata.name}`
		volumes, err := kubectl(nil, "get", "pv", "-o", selector)
		if err != nil {
			t.Errorf("cleanup cannot list PVs for StorageClass %s: %v\n%s", namespace, err, volumes)
		} else {
			for _, volume := range strings.Fields(string(volumes)) {
				remove("pv", volume)
			}
		}
		remove("storageclass", namespace)
	}()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	example := filepath.Join(root, "examples/app/kubernetes")
	for _, name := range []string{"deployment.yaml", "service.yaml"} {
		data, err := os.ReadFile(filepath.Join(example, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(temp, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Attach an encrypted Secret to the real public example through native Kustomize input.
	deploymentPath := filepath.Join(temp, "deployment.yaml")
	raw, err := os.ReadFile(deploymentPath)
	if err != nil {
		t.Fatal(err)
	}
	var workload deployment
	if err := yaml.Unmarshal(raw, &workload); err != nil {
		t.Fatal(err)
	}
	spec := &workload.Spec.Template.Spec
	spec.Containers[0].EnvFrom = []environmentSource{{SecretRef: &objectReference{Name: "check-config"}}}
	raw, err = yaml.Marshal(workload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deploymentPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"kustomization.yaml", "ksops.yaml"} {
		resource := fixtureResource[map[string]any](t, "cluster-"+name, "FIXTURE_NAMESPACE", namespace)
		data, err := yaml.Marshal(resource)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(temp, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	key := filepath.Join(temp, "age.key")
	command(t, os.Environ(), "age-keygen", "-o", key)
	recipient := strings.TrimSpace(string(command(t, os.Environ(), "age-keygen", "-y", key)))
	env := append(os.Environ(), "SOPS_AGE_KEY_FILE="+key)
	policy := filepath.Join(temp, "policy.yaml")
	if err := os.WriteFile(policy, []byte("creation_rules:\n  - age: "+recipient+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	render := func(value string) []byte {
		t.Helper()
		input := filepath.Join(temp, "config.yaml")
		if err := os.WriteFile(input, []byte("token: "+value+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		encrypted := command(t, env, "sops", "--config", policy, "encrypt", "--age", recipient, input)
		if err := os.WriteFile(filepath.Join(temp, "config.sops.yaml"), encrypted, 0600); err != nil {
			t.Fatal(err)
		}
		return command(t, env, "kustomize", "build", "--enable-alpha-plugins", "--enable-exec", temp)
	}
	first := render("fixture-one")
	must(first, "apply", "--server-side", "-f", "-")
	must(nil, "-n", namespace, "rollout", "status", "deployment/app-web", "--timeout=180s")
	generation := string(must(nil, "-n", namespace, "get", "deployment/app-web", "-o", "jsonpath={.metadata.generation}"))
	must(first, "apply", "--server-side", "-f", "-")
	if actual := string(must(nil, "-n", namespace, "get", "deployment/app-web", "-o", "jsonpath={.metadata.generation}")); actual != generation {
		t.Fatal("unchanged apply changed the workload")
	}
	out, err := kubectl(first, "diff", "--server-side", "-f", "-")
	if err != nil {
		t.Fatalf("unchanged diff: %v %s", err, out)
	}
	next := render("fixture-two")
	out, err = kubectl(next, "diff", "--server-side", "-f", "-")
	if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
		t.Fatalf("changed diff exit: %v", err)
	}
	if bytes.Contains(out, []byte("fixture-two")) || bytes.Contains(out, []byte(base64.StdEncoding.EncodeToString([]byte("token: fixture-two\n")))) {
		t.Fatal("kubectl diff exposed Secret data")
	}
	must(next, "apply", "--server-side", "-f", "-")
	must(nil, "-n", namespace, "rollout", "status", "deployment/app-web", "--timeout=180s")
	if actual := string(must(nil, "-n", namespace, "get", "deployment/app-web", "-o", "jsonpath={.metadata.generation}")); actual == generation {
		t.Fatal("Secret update did not roll the workload")
	}
	// Use a unique retained StorageClass. No existing class or volume is changed.
	var classes struct{ Items []storageClass }
	if err := json.Unmarshal(must(nil, "get", "storageclass", "-o", "json"), &classes); err != nil {
		t.Fatal(err)
	}
	var original *storageClass
	for _, class := range classes.Items {
		if class.Provisioner == "rancher.io/local-path" {
			original = &class
			break
		}
	}
	if original == nil {
		t.Fatal("the disposable cluster needs the local-path provisioner for retention checks")
	}
	class := storageClass{
		APIVersion: "storage.k8s.io/v1", Kind: "StorageClass", Metadata: objectMetadata{Name: namespace},
		Provisioner: original.Provisioner, Parameters: original.Parameters,
		ReclaimPolicy: "Retain", VolumeBindingMode: "WaitForFirstConsumer",
	}
	apply(class)
	claim := fixtureResource[persistentVolumeClaim](t, "retention-claim.yaml", "FIXTURE_NAMESPACE", namespace)
	apply(claim)
	image := spec.Containers[0].Image
	pod := func(name string, argv []string) map[string]any {
		t.Helper()
		command, err := json.Marshal(argv)
		if err != nil {
			t.Fatal(err)
		}
		return fixtureResource[map[string]any](t, "retention-pod.yaml", "FIXTURE_NAMESPACE", namespace,
			"FIXTURE_NAME", name, "FIXTURE_IMAGE", image, "FIXTURE_COMMAND", string(command))
	}
	apply(pod("write", []string{"mkdir", "-p", "/data/retained"}))
	must(nil, "-n", namespace, "wait", "--for=jsonpath={.status.phase}=Succeeded", "pod/write", "--timeout=120s")
	volume := strings.TrimSpace(string(must(nil, "-n", namespace, "get", "pvc/data", "-o", "jsonpath={.spec.volumeName}")))
	if volume == "" {
		t.Fatal("PVC did not bind")
	}
	if string(must(nil, "get", "pv", volume, "-o", "jsonpath={.spec.persistentVolumeReclaimPolicy}")) != "Retain" {
		t.Fatal("PV did not inherit retention")
	}
	must(nil, "-n", namespace, "delete", "pod/write", "--wait=true")
	must(nil, "-n", namespace, "delete", "pvc/data", "--wait=true")
	must(nil, "wait", "--for=jsonpath={.status.phase}=Released", "pv/"+volume, "--timeout=60s")
	must(nil, "patch", "pv", volume, "--type=json", "-p", `[{"op":"remove","path":"/spec/claimRef"}]`)
	claim.Spec.VolumeName = volume
	apply(claim)
	apply(pod("read", []string{"test", "-d", "/data/retained"}))
	must(nil, "-n", namespace, "wait", "--for=jsonpath={.status.phase}=Succeeded", "pod/read", "--timeout=120s")
	t.Log(`{"apply":"idempotent","Secret_diff":"masked","config_rollout":"ready","PV_retention":"retained and rebound"}`)
}
