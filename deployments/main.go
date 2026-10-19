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
		fmt.Println("Usage: ya-infra <bootstrap|kubectl|gateway> [arguments]\n  bootstrap [--local]   Install pinned client tools; optionally reconcile the local server.\n  kubectl ...           Forward native arguments using the configured default context.\n  gateway --help        Manage Cloudflare routes.")
		return nil
	}
	switch args[0] {
	case "bootstrap":
		return bootstrap(args[1:])
	case "kubectl":
		return kubectl(args[1:])
	case "gateway":
		return gateway(args[1:])
	default:
		return fmt.Errorf("unknown command %q; use --help", args[0])
	}
}
