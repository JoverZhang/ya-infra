package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These checks cross the HTTP and state-file boundaries with disposable provider state.
type cloudFixture struct {
	config          map[string]any
	records         []map[string]any
	writes          []string
	failDNS         bool
	lostDNSResponse bool
	drift           bool
	reads           int
	version         int
}

func newCloud(t *testing.T) (*cloudFixture, *httptest.Server) {
	t.Helper()
	c := &cloudFixture{version: 1, config: map[string]any{"originRequest": map[string]any{"connectTimeout": float64(17)}, "ingress": []any{map[string]any{"hostname": "*.example.com", "service": "http://other:80", "extra": "keep"}, map[string]any{"service": "http_status:404"}}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-credential-value" {
			t.Error("missing authorization")
		}
		var result any
		switch {
		case r.URL.Path == "/zones":
			result = []any{map[string]any{"id": "zone", "account": map[string]any{"id": strings.Repeat("a", 32)}}}
		case strings.HasSuffix(r.URL.Path, "/configurations"):
			if r.Method == "PUT" {
				var body struct{ Config map[string]any }
				json.NewDecoder(r.Body).Decode(&body)
				c.config = body.Config
				c.version++
				c.writes = append(c.writes, "tunnel")
			}
			c.reads++
			if c.drift && c.reads == 2 {
				c.config["external"] = true
				c.version++
			}
			result = map[string]any{"source": "cloudflare", "version": c.version, "config": c.config}
		case strings.HasSuffix(r.URL.Path, "/dns_records"):
			if r.Method == "POST" {
				if c.failDNS {
					w.WriteHeader(403)
					fmt.Fprint(w, `{"success":false,"errors":[{"code":9109,"message":"test-credential-value"}]}`)
					return
				}
				var rec map[string]any
				json.NewDecoder(r.Body).Decode(&rec)
				rec["id"] = "record"
				c.records = append(c.records, rec)
				c.writes = append(c.writes, "dns")
				if c.lostDNSResponse {
					w.WriteHeader(503)
					fmt.Fprint(w, `{ "success": false }`)
					return
				}
				result = rec
			} else {
				result = []any{}
				for _, rec := range c.records {
					if rec["name"] == r.URL.Query().Get("name") {
						result = append(result.([]any), rec)
					}
				}
			}
		case strings.HasSuffix(r.URL.Path, "/dns_records/record") && r.Method == "DELETE":
			c.records = nil
			c.writes = append(c.writes, "delete-dns")
			result = map[string]any{"id": "record"}
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
	}))
	t.Cleanup(srv.Close)
	return c, srv
}
func gatewayFixture(t *testing.T) (*cloudFixture, *gatewayController, Route) {
	c, s := newCloud(t)
	g := Gateway{Zone: "example.com", AccountID: strings.Repeat("a", 32), TunnelID: "00000000-0000-0000-0000-000000000001", APIToken: "test-credential-value"}
	ctl := &gatewayController{client: &cloudflareClient{gateway: g, baseURL: s.URL, http: s.Client()}, stateDir: t.TempDir()}
	return c, ctl, Route{Gateway: "public", Hostname: "app.example.com", Service: "http://app.apps.svc.cluster.local:8080", State: "present"}
}
func TestGatewayOwnershipAndRetry(t *testing.T) {
	c, ctl, r := gatewayFixture(t)
	before, _ := json.Marshal(c.config)
	if _, err := ctl.plan(r); err != nil {
		t.Fatal(err)
	}
	if len(c.writes) != 0 {
		t.Fatal(c.writes)
	}
	c.failDNS = true
	if err := ctl.apply(r); err == nil {
		t.Fatal("expected DNS failure")
	}
	state, err := ctl.readState()
	if err != nil {
		t.Fatal(err)
	}
	if state[r.Hostname].Phase != "dns-pending" {
		t.Fatal(state)
	}
	c.failDNS = false
	if err = ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	if err = ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.writes, []string{"tunnel", "dns"}) {
		t.Fatal(c.writes)
	}
	var original map[string]any
	json.Unmarshal(before, &original)
	if !reflect.DeepEqual(c.config["originRequest"], original["originRequest"]) {
		t.Fatal("lost unknown options")
	}
	rules := c.config["ingress"].([]any)
	if !reflect.DeepEqual(rules[1:], original["ingress"]) {
		t.Fatal("lost unrelated ingress")
	}
	info, _ := os.Stat(filepath.Join(ctl.stateDir, "routes.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	r.State = "absent"
	if err = ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	if err = ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	if len(c.records) != 0 || !reflect.DeepEqual(c.config, original) {
		t.Fatal("deletion did not preserve foreign resources")
	}
}
func TestGatewayRefusesUnownedAndDriftedResources(t *testing.T) {
	for _, mode := range []string{"matching", "foreign", "path", "drift", "dns"} {
		t.Run(mode, func(t *testing.T) {
			c, ctl, r := gatewayFixture(t)
			switch mode {
			case "matching", "foreign", "path":
				service := r.Service
				if mode == "foreign" {
					service = "http://foreign:80"
				}
				rule := map[string]any{"hostname": r.Hostname, "service": service}
				if mode == "path" {
					rule["path"] = "/api"
				}
				c.config["ingress"] = append([]any{rule}, c.config["ingress"].([]any)...)
			case "drift":
				c.drift = true
			case "dns":
				c.records = []map[string]any{{"type": "A", "name": r.Hostname, "content": "192.0.2.1"}}
			}
			if err := ctl.apply(r); err == nil {
				t.Fatal("expected conflict")
			}
			if len(c.writes) != 0 {
				t.Fatal(c.writes)
			}
		})
	}
}
func TestGatewayPreservesLegacyOwnership(t *testing.T) {
	c, ctl, r := gatewayFixture(t)
	c.config["ingress"] = append([]any{map[string]any{"hostname": r.Hostname, "service": "http://localhost:16770", "originRequest": map[string]any{"noTLSVerify": true}}}, c.config["ingress"].([]any)...)
	legacy := fmt.Sprintf(`{"%s":{"gateway":"public","origin":"http://localhost:16770","phase":"verified","backend":"paseo/demo","cname":"%s.cfargotunnel.com","dns_id":"record"}}`, r.Hostname, ctl.client.gateway.TunnelID)
	os.WriteFile(filepath.Join(ctl.stateDir, "routes.json"), []byte(legacy), 0600)
	c.records = []map[string]any{{"id": "record", "name": r.Hostname, "type": "CNAME", "content": ctl.client.gateway.TunnelID + ".cfargotunnel.com", "proxied": true}}
	if err := ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	if c.config["ingress"].([]any)[0].(map[string]any)["originRequest"] == nil {
		t.Fatal("lost route options")
	}
}
func TestCloudflareErrorsAreRedacted(t *testing.T) {
	for _, message := range []string{"test-credential-value", "Cannot use the access token from location: 192.0.2.10", "Cannot use the access token from location: fe80::1%test-credential-value"} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(403)
			json.NewEncoder(w).Encode(map[string]any{"success": false, "errors": []any{map[string]any{"code": 9109, "message": message}}})
		}))
		c := &cloudflareClient{gateway: Gateway{Zone: "example.com"}, baseURL: s.URL, http: s.Client()}
		var out any
		err := c.request("GET", "/zones", nil, &out)
		s.Close()
		if err == nil || strings.Contains(err.Error(), "test-credential-value") {
			t.Fatalf("unsafe error: %v", err)
		}
		if strings.HasPrefix(message, "Cannot use") && !strings.Contains(err.Error(), "IP") {
			t.Fatal(err)
		}
	}
}

func TestGatewayCompletesAnUncertainDNSWrite(t *testing.T) {
	c, ctl, r := gatewayFixture(t)
	c.lostDNSResponse = true
	if err := ctl.apply(r); err == nil {
		t.Fatal("expected a lost response")
	}
	c.lostDNSResponse = false
	if err := ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	state, err := ctl.readState()
	if err != nil {
		t.Fatal(err)
	}
	record := state[r.Hostname]
	if record.Phase != "published" || record.DNSID != "record" || record.DesiredOrigin != "" {
		t.Fatal("pending state was not completed", record.Phase)
	}
	if !reflect.DeepEqual(c.writes, []string{"tunnel", "dns"}) {
		t.Fatal(c.writes)
	}
}

func TestNamedGatewaysAndTwoRoutesToOneOrigin(t *testing.T) {
	c, ctl, first := gatewayFixture(t)
	second := first
	second.Hostname = "alternate.example.com"
	for _, r := range []Route{first, second, first, second} {
		if err := ctl.apply(r); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.writes) != 4 {
		t.Fatal("unchanged routes caused writes", c.writes)
	}
	secondCloud, secondCTL, third := gatewayFixture(t)
	third.Gateway = "private"
	if err := secondCTL.apply(third); err != nil {
		t.Fatal(err)
	}
	state, err := secondCTL.readState()
	if err != nil {
		t.Fatal(err)
	}
	if state[third.Hostname].Gateway != "private" || len(secondCloud.writes) != 2 {
		t.Fatal("named Gateway state was not isolated")
	}
	if err := ctl.apply(first); err != nil {
		t.Fatal(err)
	}
	if len(c.writes) != 4 {
		t.Fatal("another Gateway changed the first Gateway")
	}
}

func TestDeletedRecordCannotClaimARecreatedIngress(t *testing.T) {
	c, ctl, r := gatewayFixture(t)
	if err := ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	r.State = "absent"
	if err := ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	writes := len(c.writes)
	c.config["ingress"] = append([]any{map[string]any{"hostname": r.Hostname, "service": r.Service}}, c.config["ingress"].([]any)...)
	for _, state := range []string{"absent", "present"} {
		r.State = state
		if err := ctl.apply(r); err == nil {
			t.Fatal("deleted record authorized an external recreation", state)
		}
	}
	if len(c.writes) != writes {
		t.Fatal("external ingress was changed")
	}
}

func TestRecreatedRouteUsesNewOwnershipIntent(t *testing.T) {
	_, ctl, r := gatewayFixture(t)
	if err := ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	before, err := ctl.readState()
	if err != nil {
		t.Fatal(err)
	}
	r.State = "absent"
	if err := ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	r.State = "present"
	if err := ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	after, err := ctl.readState()
	if err != nil {
		t.Fatal(err)
	}
	if before[r.Hostname].DNSMarker == after[r.Hostname].DNSMarker {
		t.Fatal("new generation reused a deleted ownership marker")
	}
}

func TestGatewayReportsObservedDriftAndDesiredChanges(t *testing.T) {
	c, ctl, r := gatewayFixture(t)
	if err := ctl.apply(r); err != nil {
		t.Fatal(err)
	}
	c.config["ingress"].([]any)[0].(map[string]any)["service"] = "http://foreign:80"
	p, err := ctl.plan(r)
	if err == nil {
		t.Fatal("expected drift")
	}
	var out strings.Builder
	reportRoute(&out, "status", r, p, err)
	if !strings.Contains(out.String(), "http://foreign:80") || !strings.Contains(out.String(), r.Service) || !strings.Contains(out.String(), "conflict") || strings.Contains(out.String(), "unknown") {
		t.Fatal(out.String())
	}
	out.Reset()
	reportRoute(&out, "status", r, routePlan{}, fmt.Errorf("failed read"))
	if !strings.Contains(out.String(), "unknown") {
		t.Fatal(out.String())
	}
	out.Reset()
	reportRoute(&out, "diff", r, p, err)
	if !strings.Contains(out.String(), "DNS") || !strings.Contains(out.String(), "CNAME") {
		t.Fatal(out.String())
	}
	c.config["ingress"].([]any)[0].(map[string]any)["service"] = "https://user:private@foreign:443/?token=private"
	p, err = ctl.plan(r)
	out.Reset()
	reportRoute(&out, "status", r, p, err)
	if strings.Contains(out.String(), "private") {
		t.Fatal("remote credential leaked", out.String())
	}
}
