package model

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/builtin"
	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/kubernetes-incubator/kube-aws/provisioner"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	vpcLogicalName             = "VPC"
	internetGatewayLogicalName = "InternetGateway"
)

// Config contains configuration parameters available when rendering userdata injected into a controller or an etcd node from golang text templates
type Config struct {
	*api.Cluster

	AdminAPIEndpoint APIEndpoint
	APIEndpoints     APIEndpoints

	// EtcdNodes is the golang-representation of etcd nodes, which is used to differentiate unique etcd nodes
	// This is used to simplify templating of the control-plane stack template.
	EtcdNodes []EtcdNode

	APIServerVolumes api.APIServerVolumes
	APIServerFlags   api.CommandLineFlags
	ControllerFlags  api.CommandLineFlags

	KubernetesManifestFiles []*provisioner.RemoteFile
	HelmReleaseFilesets     []api.HelmReleaseFileset
}

func (c *Config) EtcdCluster() EtcdCluster {
	etcdNetwork := NewNetwork(c.Etcd.Subnets, c.NATGateways())
	return NewEtcdCluster(c.Etcd.Cluster, c.Region, etcdNetwork, c.Etcd.Count)
}

// AdminAPIEndpointURL is the url of the API endpoint which is written in kubeconfig and used to by admins
func (c *Config) AdminAPIEndpointURL() string {
	return fmt.Sprintf("https://%s", c.AdminAPIEndpoint.DNSName)
}

func (c *Config) APIEndpointURLPort() string {
	return fmt.Sprintf("https://%s:443", c.APIEndpoints.GetDefault().DNSName)
}

func (c *Config) AWSIAMAuthenticatorClusterIDRef() string {
	var rawClusterId string
	if c.Kubernetes.Authentication.AWSIAM.ClusterID != "" {
		rawClusterId = c.Kubernetes.Authentication.AWSIAM.ClusterID
	} else {
		rawClusterId = c.ClusterName
	}
	return fmt.Sprintf(`"%s"`, rawClusterId)
}

func (c *Config) IAMRoleARNs() []string {
	arns := []string{
		`{"Fn::GetAtt": ["IAMRoleController", "Arn"]}`,
	}

	for _, np := range c.NodePools {
		arns = append(arns, fmt.Sprintf(`{"Fn::ImportValue" : {"Fn::Sub" : "${NetworkStackName}-%sIAMRoleWorkerArn"}}`, np.NodePoolName))
	}

	return arns
}

func (c Config) VPCLogicalName() (string, error) {
	if c.VPC.HasIdentifier() {
		return "", fmt.Errorf("[BUG] .VPCLogicalName should not be called in stack template when vpc id is specified")
	}
	return vpcLogicalName, nil
}

func (c Config) VPCID() (string, error) {
	logger.Warn(".VPCID in stack template is deprecated and will be removed in v0.9.9. Please use .VPC.ID instead")
	if !c.VPC.HasIdentifier() {
		return "", fmt.Errorf("[BUG] .VPCID should not be called in stack template when vpc.id(FromStackOutput) is specified. Use .VPCManaged instead.")
	}
	return c.VPC.ID, nil
}

func (c Config) VPCManaged() bool {
	return !c.VPC.HasIdentifier()
}

func (c Config) VPCRef() (string, error) {
	return c.VPC.RefOrError(c.VPCLogicalName)
}

func (c Config) InternetGatewayLogicalName() string {
	return internetGatewayLogicalName
}

func (c Config) InternetGatewayRef() string {
	return c.InternetGateway.Ref(c.InternetGatewayLogicalName)
}

// Etcdadm returns the content of the etcdadm script to be embedded into cloud-config-etcd
func (c *Config) Etcdadm() (string, error) {
	return gzipcompressor.BytesToGzippedBase64String(builtin.Bytes("etcdadm/etcdadm"))
}

// ManageELBLogicalNames returns all the logical names of the cfn resources corresponding to ELBs managed by kube-aws for API endpoints
func (c *Config) ManagedELBLogicalNames() []string {
	return c.APIEndpoints.ManagedELBLogicalNames()
}

type kubernetesManifestPlugin struct {
	Manifests []*provisioner.RemoteFile
}

func (p kubernetesManifestPlugin) ManifestListFile() *provisioner.RemoteFile {
	paths := []string{}
	for _, m := range p.Manifests {
		paths = append(paths, m.Path)
	}
	bytes := []byte(strings.Join(paths, "\n"))
	return provisioner.NewRemoteFileAtPath(p.listFilePath(), bytes)
}

func (p kubernetesManifestPlugin) listFilePath() string {
	return "/srv/kube-aws/plugins/kubernetes-manifests"
}

func (p kubernetesManifestPlugin) Directory() string {
	return filepath.Dir(p.listFilePath())
}

type helmReleasePlugin struct {
	Releases []api.HelmReleaseFileset
}

func (p helmReleasePlugin) ReleaseListFile() *provisioner.RemoteFile {
	paths := []string{}
	for _, r := range p.Releases {
		paths = append(paths, r.ReleaseFile.Path)
	}
	bytes := []byte(strings.Join(paths, "\n"))
	return provisioner.NewRemoteFileAtPath(p.listFilePath(), bytes)
}

func (p helmReleasePlugin) listFilePath() string {
	return "/srv/kube-aws/plugins/helm-releases"
}

func (p helmReleasePlugin) Directory() string {
	return filepath.Dir(p.listFilePath())
}

func (c *Config) KubernetesManifestPlugin() kubernetesManifestPlugin {
	p := kubernetesManifestPlugin{
		Manifests: c.KubernetesManifestFiles,
	}
	return p
}

func (c *Config) HelmReleasePlugin() helmReleasePlugin {
	p := helmReleasePlugin{
		Releases: c.HelmReleaseFilesets,
	}
	return p
}

func WithTrailingDot(s string) string {
	if s == "" {
		return s
	}
	lastRune, _ := utf8.DecodeLastRuneInString(s)
	if lastRune != rune('.') {
		return s + "."
	}
	return s
}

func (c Config) NetworkStackName() string {
	if c.CloudFormation.StackNameOverrides.Network != "" {
		return c.CloudFormation.StackNameOverrides.Network
	}
	return networkStackName
}

func (c Config) EtcdStackName() string {
	if c.CloudFormation.StackNameOverrides.Etcd != "" {
		return c.CloudFormation.StackNameOverrides.Etcd
	}
	return etcdStackName
}
