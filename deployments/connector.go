package main

import (
	"errors"
	"fmt"
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
	changed, err := host.PrepareSecret("ya-infra-gateway-"+name, "ya-infra.gateway", name, []byte(token))
	if err != nil {
		return err
	}
	if changed {
		fmt.Println(name + ": connector secret prepared; recreate an existing connector to use a changed token")
	} else {
		fmt.Println(name + ": connector secret unchanged")
	}
	return nil
}
