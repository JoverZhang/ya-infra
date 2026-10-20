//go:build integration

package checks

// These views type the fields changed or inspected by the checks. Inline fields
// retain native settings, including probes and security controls, during apply.
type deployment struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   objectMetadata `yaml:"metadata"`
	Spec       deploymentSpec `yaml:"spec"`
	Extra      map[string]any `yaml:",inline"`
}

type objectMetadata struct {
	Name      string            `yaml:"name,omitempty"`
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
	Extra     map[string]any    `yaml:",inline"`
}

type deploymentSpec struct {
	Replicas *int           `yaml:"replicas,omitempty"`
	Template podTemplate    `yaml:"template"`
	Extra    map[string]any `yaml:",inline"`
}

type podTemplate struct {
	Metadata objectMetadata `yaml:"metadata"`
	Spec     podSpec        `yaml:"spec"`
	Extra    map[string]any `yaml:",inline"`
}

type podSpec struct {
	Containers     []container       `yaml:"containers"`
	InitContainers []container       `yaml:"initContainers,omitempty"`
	Volumes        []volume          `yaml:"volumes,omitempty"`
	NodeSelector   map[string]string `yaml:"nodeSelector,omitempty"`
	HostAliases    []hostAlias       `yaml:"hostAliases,omitempty"`
	Extra          map[string]any    `yaml:",inline"`
}

type hostAlias struct {
	IP        string   `yaml:"ip"`
	Hostnames []string `yaml:"hostnames"`
}

type container struct {
	Name    string                `yaml:"name"`
	Image   string                `yaml:"image"`
	Args    []string              `yaml:"args,omitempty"`
	Env     []environmentVariable `yaml:"env,omitempty"`
	EnvFrom []environmentSource   `yaml:"envFrom,omitempty"`
	Extra   map[string]any        `yaml:",inline"`
}

type environmentVariable struct {
	Name  string         `yaml:"name"`
	Value *string        `yaml:"value,omitempty"`
	Extra map[string]any `yaml:",inline"`
}

type environmentSource struct {
	SecretRef *objectReference `yaml:"secretRef,omitempty"`
	Extra     map[string]any   `yaml:",inline"`
}

type objectReference struct {
	Name  string         `yaml:"name"`
	Extra map[string]any `yaml:",inline"`
}

type volume struct {
	Name                  string          `yaml:"name"`
	Secret                *secretVolume   `yaml:"secret,omitempty"`
	PersistentVolumeClaim *map[string]any `yaml:"persistentVolumeClaim,omitempty"`
	HostPath              *map[string]any `yaml:"hostPath,omitempty"`
	EmptyDir              *map[string]any `yaml:"emptyDir,omitempty"`
	Extra                 map[string]any  `yaml:",inline"`
}

type secretVolume struct {
	SecretName string         `yaml:"secretName"`
	Extra      map[string]any `yaml:",inline"`
}

type persistentVolumeClaim struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   objectMetadata `yaml:"metadata"`
	Spec       struct {
		VolumeName string         `yaml:"volumeName,omitempty"`
		Extra      map[string]any `yaml:",inline"`
	} `yaml:"spec"`
}

type storageClass struct {
	APIVersion        string            `yaml:"apiVersion"`
	Kind              string            `yaml:"kind"`
	Metadata          objectMetadata    `yaml:"metadata"`
	Provisioner       string            `yaml:"provisioner"`
	Parameters        map[string]string `yaml:"parameters,omitempty"`
	ReclaimPolicy     string            `yaml:"reclaimPolicy"`
	VolumeBindingMode string            `yaml:"volumeBindingMode"`
}
