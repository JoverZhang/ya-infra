package main

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloadReportsConnectionFailure(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	_, err := fetch(server.URL+"/binary", 1024)
	var connectionError *net.OpError
	if !errors.As(err, &connectionError) || !strings.Contains(err.Error(), server.URL+"/binary") {
		t.Fatalf("missing download URL or connection cause: %v", err)
	}
}
