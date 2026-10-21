//go:build integration

package checks

import (
	"bytes"
	"encoding/base64"
	"fmt"
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
			for _, args := range [][]string{{"get", "events", "--sort-by=.lastTimestamp"}, {"logs", "deployment/browserlab", "--all-containers", "--tail=80"}, {"logs", "deployment/tbg", "--all-containers", "--tail=80"}} {
				out, err := run(nil, args...)
				t.Logf("diagnostic %v: %v\n%s", args, err, out)
			}
		}
		if out, err := run(nil, "delete", "namespace", namespace, "--wait=true", "--timeout=90s"); err != nil {
			t.Errorf("cleanup namespace/%s failed; inspect remaining resources: %v\n%s", namespace, err, out)
		}
	}()
	apply := func(resource any) {
		t.Helper()
		data, err := yaml.Marshal(resource)
		if err != nil {
			t.Fatal(err)
		}
		must(data, "apply", "--server-side", "-f", "-")
	}
	load := func(app, file string) []deployment {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "apps", app, "kubernetes", file))
		if err != nil {
			t.Fatal(err)
		}
		return decodeDocuments[deployment](t, data)
	}
	disposable := func(resource *deployment) *podSpec {
		one := 1
		resource.Spec.Replicas = &one
		pod := &resource.Spec.Template.Spec
		pod.NodeSelector = nil
		for i := range pod.Volumes {
			volume := &pod.Volumes[i]
			if volume.PersistentVolumeClaim != nil || volume.HostPath != nil {
				volume.PersistentVolumeClaim = nil
				volume.HostPath = nil
				volume.EmptyDir = &map[string]any{}
			}
		}
		return pod
	}
	paseo := load("paseo", "deployment.yaml")
	for _, resource := range paseo {
		pod := disposable(&resource)
		if resource.Metadata.Name == "paseo-demo" {
			pod.InitContainers[0].Args = []string{"--new"}
			daemon := &pod.Containers[0]
			daemon.EnvFrom = nil
			password := "disposable-test-password"
			daemon.Env = append(daemon.Env, environmentVariable{Name: "PASEO_PASSWORD", Value: &password})
			raw, err := yaml.Marshal(resource)
			if err != nil {
				t.Fatal(err)
			}
			var second deployment
			if err := yaml.Unmarshal(bytes.ReplaceAll(raw, []byte("paseo-demo"), []byte("paseo-second")), &second); err != nil {
				t.Fatal(err)
			}
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
	pod := disposable(&browser)
	init := pod.InitContainers
	image := init[0].Image
	marker := fixtureResource[container](t, "browser-marker.yaml", "FIXTURE_IMAGE", image)
	pod.InitContainers = append([]container{marker}, init...)
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
	// Use the exact TBG image, synthetic SQLite data, and no Telegram network access.
	database := filepath.Join(t.TempDir(), "gateway.sqlite")
	command(t, os.Environ(), "sqlite3", database, "VACUUM;")
	dbBytes, err := os.ReadFile(database)
	if err != nil {
		t.Fatal(err)
	}
	apply(fixtureResource[map[string]any](t, "tbg-server.yaml"))
	apply(map[string]any{"apiVersion": "v1", "kind": "Secret", "metadata": map[string]any{"name": "database-fixture"}, "data": map[string]string{"gateway.sqlite": base64.StdEncoding.EncodeToString(dbBytes)}})
	tbg := load("tbg", "deployment.yaml")[0]
	pod = disposable(&tbg)
	pod.HostAliases = []hostAlias{{IP: "127.0.0.1", Hostnames: []string{"api.telegram.org"}}}
	gateway := &pod.Containers[0]
	proxy, noProxy := "http://127.0.0.1:1", ""
	gateway.Env = []environmentVariable{{Name: "HTTPS_PROXY", Value: &proxy}, {Name: "https_proxy", Value: &proxy}, {Name: "NO_PROXY", Value: &noProxy}}
	apply(tbg)
	rejected := false
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		out, _ := run(nil, "get", "pods", "-l", "app=tbg", "-o", "jsonpath={.items[0].status.initContainerStatuses[0].lastState.terminated.exitCode}")
		if string(out) == "1" {
			rejected = true
			break
		}
	}
	if !rejected {
		t.Fatal("empty database did not fail the init check")
	}
	must(nil, "scale", "deployment/tbg", "--replicas=0")
	must(nil, "wait", "--for=delete", "pod", "-l", "app=tbg", "--timeout=45s")
	pod.Volumes = append(pod.Volumes, volume{Name: "database-fixture", Secret: &secretVolume{SecretName: "database-fixture"}})
	restore := fixtureResource[container](t, "tbg-restore.yaml", "FIXTURE_IMAGE", gateway.Image)
	pod.InitContainers = append([]container{restore}, pod.InitContainers...)
	apply(tbg)
	must(nil, "rollout", "status", "deployment/tbg", "--timeout=90s")
	// A separate client in the same host network checks HTTP 405 without a notification.
	probe := fixtureResource[map[string]any](t, "tbg-probe.yaml", "FIXTURE_IMAGE", image)
	apply(probe)
	must(nil, "wait", "--for=jsonpath={.status.phase}=Succeeded", "pod/tbg-probe", "--timeout=60s")
	if got := strings.TrimSpace(string(must(nil, "logs", "tbg-probe"))); got != "405" {
		t.Fatal("TBG protocol probe", got)
	}
	old = strings.TrimSpace(string(must(nil, "get", "pods", "-l", "app=tbg", "-o", "jsonpath={.items[0].metadata.name}")))
	must(nil, "scale", "deployment/tbg", "--replicas=0")
	must(nil, "wait", "--for=delete", "pod/"+old, "--timeout=45s")

	t.Log(`{"paseo":"two daemons ready; separate files; native WebSocket; clean shutdown","web":"ready","tbg":"empty database rejected; synthetic restore ready; Secret mount; loopback HTTP 405; clean shutdown; Telegram blocked","browser":"desktop ready; CDP relay works; workspace read-only; clean shutdown"}`)
}
