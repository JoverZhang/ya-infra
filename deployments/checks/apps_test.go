//go:build integration

package checks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// Run only after importing the pinned local images into the disposable cluster.
func TestApplicationImages(t *testing.T) {
	if *clusterContext == "" {
		t.Skip("set -cluster-context after importing the app images")
	}
	namespace := fmt.Sprintf("ya-infra-images-%x", time.Now().UnixNano())
	run := func(input []byte, args ...string) ([]byte, error) {
		cmd := exec.Command("kubectl", append([]string{"--context=" + *clusterContext, "-n", namespace}, args...)...)
		cmd.Stdin = bytes.NewReader(input)
		return cmd.CombinedOutput()
	}
	must := func(input []byte, args ...string) []byte {
		t.Helper()
		out, err := run(input, args...)
		if err != nil {
			t.Fatalf("kubectl %v: %v\n%s", args, err, out)
		}
		return out
	}
	must(nil, "create", "namespace", namespace)
	defer func() {
		if t.Failed() {
			for _, args := range [][]string{{"get", "events", "--sort-by=.lastTimestamp"}, {"logs", "deployment/browserlab", "--all-containers", "--tail=80"}} {
				out, _ := run(nil, args...)
				t.Log(string(out))
			}
		}
		run(nil, "delete", "namespace", namespace, "--wait=false")
	}()
	apply := func(resource map[string]any) {
		t.Helper()
		data, err := yaml.Marshal(resource)
		if err != nil {
			t.Fatal(err)
		}
		must(data, "apply", "--server-side", "-f", "-")
	}
	load := func(app, file string) []map[string]any {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "apps", app, "kubernetes", file))
		if err != nil {
			t.Fatal(err)
		}
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		var resources []map[string]any
		for {
			var resource map[string]any
			err := decoder.Decode(&resource)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			resources = append(resources, resource)
		}
		return resources
	}
	disposable := func(resource map[string]any) map[string]any {
		spec := resource["spec"].(map[string]any)
		spec["replicas"] = 1
		pod := spec["template"].(map[string]any)["spec"].(map[string]any)
		delete(pod, "nodeSelector")
		for _, value := range pod["volumes"].([]any) {
			v := value.(map[string]any)
			if v["persistentVolumeClaim"] != nil || v["hostPath"] != nil {
				delete(v, "persistentVolumeClaim")
				delete(v, "hostPath")
				v["emptyDir"] = map[string]any{}
			}
		}
		return pod
	}
	paseo := load("paseo", "deployment.yaml")
	for _, resource := range paseo {
		pod := disposable(resource)
		if resource["metadata"].(map[string]any)["name"] == "paseo-demo" {
			pod["initContainers"].([]any)[0].(map[string]any)["args"] = []string{"--new"}
			daemon := pod["containers"].([]any)[0].(map[string]any)
			delete(daemon, "envFrom")
			daemon["env"] = append(daemon["env"].([]any), map[string]any{"name": "PASEO_PASSWORD", "value": "disposable-test-password"})
			raw, _ := json.Marshal(resource)
			var second map[string]any
			json.Unmarshal(bytes.ReplaceAll(raw, []byte("paseo-demo"), []byte("paseo-second")), &second)
			apply(second)
		}
		apply(resource)
	}
	for _, name := range []string{"paseo-web", "paseo-demo", "paseo-second"} {
		must(nil, "rollout", "status", "deployment/"+name, "--timeout=180s")
	}
	must(nil, "exec", "deployment/paseo-demo", "-c", "daemon", "--", "touch", "/workspace/only-first")
	if _, err := run(nil, "exec", "deployment/paseo-second", "-c", "daemon", "--", "test", "-e", "/workspace/only-first"); err == nil {
		t.Fatal("second daemon saw the first daemon's workspace")
	}
	must(nil, "exec", "deployment/paseo-demo", "-c", "daemon", "--", "codex", "--version")
	must(nil, "exec", "deployment/paseo-demo", "-c", "daemon", "--", "paseo", "ls", "--host", "tcp://127.0.0.1:6767", "--json")
	must(nil, "exec", "deployment/paseo-second", "-c", "daemon", "--", "paseo", "ls", "--host", "tcp://127.0.0.1:6767", "--json")
	// Native process shutdown must terminate the old Pod without forced deletion.
	old := strings.TrimSpace(string(must(nil, "get", "pods", "-l", "app=paseo-second", "-o", "jsonpath={.items[0].metadata.name}")))
	must(nil, "scale", "deployment/paseo-second", "--replicas=0")
	must(nil, "wait", "--for=delete", "pod/"+old, "--timeout=45s")
	browser := load("browserlab", "deployment.yaml")[0]
	pod := disposable(browser)
	init := pod["initContainers"].([]any)
	image := init[0].(map[string]any)["image"]
	marker := map[string]any{"name": "fixture-marker", "image": image, "imagePullPolicy": "Never", "command": []string{"touch", "/config/.ya-infra-restored"}, "volumeMounts": []any{map[string]any{"name": "profile", "mountPath": "/config"}}}
	pod["initContainers"] = append([]any{marker}, init...)
	apply(browser)
	must(nil, "rollout", "status", "deployment/browserlab", "--timeout=240s")
	must(nil, "exec", "deployment/browserlab", "-c", "browser", "--", "curl", "-fsS", "http://127.0.0.1:9222/json/version")
	must(nil, "exec", "deployment/browserlab", "-c", "browser", "--", "curl", "-fsS", "http://127.0.0.1:9223/json/version")
	if _, err := run(nil, "exec", "deployment/browserlab", "-c", "browser", "--", "touch", "/workspace/lab/write-test"); err == nil {
		t.Fatal("browser workspace became writable")
	}
	old = strings.TrimSpace(string(must(nil, "get", "pods", "-l", "app=browserlab", "-o", "jsonpath={.items[0].metadata.name}")))
	must(nil, "scale", "deployment/browserlab", "--replicas=0")
	must(nil, "wait", "--for=delete", "pod/"+old, "--timeout=45s")
	t.Log(`{"paseo":"two daemons ready; separate files; native WebSocket; clean shutdown","web":"ready","browser":"desktop ready; CDP relay works; workspace read-only; clean shutdown"}`)
}
