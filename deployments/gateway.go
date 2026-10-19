package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
)

type routeRecord struct {
	Route         string `json:"route,omitempty"`
	Gateway       string `json:"gateway"`
	Backend       string `json:"backend,omitempty"`
	Origin        string `json:"origin"`
	DesiredOrigin string `json:"desired_origin,omitempty"`
	Cname         string `json:"cname"`
	DNSID         string `json:"dns_id,omitempty"`
	DNSMarker     string `json:"dns_marker,omitempty"`
	Phase         string `json:"phase"`
	VerifiedAt    string `json:"verified_at,omitempty"`
}
type gatewayController struct {
	client   *cloudflareClient
	stateDir string
}
type routePlan struct {
	before, config           map[string]any
	records                  []map[string]any
	dnsTarget                string
	ingressChange, dnsChange bool
	observed                 bool
}

func (g *gatewayController) readState() (map[string]routeRecord, error) {
	state := map[string]routeRecord{}
	data, err := os.ReadFile(filepath.Join(g.stateDir, "routes.json"))
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}
	if json.Unmarshal(data, &state) != nil || state == nil {
		return nil, errors.New("invalid Gateway ownership state; recover the saved state before applying")
	}
	return state, nil
}
func (g *gatewayController) writeState(state map[string]routeRecord) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return host.WritePrivate(filepath.Join(g.stateDir, "routes.json"), append(data, '\n'))
}
func clone(value map[string]any) map[string]any {
	data, _ := json.Marshal(value)
	var result map[string]any
	json.Unmarshal(data, &result)
	return result
}
func (g *gatewayController) plan(r Route) (routePlan, error) {
	var p routePlan
	p.dnsTarget = g.client.gateway.TunnelID + ".cfargotunnel.com"
	var err error
	p.before, err = g.client.configuration()
	if err != nil {
		return p, err
	}
	p.records, err = g.client.records(r.Hostname)
	if err != nil {
		return p, err
	}
	p.observed = true
	original, ok := p.before["config"].(map[string]any)
	if !ok {
		return p, errors.New("Tunnel configuration is missing; configure a catch-all rule first")
	}
	p.config = clone(original)
	ingress, ok := p.config["ingress"].([]any)
	if !ok || len(ingress) == 0 {
		return p, errors.New("Tunnel ingress must contain a final catch-all rule")
	}
	for i, item := range ingress {
		rule, ok := item.(map[string]any)
		if !ok || rule["service"] == nil {
			return p, errors.New("invalid Tunnel ingress rule")
		}
		if i == len(ingress)-1 && (rule["hostname"] != nil || rule["path"] != nil) {
			return p, errors.New("Tunnel ingress must end with a catch-all rule")
		}
	}
	state, err := g.readState()
	if err != nil {
		return p, err
	}
	record, known := state[r.Hostname]
	target := g.client.gateway.TunnelID + ".cfargotunnel.com"
	sameOwner := known && record.Gateway == r.Gateway && record.Cname == target
	if known && !sameOwner {
		return p, errors.New("hostname belongs to another saved Gateway or Tunnel")
	}
	owned := sameOwner && record.Phase != "absent"
	match := -1
	for i, item := range ingress {
		rule := item.(map[string]any)
		if rule["hostname"] != r.Hostname {
			continue
		}
		if match != -1 || rule["path"] != nil {
			return p, errors.New("existing hostname has multiple entries or path rules")
		}
		match = i
		if !owned {
			return p, errors.New("existing hostname has no saved ownership; import verified legacy state before applying")
		}
		if rule["service"] != record.Origin && (record.DesiredOrigin == "" || rule["service"] != record.DesiredOrigin) {
			return p, errors.New("owned hostname has external origin changes; inspect before applying")
		}
	}
	if len(p.records) > 1 {
		return p, errors.New("hostname has multiple DNS records")
	}
	if len(p.records) == 1 {
		dns := p.records[0]
		content, _ := dns["content"].(string)
		if !owned || dns["type"] != "CNAME" || strings.TrimSuffix(strings.ToLower(content), ".") != target || dns["proxied"] != true {
			return p, errors.New("existing DNS is not the owned proxied Tunnel CNAME")
		}
		sameID := record.DNSID != "" && record.DNSID == dns["id"]
		pending := record.DNSMarker != "" && record.DNSMarker == dns["comment"] && (record.Phase == "publishing" || record.Phase == "dns-pending")
		if !sameID && !pending {
			return p, errors.New("DNS identity differs from saved ownership; matching values do not prove ownership")
		}
	}
	if r.State == "absent" {
		if !owned && (match >= 0 || len(p.records) > 0) {
			return p, errors.New("cannot delete an unowned route")
		}
		if match >= 0 {
			ingress = append(ingress[:match], ingress[match+1:]...)
		}
		p.dnsChange = len(p.records) > 0
	} else {
		if match >= 0 {
			ingress[match].(map[string]any)["service"] = r.Service
		} else {
			ingress = append([]any{map[string]any{"hostname": r.Hostname, "service": r.Service}}, ingress...)
		}
		p.dnsChange = len(p.records) == 0
	}
	p.config["ingress"] = ingress
	p.ingressChange = !reflect.DeepEqual(original, p.config)
	return p, nil
}

func (g *gatewayController) apply(r Route) error {
	p, err := g.plan(r)
	if err != nil {
		return err
	}
	if !p.ingressChange && !p.dnsChange {
		state, err := g.readState()
		if err != nil {
			return err
		}
		record, known := state[r.Hostname]
		if !known || record.Phase == "published" || record.Phase == "verified" || record.Phase == "absent" {
			return nil
		}
		// A lost response can leave intent state after the remote write succeeded.
	}
	latest, err := g.client.configuration()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(p.before, latest) {
		return errors.New("Tunnel changed since preview; rerun diff")
	}
	records, err := g.client.records(r.Hostname)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(records, p.records) {
		return errors.New("DNS changed since preview; rerun diff")
	}
	snapshot, _ := json.MarshalIndent(map[string]any{"tunnel": p.before, "dns": p.records}, "", "  ")
	if err = host.WritePrivate(filepath.Join(g.stateDir, "before-last-apply.json"), snapshot); err != nil {
		return err
	}
	state, err := g.readState()
	if err != nil {
		return err
	}
	record := state[r.Hostname]
	if record.Phase == "absent" {
		record = routeRecord{}
	}
	record.Gateway = r.Gateway
	record.Cname = g.client.gateway.TunnelID + ".cfargotunnel.com"
	record.VerifiedAt = ""
	record.Phase = "publishing"
	record.DesiredOrigin = r.Service
	if r.State == "absent" {
		record.Phase = "deleting"
		record.DesiredOrigin = ""
	}
	if record.DNSMarker == "" {
		var nonce [16]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			return err
		}
		record.DNSMarker = "ya-infra/gateway:" + r.Gateway + ":" + hex.EncodeToString(nonce[:])
	}
	state[r.Hostname] = record
	if err = g.writeState(state); err != nil {
		return err
	}
	if p.ingressChange {
		if err = g.client.request("PUT", g.client.configurationPath(), map[string]any{"config": p.config}, nil); err != nil {
			return err
		}
	}
	observed, err := g.client.configuration()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(observed["config"], p.config) {
		return errors.New("Tunnel readback differs; inspect remote changes before retry")
	}
	if r.State == "present" {
		record.Origin = r.Service
		record.DesiredOrigin = ""
		record.Phase = "dns-pending"
	}
	state[r.Hostname] = record
	if err = g.writeState(state); err != nil {
		return err
	}
	// Recheck DNS after the Tunnel write. Preserve changes made by another operator.
	records, err = g.client.records(r.Hostname)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(records, p.records) {
		return errors.New("DNS changed during Tunnel update; inspect before retry")
	}
	zone, err := g.client.zoneID()
	if err != nil {
		return err
	}
	if p.dnsChange {
		if r.State == "absent" {
			id, _ := p.records[0]["id"].(string)
			if id == "" {
				return errors.New("DNS record has no identity")
			}
			if err = g.client.request("DELETE", "/zones/"+zone+"/dns_records/"+id, nil, nil); err != nil {
				return err
			}
		} else {
			body := map[string]any{"type": "CNAME", "name": r.Hostname, "content": record.Cname, "proxied": true, "ttl": 1, "comment": record.DNSMarker}
			var created map[string]any
			if err = g.client.request("POST", "/zones/"+zone+"/dns_records", body, &created); err != nil {
				return err
			}
			record.DNSID, _ = created["id"].(string)
		}
	} else if len(p.records) == 1 {
		record.DNSID, _ = p.records[0]["id"].(string)
	}
	records, err = g.client.records(r.Hostname)
	if err != nil {
		return err
	}
	if r.State == "absent" {
		if len(records) != 0 {
			return errors.New("DNS deletion readback failed")
		}
		record.Phase = "absent"
	} else {
		if len(records) != 1 || records[0]["id"] != record.DNSID || records[0]["type"] != "CNAME" || records[0]["content"] != record.Cname || records[0]["proxied"] != true {
			return errors.New("DNS publication readback failed")
		}
		record.Phase = "published"
	}
	state[r.Hostname] = record
	return g.writeState(state)
}

func checkCredentials(c *cloudflareClient, routes []Route) error {
	failed := false
	check := func(label string, operation func() error) bool {
		if err := operation(); err != nil {
			prefix := "FAIL"
			if e, ok := err.(*apiError); ok && e.blocked != "" {
				prefix = "BLOCKED"
			}
			fmt.Printf("%s %s: %v\n", prefix, label, err)
			failed = true
			return false
		}
		fmt.Println("PASS", label)
		return true
	}
	check("Token validity", c.verifyToken)
	zoneOK := check("Zone / Zone / Read", func() error { _, err := c.zoneID(); return err })
	check("Account / cloudflared / Read", func() error { _, err := c.configuration(); return err })
	if zoneOK {
		for _, r := range routes {
			check("Zone / DNS / Read ("+r.Hostname+")", func() error { _, err := c.records(r.Hostname); return err })
		}
	} else {
		fmt.Println("SKIP Zone / DNS / Read: resolve the Zone lookup result first")
	}
	fmt.Println("NOT TESTED write permissions: Account / cloudflared / Edit and Zone / DNS / Edit")
	if failed {
		return errors.New("Cloudflare credential checks failed; use the endpoint-specific results above")
	}
	return nil
}

func gateway(args []string) error {
	if len(args) == 0 || args[0] == "--help" {
		fmt.Println("Usage: ya-infra gateway <diff|apply|status|verify|check-credentials|setup> [hostname]\nRoutes use explicit service URLs in gateway/config.sops.yaml.\nsetup --edit opens SOPS for credential editing. Other commands do not prompt.\nMissing declarations preserve remote resources; state: absent requests owned deletion.")
		return nil
	}
	action := args[0]
	allowed := map[string]bool{"diff": true, "apply": true, "status": true, "verify": true, "check-credentials": true, "setup": true}
	if !allowed[action] {
		return errors.New("unknown Gateway action; use gateway --help")
	}
	config, err := host.Load()
	if err != nil {
		return err
	}
	if action == "setup" {
		flags := flag.NewFlagSet("gateway setup", flag.ContinueOnError)
		edit := flags.Bool("edit", false, "edit credentials with SOPS")
		if err = flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("setup takes no hostname")
		}
		fmt.Println("Create a Cloudflare management token with Account / Cloudflare One Connector: cloudflared / Edit, Zone / DNS / Edit, and Zone / Zone / Read. Limit it to the configured account and zone. Check the entire client-IP filter rule. Use https://dash.cloudflare.com/profile/api-tokens/")
		if *edit {
			return host.Run("sops", "--config", filepath.Join(config.Source, ".sops.yaml"), filepath.Join(config.Source, "gateway", "config.sops.yaml"))
		}
		return nil
	}
	if len(args) > 2 {
		return errors.New("expected at most one hostname")
	}
	declarations, err := loadGateway(config.Source)
	if err != nil {
		return err
	}
	grouped := map[string][]Route{}
	for _, r := range declarations.Routes {
		if len(args) == 1 || args[1] == r.Hostname {
			grouped[r.Gateway] = append(grouped[r.Gateway], r)
		}
	}
	if len(args) == 2 && len(grouped) == 0 {
		return errors.New("unknown route hostname")
	}
	if action == "check-credentials" && len(args) == 1 {
		for name := range declarations.Gateways {
			if _, ok := grouped[name]; !ok {
				grouped[name] = nil
			}
		}
	}
	names := []string{}
	for name := range grouped {
		names = append(names, name)
	}
	sort.Strings(names)
	var failures []error
	for _, name := range names {
		c := newClient(declarations.Gateways[name])
		g := &gatewayController{client: c, stateDir: filepath.Join(host.XDG("XDG_STATE_HOME", ".local/state"), "infra", "gateway", name)}
		if action == "check-credentials" {
			if err = checkCredentials(c, grouped[name]); err != nil {
				failures = append(failures, err)
			}
			continue
		}
		var lock *os.File
		if action == "apply" {
			if err = os.MkdirAll(g.stateDir, 0700); err != nil {
				return err
			}
			if err = os.Chmod(g.stateDir, 0700); err != nil {
				return err
			}
			lock, err = os.OpenFile(filepath.Join(g.stateDir, "manage.lock"), os.O_CREATE|os.O_RDWR, 0600)
			if err != nil {
				return err
			}
			if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
				lock.Close()
				return errors.New("another Gateway operation holds the lock")
			}
		}
		for _, r := range grouped[name] {
			var operationErr error
			switch action {
			case "apply":
				operationErr = g.apply(r)
				if operationErr == nil {
					fmt.Println(r.Hostname + ": applied")
				}
			case "verify":
				operationErr = verifyRoute(r)
				if operationErr == nil {
					fmt.Println(r.Hostname + ": public HTTP check passed")
				}
			default:
				var p routePlan
				p, operationErr = g.plan(r)
				reportRoute(os.Stdout, action, r, p, operationErr)
			}
			if operationErr != nil {
				failures = append(failures, fmt.Errorf("%s: %w", r.Hostname, operationErr))
				if action == "apply" {
					break
				}
			}
		}
		if lock != nil {
			lock.Close()
		}
		if len(failures) > 0 && action == "apply" {
			break
		}
	}
	return errors.Join(failures...)
}
func verifyRoute(r Route) error {
	if r.State == "absent" {
		return errors.New("public verification requires state: present")
	}
	return host.Probe("https://"+r.Hostname+r.ProbePath, r.ProbeStatus)
}

// Print only managed fields. Provider options and URL credentials stay private.
func reportRoute(out io.Writer, action string, r Route, p routePlan, conflict error) {
	if !p.observed {
		fmt.Fprintln(out, r.Hostname+": remote state unknown")
		return
	}
	origins := []string{}
	if config, ok := p.before["config"].(map[string]any); ok {
		if rules, ok := config["ingress"].([]any); ok {
			for _, entry := range rules {
				rule, ok := entry.(map[string]any)
				if !ok || rule["hostname"] != r.Hostname {
					continue
				}
				raw, _ := rule["service"].(string)
				u, err := url.Parse(raw)
				value := "[non-HTTP origin]"
				if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
					value = u.Scheme + "://" + u.Host
				}
				origins = append(origins, value)
			}
		}
	}
	desired := r.Service
	if r.State == "absent" {
		desired = "absent"
	}
	fmt.Fprintf(out, "%s: origin observed=%q desired=%q\n", r.Hostname, origins, desired)
	fmt.Fprintf(out, "  DNS observed_records=%d", len(p.records))
	for _, record := range p.records {
		kind, _ := record["type"].(string)
		if kind == "CNAME" {
			content, _ := record["content"].(string)
			if !dnsName.MatchString(strings.TrimSuffix(content, ".")) {
				content = "[invalid hostname]"
			}
			fmt.Fprintf(out, " CNAME=%q proxied=%t", content, record["proxied"] == true)
		} else {
			fmt.Fprint(out, " [non-CNAME record]")
		}
	}
	if r.State == "absent" {
		fmt.Fprintln(out, " desired=absent")
	} else {
		fmt.Fprintf(out, " desired=CNAME:%s proxied=true\n", p.dnsTarget)
	}
	if conflict != nil {
		fmt.Fprintln(out, "  conflict: inspect observed state before apply")
	} else if action == "diff" {
		fmt.Fprintf(out, "  changes: ingress=%t DNS=%t\n", p.ingressChange, p.dnsChange)
	}
}
