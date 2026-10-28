package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
)

func prepareConnectorSecrets(config gatewayConfig, selected string) error {
	if _, ok := config.Gateways[selected]; selected != "" && !ok {
		return errors.New("unknown gateway")
	}
	var names []string
	for name, gateway := range config.Gateways {
		if selected != "" && name != selected {
			continue
		}
		// Validate all selected credentials before changing the runtime store.
		if len(gateway.TunnelToken) < 20 || strings.ContainsAny(gateway.TunnelToken, " \t\r\n") {
			return fmt.Errorf("%s: prepare requires a tunnel_token", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	rootless, err := exec.Command("podman", "info", "--format", "{{.Host.Security.Rootless}}").Output()
	if err != nil {
		return fmt.Errorf("inspect Podman runtime: %w", err)
	}
	if strings.TrimSpace(string(rootless)) != "true" {
		return errors.New("connector preparation requires rootless Podman")
	}
	for _, name := range names {
		if err := prepareConnectorSecret(name, config.Gateways[name].TunnelToken); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func prepareConnectorSecret(name, token string) error {
	lock, err := lockGateway(filepath.Join(host.XDG("XDG_STATE_HOME", ".local/state"), "infra", "gateway", name))
	if err != nil {
		return err
	}
	defer lock.Close()
	secret := "ya-infra-gateway-" + name
	sum := sha256.Sum256([]byte(token))
	digest := hex.EncodeToString(sum[:])
	exists := exec.Command("podman", "secret", "exists", secret).Run()
	if exists != nil {
		var exit *exec.ExitError
		if !errors.As(exists, &exit) || exit.ExitCode() != 1 {
			return fmt.Errorf("check connector secret: %w", exists)
		}
	} else {
		data, err := exec.Command("podman", "secret", "inspect", secret).Output()
		if err != nil {
			return fmt.Errorf("inspect connector secret: %w", err)
		}
		var records []struct {
			Spec struct{ Labels map[string]string }
		}
		if err := json.Unmarshal(data, &records); err != nil {
			return fmt.Errorf("decode connector secret metadata: %w", err)
		}
		if len(records) != 1 || records[0].Spec.Labels["ya-infra.gateway"] != name {
			return errors.New("connector secret is not owned by this Gateway; inspect it before adoption")
		}
		if records[0].Spec.Labels["ya-infra.sha256"] == digest {
			fmt.Println(name + ": connector secret unchanged")
			return nil
		}
	}
	args := []string{"secret", "create", "--label", "ya-infra.gateway=" + name, "--label", "ya-infra.sha256=" + digest}
	if exists == nil {
		args = append(args, "--replace")
	}
	cmd := exec.Command("podman", append(args, secret, "-")...)
	cmd.Stdin = strings.NewReader(token)
	// Keep provider output private. The token goes only through stdin.
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("write connector secret: %w", err)
	}
	fmt.Println(name + ": connector secret prepared; recreate an existing connector to use a changed token")
	return nil
}
