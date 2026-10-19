package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/JoverZhang/ya-infra/deployments/internal/host"
	"os"
	"path/filepath"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" {
		fmt.Println("Usage: browserlab build | verify [--desktop-url URL] [--cdp-url URL]\nBuild creates a local image. Verify reads the loopback desktop and CDP endpoints.")
		return nil
	}
	switch args[0] {
	case "build":
		if len(args) != 1 {
			return errors.New("build takes no arguments")
		}
		c, err := host.Load()
		if err != nil {
			return err
		}
		temp, err := os.MkdirTemp("", "ya-infra-browser-build-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(temp)
		if err = host.CopyFile(filepath.Join(c.Source, "apps/browserlab/images/browser/Containerfile"), filepath.Join(temp, "Containerfile")); err != nil {
			return err
		}
		if err = host.BuildHelper(c.Source, "./apps/browserlab/images/browser", filepath.Join(temp, "browserlab")); err != nil {
			return err
		}
		return host.BuildImage(temp, "localhost/ya-infra-browserlab:1.7.22")
	case "verify":
		flags := flag.NewFlagSet("verify", flag.ContinueOnError)
		desktop := flags.String("desktop-url", "http://127.0.0.1:43000/", "local desktop URL")
		cdp := flags.String("cdp-url", "http://127.0.0.1:9222/json/version", "local CDP URL")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("verify takes no positional arguments")
		}
		if err := host.Probe(*desktop, 200); err != nil {
			return err
		}
		return host.Probe(*cdp, 200)
	default:
		return errors.New("unsupported action; use --help")
	}
}
