package main

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
)

func kubectl(args []string) error {
	config, err := host.Load()
	if err != nil {
		return err
	}
	explicit := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--context" || strings.HasPrefix(arg, "--context=") {
			explicit = true
			break
		}
	}
	if !explicit {
		// Put the default before the argument terminator, including for exec commands.
		at := len(args)
		for i, arg := range args {
			if arg == "--" {
				at = i
				break
			}
		}
		forwarded := append([]string{}, args[:at]...)
		forwarded = append(forwarded, "--context="+config.Context)
		args = append(forwarded, args[at:]...)
	}
	path, err := exec.LookPath("kubectl")
	if err != nil {
		return err
	}
	return syscall.Exec(path, append([]string{"kubectl"}, args...), os.Environ())
}
