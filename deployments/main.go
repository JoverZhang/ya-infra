package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() > 0 {
			os.Exit(e.ExitCode())
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Println("Usage: ya-infra <bootstrap|gateway> [arguments]\n  bootstrap             Install pinned SOPS and age tools.\n  gateway --help        Manage Cloudflare routes.\nUse just compose <app> ... for native container commands.")
		return nil
	}
	switch args[0] {
	case "bootstrap":
		return bootstrap(args[1:])
	case "gateway":
		return gateway(args[1:])
	default:
		return fmt.Errorf("unknown command %q; use --help", args[0])
	}
}
