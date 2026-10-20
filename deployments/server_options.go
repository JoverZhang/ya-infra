package main

import (
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

// Only options interpreted by bootstrap are typed. The upstream server owns
// all other options; retain them when writing its native configuration.
type serverOptions struct {
	StoragePath         string         `yaml:"default-local-storage-path,omitempty"`
	SecretsEncryption   *bool          `yaml:"secrets-encryption,omitempty"`
	WriteKubeconfigMode string         `yaml:"write-kubeconfig-mode,omitempty"`
	ClusterCIDR         cidrRanges     `yaml:"cluster-cidr,omitempty"`
	ServiceCIDR         cidrRanges     `yaml:"service-cidr,omitempty"`
	HTTPSListenPort     *int           `yaml:"https-listen-port,omitempty"`
	FlannelBackend      string         `yaml:"flannel-backend,omitempty"`
	Extra               map[string]any `yaml:",inline"`
}

type cidrRanges []string

func (ranges *cidrRanges) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
		*ranges = strings.Split(node.Value, ",")
		return nil
	}
	if node.Kind != yaml.SequenceNode {
		return errors.New("CIDRs must be a string or sequence")
	}
	var values []string
	if err := node.Decode(&values); err != nil {
		return err
	}
	if len(values) == 0 {
		return errors.New("CIDR sequence must not be empty")
	}
	*ranges = values
	return nil
}
