package pluginmodel

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kubernetes-incubator/kube-aws/model"
)

// A plugin consists of two parts: a set of metadata and a spec
type Plugin struct {
	Metadata `yaml:"metadata,omitempty"`
	Spec     `yaml:"spec,omitempty"`
}

func (p Plugin) EnabledIn(plugins model.PluginConfigs) (bool, *model.PluginConfig) {
	for name, c := range plugins {
		if name == p.SettingKey() && c.Enabled {
			return true, &c
		}
	}
	return false, nil
}

func (p Plugin) Validate() error {
	if err := p.Metadata.Validate(); err != nil {
		return fmt.Errorf("Invalid metadata: %v", err)
	}
	return nil
}

func (p Plugin) SettingKey() string {
	words := strings.Split(p.Metadata.Name, "-")
	for i, _ := range words {
		if i > 0 {
			words[i] = strings.Title(words[i])
		}
	}
	return strings.Join(words, "")
}

// Metadata is the metadata of a kube-aws plugin consists of various settings specific to the plugin itself
// Metadata never affects what are injected into K8S clusters, node, other CFN resources managed by kube-aws.
type Metadata struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	// ClusterSettingsKey is the key in the root of cluster.yaml used for configuring this plugin cluster-wide
	ClusterSettingsKey string `yaml:"clusterSettingsKey,omitempty"`
	// NodePoolSettingsKey is the key in the root of a node pool settings in cluster.yaml used for configuring this plugin only for a node pool
	NodePoolSettingsKey string `yaml:"nodePoolSettingKey,omitempty"`
}

func (m Metadata) Validate() error {
	if m.Name == "" {
		return errors.New("`name` must not be empty")
	}
	if m.Version == "" {
		return errors.New("`version` must not be empty")
	}
	return nil
}

// Spec is the specification of a kube-aws plugin
// A spec consists of two parts: Configuration and Command
type Spec struct {
	// Configuration is the configuration part of a plugin which is used to append arbitrary configs into various resources managed by kube-aws
	Configuration `yaml:"configuration,omitempty"`
}

// Configuration is the configuration part of a plugin which is used to append arbitrary configs into various resources managed by kube-aws
type Configuration struct {
	// Values represents the values available in templates
	Values `yaml:"values,omitempty"`
	// CloudFormation represents customizations to CloudFormation-related settings and configurations
	CloudFormation `yaml:"cloudformation,omitempty"`
	// Helm represents what are injected into the resulting K8S cluster via Helm - a package manager for K8S
	Helm `yaml:"helm,omitempty"`
	// Kubernetes represents what are injected into the resulting K8S
	Kubernetes `yaml:"kubernetes,omitempty"`
	// Node represents what are injected into each node managed by kube-aws
	Node `yaml:"node,omitempty"`
}

// CloudFormation represents customizations to CloudFormation-related settings and configurations
type CloudFormation struct {
	Stacks `yaml:"stacks,omitempty"`
}

type Stacks struct {
	Root         Stack `yaml:"root,omitempty"`
	ControlPlane Stack `yaml:"controlPlane,omitempty"`
	NodePool     Stack `yaml:"nodePool,omitempty"`
}

// Stack represents a set of customizations to a CloudFormation stack template
// Top-level keys should be one of: Resources, Outputs
// Second-level keys should be cfn resource names
type Stack struct {
	Resources `yaml:"resources,omitempty"`
	Outputs   `yaml:"outputs,omitempty"`
}

type Resources struct {
	Append `yaml:"append,omitempty"`
}

type Outputs struct {
	Append `yaml:"append,omitempty"`
}

type Append struct {
	Contents `yaml:",inline"`
}

type Helm struct {
	// Releases is a list of helm releases to be maintained on the cluster.
	// Note that the list is sorted by their names by kube-aws so that it won't result in unnecessarily node replacements.
	Releases HelmReleases `yaml:"releases,omitempty"`
}

func (k *Helm) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type t Helm
	work := t(Helm{
		Releases: HelmReleases{},
	})
	if err := unmarshal(&work); err != nil {
		return fmt.Errorf("failed to parse helm plugin config: %v", err)
	}
	*k = Helm(work)

	return nil
}

type HelmReleases []HelmRelease

type HelmRelease struct {
	Name    string `yaml:"name,omitempty"`
	Chart   string `yaml:"chart,omitempty"`
	Version string `yaml:"version,omitempty"`
	Values  Values `yaml:"values,omitempty"`
}

type Kubernetes struct {
	APIServer KubernetesAPIServer `yaml:"apiserver,omitempty"`
	// Manifests is a list of manifests to be installed to the cluster.
	// Note that the list is sorted by their names by kube-aws so that it won't result in unnecessarily node replacements.
	Manifests KubernetesManifests `yaml:"manifests,omitempty"`
}

func (k *Kubernetes) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type t Kubernetes
	work := t(Kubernetes{
		Manifests: KubernetesManifests{},
	})
	if err := unmarshal(&work); err != nil {
		return fmt.Errorf("failed to parse kubernetes plugin config: %v", err)
	}
	*k = Kubernetes(work)

	return nil
}

type KubernetesAPIServer struct {
	Flags   APIServerFlags   `yaml:"flags,omitempty"`
	Volumes APIServerVolumes `yaml:"volumes,omitempty"`
}

type APIServerFlags []APIServerFlag

type APIServerFlag struct {
	// Name is the name of a command-line flag passed to the k8s apiserver.
	// For example, a name 	is "oidc-issuer-url" for the flag `--oidc-issuer-url`.
	Name string `yaml:"name,omitempty"`
	// Value is a golang text template resulting to the value of a command-line flag passed to the k8s apiserver
	Value string `yaml:"value,omitempty"`
}

type APIServerVolumes []APIServerVolume

type APIServerVolume struct {
	// Name is translated to both a volume mount's and volume's name
	Name string `yaml:"name,omitempty"`
	// Path is translated to both a volume mount's mountPath and a volume's hostPath
	Path     string `yaml:"path,omitempty"`
	ReadOnly bool   `yaml:"readOnly,omitempty"`
}

type KubernetesManifests []KubernetesManifest

type KubernetesManifest struct {
	Name     string `yaml:"name,omitempty"`
	Contents `yaml:"contents,omitempty"`
}

type Contents struct {
	Inline string `yaml:"inline,omitempty"`
	Source `yaml:"source,omitempty"`
	// TODO Better naming
	UnknownKeys map[string]interface{} `yaml:",inline"`
}

type Source struct {
	Path string `yaml:"path,omitempty"`
}

type Node struct {
	Roles NodeRoles `yaml:"roles,omitempty"`
}

type NodeRoles struct {
	Controller `yaml:"controller,omitempty"`
	Etcd       `yaml:"etcd,omitempty"`
	Worker     `yaml:"worker,omitempty"`
}

type Controller struct {
	CommonNodeConfig `yaml:",inline"`
	Kubelet          `yaml:"kubelet,omitempty"`
}

type Etcd struct {
	CommonNodeConfig `yaml:",inline"`
}

type Worker struct {
	CommonNodeConfig `yaml:",inline"`
	Kubelet          `yaml:"kubelet,omitempty"`
}

type CommonNodeConfig struct {
	Storage `yaml:"storage,omitempty"`
	IAM     `yaml:"iam,omitempty"`
	Systemd `yaml:"systemd,omitempty"`
}

type Storage struct {
	Files `yaml:"files,omitempty"`
}

type Files []File

type File struct {
	Path     string `yaml:"path,omitempty"`
	Contents `yaml:"contents,omitempty"`
	//Mode     string `yaml:"mode,omitempty"`
	Permissions uint `yaml:"permissions,omitempty"`
}

type IAM struct {
	Policy model.IAMPolicy `yaml:"policy,omitempty"`
}

type Systemd struct {
	// Units is a list of systemd units installed on the nodes
	Units SystemdUnits `yaml:"units,omitempty"`
}

type SystemdUnits []SystemdUnit

type SystemdUnit struct {
	Name string `yaml:"name,omitempty"`
	// Contents must be a valid go text template producing a valid systemd unit definition
	Contents `yaml:"contents,omitempty"`
}

// Kubelet represents a set of customizations to kubelets running on the nodes
// Keys must be included in: nodeLabels, featureGates, etc
// kubelet can be configured per-node-pool-basic hence a part of WorkerSettings
type Kubelet struct {
	FeatureGates FeatureGates `yaml:"featureGates,omitempty"`
	NodeLabels   NodeLabels   `yaml:"nodeLabels,omitempty"`
}

type FeatureGates map[string]string
type NodeLabels map[string]string
type Values map[string]interface{}
