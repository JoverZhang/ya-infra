package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type cloudflareClient struct {
	gateway Gateway
	baseURL string
	http    *http.Client
	zone    string
}

type apiError struct {
	method, path  string
	status        int
	codes         []int
	hint, blocked string
}

func (e *apiError) Error() string {
	message := fmt.Sprintf("Cloudflare %s %s failed: HTTP %d, error codes %v", e.method, e.path, e.status, e.codes)
	if e.blocked != "" {
		return message + ". Blocked by token Client IP Address Filtering; " + e.blocked
	}
	if (e.status == 401 || e.status == 403) && e.hint != "" {
		message += ". Check " + e.hint + "; resource scope and Client IP Address Filtering"
	}
	return message
}
func newClient(g Gateway) *cloudflareClient {
	return &cloudflareClient{gateway: g, baseURL: "https://api.cloudflare.com/client/v4", http: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *cloudflareClient) configurationPath() string {
	return "/accounts/" + c.gateway.AccountID + "/cfd_tunnel/" + c.gateway.TunnelID + "/configurations"
}
func (c *cloudflareClient) permission(method, path string) string {
	access := "Read or Edit"
	if method != "GET" {
		access = "Edit"
	}
	switch {
	case path == "/zones":
		return "Zone / Zone / Read for " + c.gateway.Zone + " in account " + c.gateway.AccountID
	case path == c.configurationPath():
		return "Account / Cloudflare One Connector: cloudflared / " + access + " for account " + c.gateway.AccountID
	case strings.Contains(path, "/dns_records"):
		return "Zone / DNS / " + access + " for " + c.gateway.Zone
	}
	return ""
}
func (c *cloudflareClient) request(method, path string, body any, out any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return errors.New("invalid Cloudflare request body")
		}
	}
	req, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return errors.New("invalid Cloudflare request URL")
	}
	req.Header.Set("Authorization", "Bearer "+c.gateway.APIToken)
	req.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(req)
	endpoint := strings.SplitN(path, "?", 2)[0]
	if err != nil {
		return fmt.Errorf("Cloudflare %s %s did not complete; read current state before retry", method, endpoint)
	}
	defer response.Body.Close()
	var envelope struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
		Errors  []struct {
			Code    json.RawMessage `json:"code"`
			Message string          `json:"message"`
		} `json:"errors"`
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&envelope)
	if err != nil || !envelope.Success || response.StatusCode >= 300 {
		failure := &apiError{method: method, path: endpoint, status: response.StatusCode, hint: c.permission(method, endpoint), codes: []int{}}
		for _, item := range envelope.Errors {
			var code int
			if json.Unmarshal(item.Code, &code) == nil {
				failure.codes = append(failure.codes, code)
			}
			if suffix, ok := strings.CutPrefix(item.Message, "Cannot use the access token from location:"); ok {
				failure.blocked = "allow the outgoing IP or remove the entire filter rule; this does not prove a missing permission"
				if ip := net.ParseIP(strings.TrimSpace(suffix)); ip != nil {
					bits := 128
					if ip.To4() != nil {
						bits = 32
					}
					failure.blocked = fmt.Sprintf("allow %s/%d or remove the entire filter rule", ip.String(), bits)
				}
			}
		}
		return failure
	}
	if out != nil && json.Unmarshal(envelope.Result, out) != nil {
		return errors.New("invalid Cloudflare result")
	}
	return nil
}
func (c *cloudflareClient) zoneID() (string, error) {
	if c.zone != "" {
		return c.zone, nil
	}
	var result []struct {
		ID      string
		Account struct{ ID string }
	}
	query := url.Values{"name": {c.gateway.Zone}, "account.id": {c.gateway.AccountID}}
	if err := c.request("GET", "/zones?"+query.Encode(), nil, &result); err != nil {
		return "", err
	}
	if len(result) != 1 || result[0].Account.ID != c.gateway.AccountID || result[0].ID == "" {
		return "", errors.New("zone lookup did not match the declared account and zone")
	}
	c.zone = result[0].ID
	return c.zone, nil
}
func (c *cloudflareClient) configuration() (map[string]any, error) {
	var value map[string]any
	err := c.request("GET", c.configurationPath(), nil, &value)
	if err == nil && value["source"] != nil && value["source"] != "cloudflare" {
		err = errors.New("gateway requires a remotely managed Tunnel")
	}
	return value, err
}
func (c *cloudflareClient) records(hostname string) ([]map[string]any, error) {
	zone, err := c.zoneID()
	if err != nil {
		return nil, err
	}
	var records []map[string]any
	err = c.request("GET", "/zones/"+zone+"/dns_records?"+url.Values{"name": {hostname}, "per_page": {"100"}}.Encode(), nil, &records)
	return records, err
}
func (c *cloudflareClient) verifyToken() error {
	var result struct{ Status string }
	err := c.request("GET", "/user/tokens/verify", nil, &result)
	if e, ok := err.(*apiError); ok && (e.status == 401 || e.status == 403) {
		first := err
		if err = c.request("GET", "/accounts/"+c.gateway.AccountID+"/tokens/verify", nil, &result); err != nil {
			return first
		}
	}
	if err != nil {
		return err
	}
	if result.Status != "active" {
		return errors.New("token is not active")
	}
	return nil
}
