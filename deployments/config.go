package main

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
)

type Gateway struct {
	Zone      string `yaml:"zone"`
	AccountID string `yaml:"account_id"`
	TunnelID  string `yaml:"tunnel_id"`
	APIToken  string `yaml:"api_token"`
}

type Route struct {
	Gateway     string `yaml:"gateway"`
	Hostname    string `yaml:"hostname"`
	Service     string `yaml:"service"`
	State       string `yaml:"state"`
	ProbePath   string `yaml:"probe_path,omitempty"`
	ProbeStatus int    `yaml:"probe_status,omitempty"`
}

type gatewayConfig struct {
	Gateways map[string]Gateway `yaml:"gateways"`
	Routes   []Route            `yaml:"routes"`
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
var dnsName = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var accountID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var tunnelID = regexp.MustCompile(`^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$`)

func loadGateway(source string) (gatewayConfig, error) {
	var config gatewayConfig
	if err := host.Decrypt(filepath.Join(source, "gateway", "config.sops.yaml"), &config); err != nil {
		return config, err
	}
	if len(config.Gateways) == 0 {
		return config, errors.New("no gateways declared")
	}
	tunnels := map[string]bool{}
	for name, g := range config.Gateways {
		if !identifier.MatchString(name) || !dnsName.MatchString(g.Zone) || len(g.Zone) > 253 || !accountID.MatchString(g.AccountID) || !tunnelID.MatchString(g.TunnelID) {
			return config, errors.New("invalid gateway name, zone, account, or Tunnel identity")
		}
		if len(g.APIToken) < 20 || strings.ContainsAny(g.APIToken, " \t\r\n") {
			return config, fmt.Errorf("%s: invalid management credential", name)
		}
		identity := g.AccountID + "/" + g.TunnelID
		if tunnels[identity] {
			return config, errors.New("one Tunnel must have one gateway owner")
		}
		tunnels[identity] = true
	}
	seen := map[string]bool{}
	for i := range config.Routes {
		r := &config.Routes[i]
		g, ok := config.Gateways[r.Gateway]
		if !ok || !dnsName.MatchString(r.Hostname) || len(r.Hostname) > 253 || !(r.Hostname == g.Zone || strings.HasSuffix(r.Hostname, "."+g.Zone)) {
			return config, errors.New("route must use an exact hostname in its gateway zone")
		}
		if seen[r.Hostname] {
			return config, errors.New("duplicate route hostname")
		}
		seen[r.Hostname] = true
		if r.State != "present" && r.State != "absent" {
			return config, fmt.Errorf("%s: state must be present or absent", r.Hostname)
		}
		u, err := url.Parse(r.Service)
		if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return config, fmt.Errorf("%s: service must be an HTTP(S) origin URL", r.Hostname)
		}
		if r.ProbePath == "" {
			r.ProbePath = "/"
		}
		if !strings.HasPrefix(r.ProbePath, "/") || strings.HasPrefix(r.ProbePath, "//") || strings.ContainsAny(r.ProbePath, "\r\n") {
			return config, errors.New("probe_path must be an absolute URL path")
		}
		if r.ProbeStatus == 0 {
			r.ProbeStatus = 200
		}
		if r.ProbeStatus < 100 || r.ProbeStatus > 599 {
			return config, errors.New("invalid probe_status")
		}
	}
	return config, nil
}
