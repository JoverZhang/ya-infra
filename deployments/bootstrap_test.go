package main

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
)

func TestBootstrapStorageAndKubeconfigGuards(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, value := range []string{"", "relative"} {
		t.Setenv("XDG_DATA_HOME", value)
		if got := host.XDG("XDG_DATA_HOME", ".local/share"); got != filepath.Join(home, ".local/share") {
			t.Fatal(got)
		}
	}
	target := filepath.Join(home, "data")
	t.Setenv("XDG_DATA_HOME", target)
	data, err := localConfiguration(map[string]any{"disable": []any{"traefik", "servicelb"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err = host.Decode(data, &config); err != nil {
		t.Fatal(err)
	}
	if config["default-local-storage-path"] != filepath.Join(target, "ya-infra", "pv") || config["secrets-encryption"] != true {
		t.Fatal(config)
	}
	if _, err = localConfiguration(map[string]any{}, "/other/data"); err == nil {
		t.Fatal("data-root change accepted")
	}
	original := map[string]any{"clusters": []any{map[string]any{"name": "other", "cluster": map[string]any{"server": "https://other"}}}, "current-context": "other"}
	source := map[string]any{"clusters": []any{map[string]any{"name": "default", "cluster": map[string]any{"server": "https://127.0.0.1:6443"}}}, "users": []any{map[string]any{"name": "default", "user": map[string]any{"token": "fixture"}}}, "contexts": []any{map[string]any{"name": "default", "context": map[string]any{"cluster": "default", "user": "default"}}}}
	if err = mergeKubeconfig(original, source, "local"); err != nil {
		t.Fatal(err)
	}
	if original["current-context"] != "other" || len(original["clusters"].([]any)) != 2 {
		t.Fatal(original)
	}
	if err = mergeKubeconfig(original, source, "local"); err != nil {
		t.Fatal(err)
	}
	source["clusters"].([]any)[0].(map[string]any)["cluster"] = map[string]any{"server": "https://foreign"}
	if err = mergeKubeconfig(original, source, "local"); err == nil {
		t.Fatal("context collision accepted")
	}
	path := filepath.Join(home, "private")
	if err = host.WritePrivate(path, []byte("same")); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	if err = host.WritePrivate(path, []byte("same")); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !before.ModTime().Equal(after.ModTime()) || after.Mode().Perm() != 0600 {
		t.Fatal("unchanged private file was rewritten")
	}
}

func TestBootstrapRejectsNetworkConflicts(t *testing.T) {
	occupied := []netip.Prefix{netip.MustParsePrefix("10.40.0.0/12")}
	if err := checkRanges(map[string]any{}, occupied); err == nil {
		t.Fatal("overlapping host route accepted")
	}
	custom := map[string]any{"cluster-cidr": "10.200.0.0/16", "service-cidr": "10.201.0.0/16"}
	if err := checkRanges(custom, occupied); err != nil {
		t.Fatal(err)
	}
	custom["service-cidr"] = "10.200.10.0/24"
	if err := checkRanges(custom, occupied); err == nil {
		t.Fatal("Pod and Service range overlap accepted")
	}
	custom["cluster-cidr"] = "not-a-network"
	if err := checkRanges(custom, nil); err == nil {
		t.Fatal("invalid network accepted")
	}
}
