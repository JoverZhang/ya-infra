package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
)

// Privileged commands run as this test binary and touch only the fixture root.
func runBootstrapFixture(name string, args []string) error {
	root := os.Getenv("TEST_BOOTSTRAP_ROOT")
	if root == "" {
		return errors.New("bootstrap fixture root is required")
	}
	if name == "sudo" {
		name, args = args[0], args[1:]
	}
	resolve := func(path string) (string, error) {
		switch path {
		case "/etc/rancher/k3s/ya-infra.yaml":
			return filepath.Join(root, "server.yaml"), nil
		case "/etc/rancher/k3s/ya-infra.yaml.restart-pending":
			return filepath.Join(root, "restart-pending"), nil
		case "/etc/rancher/k3s/k3s.yaml":
			return filepath.Join(root, "server-kubeconfig"), nil
		}
		return "", fmt.Errorf("unexpected privileged path: %s", path)
	}
	record := func(operation string) error {
		log, err := os.OpenFile(filepath.Join(root, "operations"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, writeErr := fmt.Fprintln(log, operation)
		closeErr := log.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return err
		}
		if os.Getenv("TEST_BOOTSTRAP_FAIL") == operation {
			return fmt.Errorf("fixture failure: %s", operation)
		}
		return nil
	}
	switch name {
	case "k3s":
		fmt.Println(serverVersion)
		return nil
	case "systemctl":
		_, missing := os.Stat(filepath.Join(root, "service-missing"))
		if args[0] == "show" {
			if missing == nil {
				fmt.Println("not-found")
			} else {
				fmt.Println("loaded")
			}
			return nil
		}
		if missing == nil {
			return errors.New("k3s.service does not exist")
		}
		if args[0] == "is-active" {
			return nil
		}
		if args[0] == "restart" || args[0] == "start" {
			return record(args[0])
		}
	case "env":
		if len(args) != 5 || args[0] != "INSTALL_K3S_VERSION="+serverVersion || args[3] != "sh" {
			return fmt.Errorf("unexpected installer command: %v", args)
		}
		if err := record("install-service"); err != nil {
			return err
		}
		return os.Remove(filepath.Join(root, "service-missing"))
	case "test", "cat", "touch", "rm":
		path, err := resolve(args[len(args)-1])
		if err != nil {
			return err
		}
		switch name {
		case "test":
			_, err = os.Stat(path)
			return err
		case "cat":
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			_, err = os.Stdout.Write(data)
			return err
		case "touch":
			if err := record("save-intent"); err != nil {
				return err
			}
			return os.WriteFile(path, nil, 0600)
		case "rm":
			if err := record("clear-intent"); err != nil {
				return err
			}
			return os.Remove(path)
		}
	case "install":
		if args[0] == "-d" {
			return record("create-config-directory")
		}
		if err := record("install-config"); err != nil {
			return err
		}
		target, err := resolve(args[len(args)-1])
		if err != nil {
			return err
		}
		data, err := os.ReadFile(args[len(args)-2])
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	}
	return fmt.Errorf("unexpected fixture command: %s %v", name, args)
}

type installerFixture struct{}

func (installerFixture) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.String() != "https://raw.githubusercontent.com/k3s-io/k3s/"+serverVersion+"/install.sh" {
		return nil, fmt.Errorf("unexpected fixture download: %s", request.URL)
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("fixture installer")), Header: make(http.Header)}, nil
}

func bootstrapFixture(t *testing.T) (string, []string) {
	t.Helper()
	root, env := cliFixture(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sudo", "systemctl", "k3s", "bootstrap-local"} {
		if err := os.Symlink(self, filepath.Join(root, "bin", name)); err != nil {
			t.Fatal(err)
		}
	}
	dataRoot := filepath.Join(root, "data")
	t.Setenv("XDG_DATA_HOME", dataRoot)
	before, err := localConfiguration(serverOptions{Extra: map[string]any{"node-name": "old"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	localConfig, err := os.ReadFile("testdata/kubeconfig-local.yaml")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"server.yaml":       before,
		"server-kubeconfig": localConfig,
		"source/kubernetes/bootstrap/local.sops.yaml": []byte("node-name: new\n"),
	}
	for path, data := range files {
		if err := host.WritePrivate(filepath.Join(root, path), data); err != nil {
			t.Fatal(err)
		}
	}
	env = append(env, "TEST_BOOTSTRAP_ROOT="+root, "XDG_DATA_HOME="+dataRoot, "KUBECONFIG="+filepath.Join(root, "kubeconfig"))
	return root, env
}

func runLocalBootstrap(root string, env []string) ([]byte, error) {
	command := exec.Command(filepath.Join(root, "bin", "bootstrap-local"))
	command.Dir = root
	command.Env = env
	return command.CombinedOutput()
}

func TestBootstrapRetriesInterruptedChanges(t *testing.T) {
	for _, failure := range []string{"save-intent", "install-config", "restart", "clear-intent"} {
		t.Run(failure, func(t *testing.T) {
			root, env := bootstrapFixture(t)
			if output, err := runLocalBootstrap(root, append(env, "TEST_BOOTSTRAP_FAIL="+failure)); err == nil {
				t.Fatalf("expected failure at %s: %s", failure, output)
			}
			if output, err := runLocalBootstrap(root, env); err != nil {
				t.Fatalf("retry: %v %s", err, output)
			}
			operations, err := os.ReadFile(filepath.Join(root, "operations"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(string(operations), "restart\nclear-intent\n") {
				t.Fatalf("retry did not apply configuration to the running service: %s", operations)
			}
			if _, err := os.Stat(filepath.Join(root, "restart-pending")); !os.IsNotExist(err) {
				t.Fatalf("completed bootstrap retained pending intent: %v", err)
			}
			if output, err := runLocalBootstrap(root, env); err != nil {
				t.Fatalf("unchanged bootstrap: %v %s", err, output)
			}
			after, err := os.ReadFile(filepath.Join(root, "operations"))
			if err != nil || string(after) != string(operations) {
				t.Fatalf("completed retry was not idempotent: %v %s", err, after)
			}
			t.Logf("%s failure -> retry applied configuration -> unchanged run made no system writes", failure)
		})
	}
}

func TestBootstrapRejectsInvalidKubeconfigBeforeChanges(t *testing.T) {
	for _, input := range []string{"null\n", "[]\n", "contexts: [\n", "clusters: {name: other}\n", "users: invalid\n", "contexts: [null]\n", "clusters: [{name: other, cluster: []}]\n"} {
		t.Run(strings.TrimSpace(input), func(t *testing.T) {
			root, env := bootstrapFixture(t)
			if err := os.WriteFile(filepath.Join(root, "kubeconfig"), []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			output, err := runLocalBootstrap(root, env)
			if err == nil || strings.Contains(string(output), "panic:") || !strings.Contains(string(output), "kubeconfig") {
				t.Fatalf("expected a kubeconfig validation error: %v %s", err, output)
			}
			if operations, err := os.ReadFile(filepath.Join(root, "operations")); !os.IsNotExist(err) {
				t.Fatalf("invalid kubeconfig reached system changes: %v %s", err, operations)
			}
		})
	}
}

func TestBootstrapFinishesInterruptedInstallation(t *testing.T) {
	root, env := bootstrapFixture(t)
	for _, path := range []string{"service-missing", "restart-pending"} {
		if err := os.WriteFile(filepath.Join(root, path), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if output, err := runLocalBootstrap(root, append(env, "TEST_BOOTSTRAP_FAIL=install-service")); err == nil {
		t.Fatalf("expected installer failure: %s", output)
	}
	if output, err := runLocalBootstrap(root, env); err != nil {
		t.Fatalf("resume interrupted installation: %v %s", err, output)
	}
	before, err := os.ReadFile(filepath.Join(root, "operations"))
	if err != nil || !strings.HasSuffix(string(before), "install-service\nclear-intent\n") {
		t.Fatalf("installer did not complete: %v %s", err, before)
	}
	if output, err := runLocalBootstrap(root, env); err != nil {
		t.Fatalf("unchanged bootstrap: %v %s", err, output)
	}
	after, err := os.ReadFile(filepath.Join(root, "operations"))
	if err != nil || string(after) != string(before) {
		t.Fatalf("completed installation was not idempotent: %v %s", err, after)
	}
	t.Log("binary without service -> failed installation -> successful retry -> no further writes")
}

func TestBootstrapPreservesEmptyUserConfiguration(t *testing.T) {
	root, env := bootstrapFixture(t)
	if err := os.WriteFile(filepath.Join(root, "kubeconfig"), []byte("users: [{name: anonymous, user: {}}]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if output, err := runLocalBootstrap(root, env); err != nil {
			t.Fatalf("bootstrap attempt %d: %v %s", attempt, err, output)
		}
	}
}
