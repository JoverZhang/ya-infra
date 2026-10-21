//go:build integration

package checks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDeploymentViewPreservesNativeFields(t *testing.T) {
	for _, app := range []string{"paseo", "browserlab", "tbg"} {
		t.Run(app, func(t *testing.T) {
			path := filepath.Join("..", "apps", app, "kubernetes", "deployment.yaml")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, original := range decodeDocuments[map[string]any](t, data) {
				input, err := yaml.Marshal(original)
				if err != nil {
					t.Fatal(err)
				}
				view := decodeDocuments[deployment](t, input)[0]
				encoded, err := yaml.Marshal(view)
				if err != nil {
					t.Fatal(err)
				}
				actual := decodeDocuments[map[string]any](t, encoded)[0]
				if !reflect.DeepEqual(original, actual) {
					t.Fatalf("typed view changed native manifest %s\n%s", path, encoded)
				}
			}
		})
	}
}
