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
	before                   tunnelConfiguration
	config                   tunnelConfig
	records                  []dnsRecord
	dnsTarget                string
	ingressChange, dnsChange bool
	observed                 bool
}

func (g *gatewayController) readState() (map[string]routeRecord, error) {
	state := map[string]routeRecord{}
	path := filepath.Join(g.stateDir, "routes.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode Gateway ownership state %q: %w", path, err)
	}
	if state == nil {
		return nil, fmt.Errorf("Gateway ownership state %q must be a JSON object", path)
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
func (g *gatewayController) plan(route Route) (routePlan, error) {
	var plan routePlan
	plan.dnsTarget = g.client.gateway.TunnelID + ".cfargotunnel.com"
	var err error
	plan.before, err = g.client.configuration()
	if err != nil {
		return plan, err
	}
	plan.records, err = g.client.records(route.Hostname)
	if err != nil {
		return plan, err
	}
	plan.observed = true
	if plan.before.Config == nil {
		return plan, errors.New("Tunnel configuration is missing; configure a catch-all rule first")
	}
	original := *plan.before.Config
	plan.config = original
	// Only the managed service value changes; provider options remain untouched.
	ingress := append([]ingressRule(nil), original.Ingress...)
	if len(ingress) == 0 {
		return plan, errors.New("Tunnel ingress must contain a final catch-all rule")
	}
	for i, rule := range ingress {
		if rule.Service == "" {
			return plan, errors.New("invalid Tunnel ingress rule")
		}
		if i == len(ingress)-1 && (rule.Hostname != nil || rule.Path != nil) {
			return plan, errors.New("Tunnel ingress must end with a catch-all rule")
		}
	}
	state, err := g.readState()
	if err != nil {
		return plan, err
	}
	record, known := state[route.Hostname]
	target := g.client.gateway.TunnelID + ".cfargotunnel.com"
	sameOwner := known && record.Gateway == route.Gateway && record.Cname == target
	if known && !sameOwner {
		return plan, errors.New("hostname belongs to another saved Gateway or Tunnel")
	}
	owned := sameOwner && record.Phase != "absent"
	match := -1
	for i, rule := range ingress {
		if rule.Hostname == nil || *rule.Hostname != route.Hostname {
			continue
		}
		if match != -1 || rule.Path != nil {
			return plan, errors.New("existing hostname has multiple entries or path rules")
		}
		match = i
		if !owned {
			return plan, errors.New("existing hostname has no saved ownership; import verified legacy state before applying")
		}
		if rule.Service != record.Origin && (record.DesiredOrigin == "" || rule.Service != record.DesiredOrigin) {
			return plan, errors.New("owned hostname has external origin changes; inspect before applying")
		}
	}
	if len(plan.records) > 1 {
		return plan, errors.New("hostname has multiple DNS records")
	}
	if len(plan.records) == 1 {
		dns := plan.records[0]
		if !owned || dns.Type != "CNAME" || strings.TrimSuffix(strings.ToLower(dns.Content), ".") != target || !dns.Proxied {
			return plan, errors.New("existing DNS is not the owned proxied Tunnel CNAME")
		}
		sameID := record.DNSID != "" && record.DNSID == dns.ID
		pending := record.DNSMarker != "" && record.DNSMarker == dns.Comment && (record.Phase == "publishing" || record.Phase == "dns-pending")
		if dns.ID == "" || (!sameID && !pending) {
			return plan, errors.New("DNS identity differs from saved ownership; matching values do not prove ownership")
		}
	}
	if route.State == "absent" {
		if !owned && (match >= 0 || len(plan.records) > 0) {
			return plan, errors.New("cannot delete an unowned route")
		}
		if match >= 0 {
			ingress = append(ingress[:match], ingress[match+1:]...)
		}
		plan.dnsChange = len(plan.records) > 0
	} else {
		if match >= 0 {
			ingress[match].Service = route.Service
		} else {
			ingress = append([]ingressRule{{Hostname: &route.Hostname, Service: route.Service}}, ingress...)
		}
		plan.dnsChange = len(plan.records) == 0
	}
	plan.config.Ingress = ingress
	plan.ingressChange = !reflect.DeepEqual(original, plan.config)
	return plan, nil
}

func (g *gatewayController) apply(route Route) error {
	plan, err := g.plan(route)
	if err != nil {
		return err
	}
	if !plan.ingressChange && !plan.dnsChange {
		state, err := g.readState()
		if err != nil {
			return err
		}
		record, known := state[route.Hostname]
		if known && route.State == "absent" && record.Phase != "absent" {
			record.Phase = "absent"
			record.DesiredOrigin = ""
			state[route.Hostname] = record
			return g.writeState(state)
		}
		if !known || record.Phase == "published" || record.Phase == "verified" || record.Phase == "absent" {
			return nil
		}
		// A lost response can leave intent state after the remote write succeeded.
	}
	latest, err := g.client.configuration()
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(plan.before, latest) {
		return errors.New("Tunnel changed since preview; rerun diff")
	}
	records, err := g.client.records(route.Hostname)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(records, plan.records) {
		return errors.New("DNS changed since preview; rerun diff")
	}
	snapshot, err := json.MarshalIndent(struct {
		Tunnel tunnelConfiguration `json:"tunnel"`
		DNS    []dnsRecord         `json:"dns"`
	}{plan.before, plan.records}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Gateway recovery snapshot: %w", err)
	}
	if err = host.WritePrivate(filepath.Join(g.stateDir, "before-last-apply.json"), snapshot); err != nil {
		return err
	}
	state, err := g.readState()
	if err != nil {
		return err
	}
	record := state[route.Hostname]
	if record.Phase == "absent" {
		record = routeRecord{}
	}
	record.Gateway = route.Gateway
	record.Cname = g.client.gateway.TunnelID + ".cfargotunnel.com"
	record.VerifiedAt = ""
	// plan verified this identity, including recovery through the creation marker.
	// Persist it before changing phase so a failed deletion remains retryable.
	if len(plan.records) == 1 {
		record.DNSID = plan.records[0].ID
	}
	record.Phase = "publishing"
	record.DesiredOrigin = route.Service
	if route.State == "absent" {
		record.Phase = "deleting"
		record.DesiredOrigin = ""
	}
	if record.DNSMarker == "" {
		var nonce [16]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			return err
		}
		record.DNSMarker = "ya-infra/gateway:" + route.Gateway + ":" + hex.EncodeToString(nonce[:])
	}
	state[route.Hostname] = record
	if err = g.writeState(state); err != nil {
		return err
	}
	if plan.ingressChange {
		if err = g.client.request("PUT", g.client.configurationPath(), struct {
			Config tunnelConfig `json:"config"`
		}{plan.config}, nil); err != nil {
			return err
		}
	}
	observed, err := g.client.configuration()
	if err != nil {
		return err
	}
	if observed.Config == nil || !reflect.DeepEqual(*observed.Config, plan.config) {
		return errors.New("Tunnel readback differs; inspect remote changes before retry")
	}
	if route.State == "present" {
		record.Origin = route.Service
		record.DesiredOrigin = ""
		record.Phase = "dns-pending"
	}
	state[route.Hostname] = record
	if err = g.writeState(state); err != nil {
		return err
	}
	// Recheck DNS after the Tunnel write. Preserve changes made by another operator.
	records, err = g.client.records(route.Hostname)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(records, plan.records) {
		return errors.New("DNS changed during Tunnel update; inspect before retry")
	}
	zone, err := g.client.zoneID()
	if err != nil {
		return err
	}
	if plan.dnsChange {
		if route.State == "absent" {
			id := plan.records[0].ID
			if id == "" {
				return errors.New("DNS record has no identity")
			}
			if err = g.client.request("DELETE", "/zones/"+zone+"/dns_records/"+id, nil, nil); err != nil {
				return err
			}
		} else {
			body := dnsRecord{Type: "CNAME", Name: route.Hostname, Content: record.Cname, Proxied: true, TTL: 1, Comment: record.DNSMarker}
			var created dnsRecord
			if err = g.client.request("POST", "/zones/"+zone+"/dns_records", body, &created); err != nil {
				return err
			}
			record.DNSID = created.ID
		}
	} else if len(plan.records) == 1 {
		record.DNSID = plan.records[0].ID
	}
	records, err = g.client.records(route.Hostname)
	if err != nil {
		return err
	}
	if route.State == "absent" {
		if len(records) != 0 {
			return errors.New("DNS deletion readback failed")
		}
		record.Phase = "absent"
	} else {
		if len(records) != 1 || records[0].ID != record.DNSID || records[0].Type != "CNAME" || records[0].Content != record.Cname || !records[0].Proxied {
			return errors.New("DNS publication readback failed")
		}
		record.Phase = "published"
	}
	state[route.Hostname] = record
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
		for _, route := range routes {
			check("Zone / DNS / Read ("+route.Hostname+")", func() error { _, err := c.records(route.Hostname); return err })
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
	for _, route := range declarations.Routes {
		if len(args) == 1 || args[1] == route.Hostname {
			grouped[route.Gateway] = append(grouped[route.Gateway], route)
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
		for _, route := range grouped[name] {
			var operationErr error
			switch action {
			case "apply":
				operationErr = g.apply(route)
				if operationErr == nil {
					fmt.Println(route.Hostname + ": applied")
				}
			case "verify":
				operationErr = verifyRoute(route)
				if operationErr == nil {
					fmt.Println(route.Hostname + ": public HTTP check passed")
				}
			default:
				var plan routePlan
				plan, operationErr = g.plan(route)
				reportRoute(os.Stdout, action, route, plan, operationErr)
			}
			if operationErr != nil {
				failures = append(failures, fmt.Errorf("%s: %w", route.Hostname, operationErr))
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
func verifyRoute(route Route) error {
	if route.State == "absent" {
		return errors.New("public verification requires state: present")
	}
	return host.Probe("https://"+route.Hostname+route.ProbePath, route.ProbeStatus)
}

// Print only managed fields. Provider options and URL credentials stay private.
func reportRoute(out io.Writer, action string, route Route, plan routePlan, conflict error) {
	if !plan.observed {
		fmt.Fprintln(out, route.Hostname+": remote state unknown")
		return
	}
	origins := []string{}
	if plan.before.Config != nil {
		for _, rule := range plan.before.Config.Ingress {
			if rule.Hostname == nil || *rule.Hostname != route.Hostname {
				continue
			}
			origin, err := url.Parse(rule.Service)
			value := "[non-HTTP origin]"
			if err == nil && (origin.Scheme == "http" || origin.Scheme == "https") {
				value = origin.Scheme + "://" + origin.Host
			}
			origins = append(origins, value)
		}
	}
	desired := route.Service
	if route.State == "absent" {
		desired = "absent"
	}
	fmt.Fprintf(out, "%s: origin observed=%q desired=%q\n", route.Hostname, origins, desired)
	fmt.Fprintf(out, "  DNS observed_records=%d", len(plan.records))
	for _, record := range plan.records {
		if record.Type == "CNAME" {
			content := record.Content
			if !dnsName.MatchString(strings.TrimSuffix(content, ".")) {
				content = "[invalid hostname]"
			}
			fmt.Fprintf(out, " CNAME=%q proxied=%t", content, record.Proxied)
		} else {
			fmt.Fprint(out, " [non-CNAME record]")
		}
	}
	if route.State == "absent" {
		fmt.Fprintln(out, " desired=absent")
	} else {
		fmt.Fprintf(out, " desired=CNAME:%s proxied=true\n", plan.dnsTarget)
	}
	if conflict != nil {
		fmt.Fprintln(out, "  conflict: inspect observed state before apply")
	} else if action == "diff" {
		fmt.Fprintf(out, "  changes: ingress=%t DNS=%t\n", plan.ingressChange, plan.dnsChange)
	}
}
