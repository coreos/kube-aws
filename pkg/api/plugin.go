package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kubernetes-incubator/kube-aws/provisioner"
)

// A plugin consists of two parts: a set of metadata and a spec
type Plugin struct {
	Metadata `yaml:"metadata,omitempty"`
	Spec     PluginSpec `yaml:"spec,omitempty"`
}

func (p Plugin) EnabledIn(plugins PluginConfigs) (bool, *PluginConfig) {
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

// PluginSpec is the specification of a kube-aws plugin
// A spec consists of two parts: Cluster and Command
type PluginSpec struct {
	// Cluster is the configuration part of a plugin which is used to append arbitrary configs into various resources managed by kube-aws
	Cluster ClusterSpec `yaml:"cluster,omitempty"`
}

// Cluster is the configuration part of a plugin which is used to append arbitrary configs into various resources managed by kube-aws
type ClusterSpec struct {
	// Values represents the values available in templates
	Values `yaml:"values,omitempty"`
	// CloudFormation represents customizations to CloudFormation-related settings and configurations
	CloudFormation CloudFormationSpec `yaml:"cloudformation,omitempty"`
	// Helm represents what are injected into the resulting K8S cluster via Helm - a package manager for K8S
	Helm `yaml:"helm,omitempty"`
	// Kubernetes represents what are injected into the resulting K8S
	Kubernetes Kubernetes `yaml:"kubernetes,omitempty"`
	// Machine represents what are injected into each machines managed by kube-aws
	Machine `yaml:"machine,omitempty"`
	// PKI extends the cluster PKI managed by kube-aws
	PKI `yaml:"pki,omitempty"`
}

// CloudFormation represents customizations to CloudFormation-related settings and configurations
type CloudFormationSpec struct {
	Stacks `yaml:"stacks,omitempty"`
}

type Stacks struct {
	Root         Stack `yaml:"root,omitempty"`
	Network      Stack `yaml:"network,omitempty"`
	ControlPlane Stack `yaml:"controlPlane,omitempty"`
	Etcd         Stack `yaml:"etcd,omitempty"`
	NodePool     Stack `yaml:"nodePool,omitempty"`
}

// Stack represents a set of customizations to a CloudFormation stack template
// Top-level keys should be one of: Resources, Outputs
// Second-level keys should be cfn resource names
type Stack struct {
	Resources `yaml:"resources,omitempty"`
	Outputs   `yaml:"outputs,omitempty"`
	Tags      `yaml:"tags,omitempty"`
}

type Resources struct {
	provisioner.RemoteFileSpec `yaml:",inline"`
}

type Outputs struct {
	provisioner.RemoteFileSpec `yaml:",inline"`
}

type Tags struct {
	provisioner.RemoteFileSpec `yaml:",inline"`
}

type Helm struct {
	// Releases is a list of helm releases to be maintained on the cluster.
	// Note that the list is sorted by their names by kube-aws so that it won't result in unnecessarily node replacements.
	Releases HelmReleases `yaml:"releases,omitempty"`
}

//func (k *Helm) UnmarshalYAML(unmarshal func(interface{}) error) error {
//	type t Helm
//	work := t(Helm{
//		Releases: HelmReleases{},
//	})
//	if err := unmarshal(&work); err != nil {
//		return fmt.Errorf("failed to parse helm plugin config: %v", err)
//	}
//	*k = Helm(work)
//
//	return nil
//}
//
type HelmReleases []HelmRelease

type HelmRelease struct {
	Name    string `yaml:"name,omitempty"`
	Chart   string `yaml:"chart,omitempty"`
	Version string `yaml:"version,omitempty"`
	Values  Values `yaml:"values,omitempty"`
}

type KubernetesAPIServer struct {
	Flags   CommandLineFlags `yaml:"flags,omitempty"`
	Volumes APIServerVolumes `yaml:"volumes,omitempty"`
}

type CommandLineFlags []CommandLineFlag

type CommandLineFlag struct {
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
	Name                       string `yaml:"name,omitempty"`
	provisioner.RemoteFileSpec `yaml:",inline"`
}

type Contents struct {
	provisioner.RemoteFileSpec `yaml:",inline"`
	// TODO Better naming
	UnknownKeys map[string]interface{} `yaml:",inline"`
}

type Source struct {
	Path string `yaml:"path,omitempty"`
}

type Machine struct {
	Roles MachineRoles `yaml:"roles,omitempty"`
}

type MachineRoles struct {
	Controller Node        `yaml:"controller,omitempty"`
	Etcd       MachineSpec `yaml:"etcd,omitempty"`
	Worker     Node        `yaml:"worker,omitempty"`
}

// Node is a worker machine in Kubernetes
type Node struct {
	MachineSpec `yaml:",inline"`
	Kubelet     KubeletSpec `yaml:"kubelet,omitempty"`
}

type MachineSpec struct {
	Files   `yaml:"files,omitempty"`
	IAM     `yaml:"iam,omitempty"`
	Systemd `yaml:"systemd,omitempty"`
}

type Files []provisioner.RemoteFileSpec

type IAM struct {
	Policy IAMPolicy `yaml:"policy,omitempty"`
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
type KubeletSpec struct {
	FeatureGates FeatureGates           `yaml:"featureGates,omitempty"`
	NodeLabels   NodeLabels             `yaml:"nodeLabels,omitempty"`
	Kubeconfig   string                 `yaml:"kubeconfig,omitempty"`
	Mounts       []ContainerVolumeMount `yaml:"mounts,omitempty"`
}

type ContainerVolumeMount string

func (m ContainerVolumeMount) ToRktRunArgs() []string {
	args := []string{}
	// Avoids invalid volname like "-opt-bin" for "/opt/bin" or "opt-bin-" for "opt/bin/". It should obviously be "opt-bin".
	volname := strings.Replace(strings.TrimPrefix(strings.TrimSuffix(string(m), "/"), "/"), "/", "-", -1)
	args = append(
		args,
		fmt.Sprintf("--mount volume=%s,target=%s", volname, string(m)),
		fmt.Sprintf("--volume %s,kind=host,source=%s", volname, string(m)),
	)
	return args
}

type Values map[string]interface{}
