package host

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeReportsConnectionFailureWithoutURLSecrets(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	endpoint := strings.Replace(server.URL, "//", "//fixture-user:fixture-password@", 1) + "/healthz?token=fixture-query"
	err := Probe(endpoint, http.StatusOK)
	var connectionError *net.OpError
	if !errors.As(err, &connectionError) || !strings.Contains(err.Error(), server.URL+"/healthz") {
		t.Fatalf("missing endpoint or connection cause: %v", err)
	}
	for _, secret := range []string{"fixture-user", "fixture-password", "fixture-query"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error contains URL secret: %v", err)
		}
	}
}
