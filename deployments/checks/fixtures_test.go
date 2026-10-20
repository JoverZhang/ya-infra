//go:build integration

package checks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func fixtureResource(t *testing.T, name string, substitutions ...string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var resource map[string]any
	if err := yaml.Unmarshal([]byte(strings.NewReplacer(substitutions...).Replace(string(data))), &resource); err != nil {
		t.Fatal(err)
	}
	return resource
}
