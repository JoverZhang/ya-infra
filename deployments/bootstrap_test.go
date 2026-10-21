package main

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
)

func TestXDGDataFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, value := range []string{"", "relative"} {
		t.Setenv("XDG_DATA_HOME", value)
		if got := host.XDG("XDG_DATA_HOME", ".local/share"); got != filepath.Join(home, ".local/share") {
			t.Fatal(got)
		}
	}
}

func TestBootstrapStorageGuards(t *testing.T) {
	target := filepath.Join(t.TempDir(), "data")
	t.Setenv("XDG_DATA_HOME", target)
	data, err := localConfiguration(serverOptions{Extra: map[string]any{"disable": []any{"traefik", "servicelb"}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	var config serverOptions
	if err = host.Decode(data, &config); err != nil {
		t.Fatal(err)
	}
	if config.StoragePath != filepath.Join(target, "ya-infra", "pv") || config.SecretsEncryption == nil || !*config.SecretsEncryption || config.Extra["disable"] == nil {
		t.Fatal(config)
	}
	if _, err = localConfiguration(serverOptions{}, "/other/data"); err == nil {
		t.Fatal("data-root change accepted")
	}
}

func TestKubeconfigMergePreservesOtherContexts(t *testing.T) {
	existingData, err := os.ReadFile("testdata/kubeconfig-existing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	original, err := decodeKubeconfig(existingData)
	if err != nil {
		t.Fatal(err)
	}
	localData, err := os.ReadFile("testdata/kubeconfig-local.yaml")
	if err != nil {
		t.Fatal(err)
	}
	source, err := decodeKubeconfig(localData)
	if err != nil {
		t.Fatal(err)
	}
	if err = mergeKubeconfig(original, source, "local"); err != nil {
		t.Fatal(err)
	}
	if original.CurrentContext != "other" || len(original.Clusters) != 2 || original.Extra["extensions"] == nil || (*original.Clusters[0].Cluster)["proxy-url"] != "http://proxy.example:8080" || original.Contexts[0].Context.Extra["namespace"] != "apps" {
		t.Fatal(original)
	}
	if err = mergeKubeconfig(original, source, "local"); err != nil {
		t.Fatal(err)
	}
	source.Clusters[0].Cluster = &map[string]any{"server": "https://foreign"}
	if err = mergeKubeconfig(original, source, "local"); err == nil {
		t.Fatal("context collision accepted")
	}
}

func TestPrivateWriteIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private")
	if err := host.WritePrivate(path, []byte("same")); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := host.WritePrivate(path, []byte("same")); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) || after.Mode().Perm() != 0600 {
		t.Fatal("unchanged private file was rewritten")
	}
}

func TestBootstrapRejectsNetworkConflicts(t *testing.T) {
	occupied := []netip.Prefix{netip.MustParsePrefix("10.40.0.0/12")}
	if err := checkRanges(serverOptions{}, occupied); err == nil {
		t.Fatal("overlapping host route accepted")
	}
	custom := serverOptions{ClusterCIDR: cidrRanges{"10.200.0.0/16"}, ServiceCIDR: cidrRanges{"10.201.0.0/16"}}
	if err := checkRanges(custom, occupied); err != nil {
		t.Fatal(err)
	}
	custom.ServiceCIDR = cidrRanges{"10.200.10.0/24"}
	if err := checkRanges(custom, occupied); err == nil {
		t.Fatal("Pod and Service range overlap accepted")
	}
	custom.ClusterCIDR = cidrRanges{"not-a-network"}
	if err := checkRanges(custom, nil); err == nil {
		t.Fatal("invalid network accepted")
	}
}

func TestNativeServerCIDRForms(t *testing.T) {
	for _, input := range []string{
		"cluster-cidr: 10.200.0.0/16,fd00:200::/64\nservice-cidr: 10.201.0.0/16,fd00:201::/112\n",
		"cluster-cidr: [10.200.0.0/16, 'fd00:200::/64']\nservice-cidr: [10.201.0.0/16, 'fd00:201::/112']\n",
	} {
		var options serverOptions
		if err := host.Decode([]byte(input), &options); err != nil {
			t.Fatal(err)
		}
		if err := checkRanges(options, nil); err != nil {
			t.Fatal(err)
		}
	}
}
