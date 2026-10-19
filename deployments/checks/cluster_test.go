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
	defer kubectl(nil, "delete", "namespace", namespace, "--wait=false")
	root, _ := filepath.Abs("..")
	temp := t.TempDir()
	example := filepath.Join(root, "examples/app/kubernetes")
	for _, name := range []string{"deployment.yaml", "service.yaml"} {
		data, err := os.ReadFile(filepath.Join(example, name))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(temp, name), data, 0600)
	}
	// Attach an encrypted Secret to the real public example through native Kustomize input.
	deploymentPath := filepath.Join(temp, "deployment.yaml")
	raw, _ := os.ReadFile(deploymentPath)
	var deployment map[string]any
	yaml.Unmarshal(raw, &deployment)
	spec := deployment["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
	spec["containers"].([]any)[0].(map[string]any)["envFrom"] = []any{map[string]any{"secretRef": map[string]any{"name": "check-config"}}}
	raw, _ = yaml.Marshal(deployment)
	os.WriteFile(deploymentPath, raw, 0600)
	os.WriteFile(filepath.Join(temp, "kustomization.yaml"), []byte("resources: [deployment.yaml, service.yaml]\nnamespace: "+namespace+"\ngenerators: [ksops.yaml]\n"), 0600)
	os.WriteFile(filepath.Join(temp, "ksops.yaml"), []byte("apiVersion: viaduct.ai/v1\nkind: ksops\nmetadata:\n  name: check-config\n  annotations:\n    config.kubernetes.io/function: |\n      exec:\n        path: ksops\nsecretFrom:\n  - metadata:\n      name: check-config\n      annotations:\n        kustomize.config.k8s.io/needs-hash: \"true\"\n    files: [config.yaml=config.sops.yaml]\n"), 0600)
	key := filepath.Join(temp, "age.key")
	command(t, os.Environ(), "age-keygen", "-o", key)
	recipient := strings.TrimSpace(string(command(t, os.Environ(), "age-keygen", "-y", key)))
	env := append(os.Environ(), "SOPS_AGE_KEY_FILE="+key)
	policy := filepath.Join(temp, "policy.yaml")
	os.WriteFile(policy, []byte("creation_rules:\n  - age: "+recipient+"\n"), 0600)
	render := func(value string) []byte {
		t.Helper()
		input := filepath.Join(temp, "config.yaml")
		os.WriteFile(input, []byte("token: "+value+"\n"), 0600)
		encrypted := command(t, env, "sops", "--config", policy, "encrypt", "--age", recipient, input)
		os.WriteFile(filepath.Join(temp, "config.sops.yaml"), encrypted, 0600)
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
	var classes struct{ Items []map[string]any }
	json.Unmarshal(must(nil, "get", "storageclass", "-o", "json"), &classes)
	var original map[string]any
	for _, class := range classes.Items {
		if class["provisioner"] == "rancher.io/local-path" {
			original = class
			break
		}
	}
	if original == nil {
		t.Fatal("the disposable cluster needs the local-path provisioner for retention checks")
	}
	class := map[string]any{"apiVersion": "storage.k8s.io/v1", "kind": "StorageClass", "metadata": map[string]any{"name": namespace}, "provisioner": original["provisioner"], "reclaimPolicy": "Retain", "volumeBindingMode": "WaitForFirstConsumer"}
	if original["parameters"] != nil {
		class["parameters"] = original["parameters"]
	}
	apply(class)
	defer kubectl(nil, "delete", "storageclass", namespace)
	claim := map[string]any{"apiVersion": "v1", "kind": "PersistentVolumeClaim", "metadata": map[string]any{"name": "data", "namespace": namespace}, "spec": map[string]any{"storageClassName": namespace, "accessModes": []string{"ReadWriteOnce"}, "resources": map[string]any{"requests": map[string]any{"storage": "64Mi"}}}}
	apply(claim)
	image := spec["containers"].([]any)[0].(map[string]any)["image"]
	pod := func(name string, argv []string) map[string]any {
		return map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": map[string]any{"name": name, "namespace": namespace}, "spec": map[string]any{"restartPolicy": "Never", "containers": []any{map[string]any{"name": "check", "image": image, "command": argv, "volumeMounts": []any{map[string]any{"name": "data", "mountPath": "/data"}}}}, "volumes": []any{map[string]any{"name": "data", "persistentVolumeClaim": map[string]any{"claimName": "data"}}}}}
	}
	apply(pod("write", []string{"mkdir", "-p", "/data/retained"}))
	must(nil, "-n", namespace, "wait", "--for=jsonpath={.status.phase}=Succeeded", "pod/write", "--timeout=120s")
	volume := strings.TrimSpace(string(must(nil, "-n", namespace, "get", "pvc/data", "-o", "jsonpath={.spec.volumeName}")))
	if volume == "" {
		t.Fatal("PVC did not bind")
	}
	defer kubectl(nil, "delete", "pv", volume, "--wait=false")
	if string(must(nil, "get", "pv", volume, "-o", "jsonpath={.spec.persistentVolumeReclaimPolicy}")) != "Retain" {
		t.Fatal("PV did not inherit retention")
	}
	must(nil, "-n", namespace, "delete", "pod/write", "--wait=true")
	must(nil, "-n", namespace, "delete", "pvc/data", "--wait=true")
	must(nil, "wait", "--for=jsonpath={.status.phase}=Released", "pv/"+volume, "--timeout=60s")
	must(nil, "patch", "pv", volume, "--type=json", "-p", `[{"op":"remove","path":"/spec/claimRef"}]`)
	claim["spec"].(map[string]any)["volumeName"] = volume
	apply(claim)
	apply(pod("read", []string{"test", "-d", "/data/retained"}))
	must(nil, "-n", namespace, "wait", "--for=jsonpath={.status.phase}=Succeeded", "pod/read", "--timeout=120s")
	t.Log(`{"apply":"idempotent","Secret_diff":"masked","config_rollout":"ready","PV_retention":"retained and rebound"}`)
}
