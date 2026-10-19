package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

var testCLI string

func TestMain(m *testing.M) {
	switch filepath.Base(os.Args[0]) {
	case "sops":
		if os.Getenv("TEST_DECRYPT_FAIL") != "" {
			os.Stderr.WriteString("private-provider-message")
			os.Exit(1)
		}
		data, err := os.ReadFile(os.Args[len(os.Args)-1])
		if err != nil {
			os.Exit(1)
		}
		os.Stdout.Write(data)
		os.Exit(0)
	case "kubectl":
		if len(os.Args) > 1 && os.Args[1] == "--signal" {
			fmt.Println("ready")
			for {
				syscall.Pause()
			}
		}
		if len(os.Args) > 1 && os.Args[1] == "--terminal" {
			info, _ := os.Stdin.Stat()
			if info.Mode()&os.ModeCharDevice == 0 {
				os.Exit(1)
			}
			os.Exit(0)
		}
		if len(os.Args) > 1 && os.Args[1] == "--native-exit" {
			os.Exit(42)
		}
		input, _ := io.ReadAll(os.Stdin)
		cwd, _ := os.Getwd()
		json.NewEncoder(os.Stdout).Encode(map[string]any{"args": os.Args[1:], "cwd": cwd, "stdin": string(input)})
		os.Exit(0)
	}
	for _, key := range []string{"GOCACHE", "GOMODCACHE"} {
		value, err := exec.Command("go", "env", key).Output()
		if err != nil {
			panic(err)
		}
		os.Setenv(key, strings.TrimSpace(string(value)))
	}
	dir, err := os.MkdirTemp("", "ya-infra-tests-")
	if err != nil {
		panic(err)
	}
	testCLI = filepath.Join(dir, "ya-infra")
	cmd := exec.Command("go", "build", "-o", testCLI, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func cliFixture(t *testing.T) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	os.Mkdir(bin, 0700)
	self, _ := os.Executable()
	for _, name := range []string{"sops", "kubectl"} {
		if err := os.Symlink(self, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(dir, "config.sops.yaml"), []byte("source: ./source\nkube_context: fixture\n"), 0600)
	env := append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"), "HOME="+dir, "XDG_CONFIG_HOME="+dir, "XDG_STATE_HOME="+filepath.Join(dir, "state"))
	return dir, env
}

func TestCommandContract(t *testing.T) {
	dir, env := cliFixture(t)
	for _, args := range [][]string{{"kubectl", "apply", "-f", "relative.yaml"}, {"kubectl", "--context=other", "get", "pods"}} {
		cmd := exec.Command(testCLI, args...)
		cmd.Dir = dir
		cmd.Env = env
		cmd.Stdin = strings.NewReader("manifest input")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		var result struct {
			Args       []string
			Cwd, Stdin string
		}
		if err = json.Unmarshal(out, &result); err != nil {
			t.Fatal(string(out))
		}
		if result.Cwd != dir || result.Stdin != "manifest input" {
			t.Fatalf("forwarding: %s", out)
		}
		joined := strings.Join(result.Args, " ")
		if strings.Contains(args[1], "other") {
			if strings.Contains(joined, "fixture") {
				t.Fatal(joined)
			}
		} else if !strings.Contains(joined, "--context=fixture") {
			t.Fatal(joined)
		}
	}
	cmd := exec.Command(testCLI, "kubectl", "--native-exit")
	cmd.Dir = dir
	cmd.Env = env
	if err := cmd.Run(); err == nil || err.(*exec.ExitError).ExitCode() != 42 {
		t.Fatalf("native exit: %v", err)
	}
}

func TestConfigurationFailsBeforeForwarding(t *testing.T) {
	for _, input := range []string{"source: ./source\nunknown: private-value\n", "source: [private-value]\n", "source: ./source\nkube_context: ''\n", "source: ./a\nsource: ./b\nkube_context: fixture\n"} {
		dir, env := cliFixture(t)
		os.WriteFile(filepath.Join(dir, "config.sops.yaml"), []byte(input), 0600)
		cmd := exec.Command(testCLI, "kubectl", "get", "pods")
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err == nil || strings.Contains(string(out), "private-value") {
			t.Fatalf("unsafe configuration error: %v %s", err, out)
		}
	}
	dir, env := cliFixture(t)
	cmd := exec.Command(testCLI, "kubectl", "get", "pods")
	cmd.Dir = dir
	cmd.Env = append(env, "TEST_DECRYPT_FAIL=1")
	out, err := cmd.CombinedOutput()
	if err == nil || strings.Contains(string(out), "private-provider-message") {
		t.Fatalf("decryption: %v %s", err, out)
	}
}

func TestHelpDoesNotNeedConfiguration(t *testing.T) {
	cmd := exec.Command(testCLI, "--help")
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "bootstrap") || strings.Contains(string(out), "svc") {
		t.Fatalf("help: %v %s", err, out)
	}
}

func TestJustKeepsInvocationDirectory(t *testing.T) {
	just, err := exec.LookPath("just")
	if err != nil {
		t.Skip("just is not installed")
	}
	source, _ := os.Getwd()
	dir, env := cliFixture(t)
	for _, file := range []string{filepath.Join(source, "justfile"), filepath.Join(source, "..", "justfile")} {
		cmd := exec.Command(just, "--justfile", file, "kubectl", "get", "pods")
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("just: %v %s", err, out)
		}
		var result struct{ Cwd string }
		if err = json.Unmarshal(out, &result); err != nil || result.Cwd != dir {
			t.Fatalf("just CWD: %s", out)
		}
	}
}

func TestNativeSignalAndTerminal(t *testing.T) {
	dir, env := cliFixture(t)
	cmd := exec.Command(testCLI, "kubectl", "--signal")
	cmd.Dir = dir
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "ready\n" {
		t.Fatalf("child did not start: %q %v", line, err)
	}
	if err = cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err = cmd.Wait(); err == nil {
		t.Fatal("signal was lost")
	}
	if status := cmd.ProcessState.Sys().(syscall.WaitStatus); !status.Signaled() || status.Signal() != syscall.SIGTERM {
		t.Fatal(status)
	}
	script, err := exec.LookPath("script")
	if err != nil {
		t.Skip("terminal check needs util-linux script")
	}
	terminal := exec.Command(script, "--quiet", "--return", "--command", testCLI+" kubectl --terminal", "/dev/null")
	terminal.Dir = dir
	terminal.Env = env
	if out, err := terminal.CombinedOutput(); err != nil {
		t.Fatalf("terminal forwarding: %v %s", err, out)
	}
}
