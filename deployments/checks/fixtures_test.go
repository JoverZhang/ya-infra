//go:build integration

package checks

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func fixtureResource[T any](t *testing.T, name string, substitutions ...string) T {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var resource T
	if err := yaml.Unmarshal([]byte(strings.NewReplacer(substitutions...).Replace(string(data))), &resource); err != nil {
		t.Fatal(err)
	}
	return resource
}

func decodeDocuments[T any](t *testing.T, data []byte) []T {
	t.Helper()
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var resources []T
	for {
		var resource T
		err := decoder.Decode(&resource)
		if err == io.EOF {
			return resources
		}
		if err != nil {
			t.Fatal(err)
		}
		resources = append(resources, resource)
	}
}
