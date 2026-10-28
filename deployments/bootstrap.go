package main

import (
	"debug/buildinfo"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
)

var clientTools = []struct{ name, module, version string }{
	{"sops", "github.com/getsops/sops/v3/cmd/sops", "v3.13.3"},
	{"age", "filippo.io/age/cmd/age", "v1.2.1"},
	{"age-keygen", "filippo.io/age/cmd/age-keygen", "v1.2.1"},
}

func bootstrap(args []string) error {
	flags := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("bootstrap takes no positional arguments")
	}
	dir := filepath.Join(host.XDG("XDG_DATA_HOME", ".local/share"), "ya-infra", "bin")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	for _, tool := range clientTools {
		binary := filepath.Join(dir, tool.name)
		info, err := buildinfo.ReadFile(binary)
		if err == nil && info.Main.Version == tool.version {
			continue
		}
		fmt.Println("Installing", tool.name, tool.version)
		cmd := exec.Command("go", "install", tool.module+"@"+tool.version)
		cmd.Env = append(os.Environ(), "GOBIN="+dir)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install %s: %w", tool.name, err)
		}
	}
	fmt.Println("Client tools:", dir)
	return nil
}
