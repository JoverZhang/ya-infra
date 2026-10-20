package main

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/JoverZhang/ya-infra/deployments/internal/host"
)

type kubeconfig struct {
	APIVersion     string             `yaml:"apiVersion"`
	Kind           string             `yaml:"kind"`
	Clusters       []*kubeconfigEntry `yaml:"clusters"`
	Users          []*kubeconfigEntry `yaml:"users"`
	Contexts       []*kubeconfigEntry `yaml:"contexts"`
	CurrentContext string             `yaml:"current-context"`
	Extra          map[string]any     `yaml:",inline"`
}

type kubeconfigEntry struct {
	Name string `yaml:"name"`
	// Pointers preserve empty mappings, such as an anonymous user, during encoding.
	Cluster *map[string]any    `yaml:"cluster,omitempty"`
	User    *map[string]any    `yaml:"user,omitempty"`
	Context *kubeconfigContext `yaml:"context,omitempty"`
	Extra   map[string]any     `yaml:",inline"`
}

type kubeconfigContext struct {
	Cluster string         `yaml:"cluster"`
	User    string         `yaml:"user"`
	Extra   map[string]any `yaml:",inline"`
}

func decodeKubeconfig(data []byte) (*kubeconfig, error) {
	var config *kubeconfig
	if err := host.Decode(data, &config); err != nil {
		return nil, err
	}
	if config == nil {
		return nil, errors.New("expected a YAML mapping")
	}
	for _, section := range []struct {
		name    string
		entries []*kubeconfigEntry
	}{{"clusters", config.Clusters}, {"users", config.Users}, {"contexts", config.Contexts}} {
		for _, entry := range section.entries {
			if entry == nil || entry.Name == "" || (section.name == "clusters" && entry.Cluster == nil) || (section.name == "users" && entry.User == nil) || (section.name == "contexts" && entry.Context == nil) {
				return nil, fmt.Errorf("invalid entry in %s; expected a name and configuration mapping", section.name)
			}
		}
	}
	return config, nil
}

// Preserve opaque cluster/authentication options and extensions. Only local names
// and context references change; existing entries must match before reuse.
func mergeKubeconfig(existing, local *kubeconfig, name string) error {
	for _, section := range []struct {
		name     string
		local    []*kubeconfigEntry
		existing *[]*kubeconfigEntry
	}{
		{"clusters", local.Clusters, &existing.Clusters},
		{"users", local.Users, &existing.Users},
		{"contexts", local.Contexts, &existing.Contexts},
	} {
		if len(section.local) != 1 {
			return fmt.Errorf("expected one local kubeconfig entry in %s", section.name)
		}
		item := *section.local[0]
		item.Name = name
		if section.name == "contexts" {
			context := *item.Context
			context.Cluster = name
			context.User = name
			item.Context = &context
		}
		found := false
		for _, entry := range *section.existing {
			if entry.Name == name {
				if !reflect.DeepEqual(entry, &item) {
					return errors.New("kubeconfig name collision; choose a new kube_context or reconcile the existing entry explicitly")
				}
				found = true
			}
		}
		if !found {
			*section.existing = append(*section.existing, &item)
		}
	}
	existing.APIVersion = "v1"
	existing.Kind = "Config"
	if existing.CurrentContext == "" {
		existing.CurrentContext = name
	}
	return nil
}
