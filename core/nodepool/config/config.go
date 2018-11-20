package config

import (
	"fmt"
	"strings"

	"github.com/go-yaml/yaml"

	"errors"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/kubernetes-incubator/kube-aws/cfnresource"
	cfg "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/coreos/amiregistry"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/model/derived"
	"github.com/kubernetes-incubator/kube-aws/naming"
)

type Ref struct {
	PoolName string
}

type ComputedConfig struct {
	ProvidedConfig

	// Fields computed from Cluster
	AMI string

	AssetsConfig *cfg.CompactAssets
}

type ProvidedConfig struct {
	MainClusterSettings
	// APIEndpoint is the k8s api endpoint to which worker nodes in this node pool communicate
	APIEndpoint             derived.APIEndpoint
	cfg.KubeClusterSettings `yaml:",inline"`
	WorkerNodePoolConfig    `yaml:",inline"`
	DeploymentSettings      `yaml:",inline"`
	cfg.Experimental        `yaml:",inline"`
	cfg.Kubelet             `yaml:",inline"`
	Plugins                 model.PluginConfigs `yaml:"kubeAwsPlugins,omitempty"`
	Private                 bool                `yaml:"private,omitempty"`
	NodePoolName            string              `yaml:"name,omitempty"`
	ProvidedEncryptService  cfg.EncryptService
	model.UnknownKeys       `yaml:",inline"`
}

type DeploymentSettings struct {
	cfg.DeploymentSettings `yaml:",inline"`
}

type MainClusterSettings struct {
	EtcdNodes             []derived.EtcdNode
	KubeResourcesAutosave cfg.KubeResourcesAutosave
}

type StackTemplateOptions struct {
	WorkerTmplFile        string
	StackTemplateTmplFile string
	AssetsDir             string
	PrettyPrint           bool
	S3URI                 string
	SkipWait              bool
}

// NestedStackName returns a sanitized name of this node pool which is usable as a valid cloudformation nested stack name
func (c ProvidedConfig) NestedStackName() string {
	// Convert stack name into something valid as a cfn resource name or
	// we'll end up with cfn errors like "Template format error: Resource name test5-controlplane is non alphanumeric"
	return naming.FromStackToCfnResource(c.StackName())
}

func (c ProvidedConfig) StackConfig(opts StackTemplateOptions, session *session.Session) (*StackConfig, error) {
	var err error
	stackConfig := StackConfig{
		ExtraCfnResources: map[string]interface{}{},
	}

	if stackConfig.ComputedConfig, err = c.Config(); err != nil {
		return nil, fmt.Errorf("failed to generate config : %v", err)
	}

	tlsBootstrappingEnabled := c.Experimental.TLSBootstrap.Enabled
	if stackConfig.ComputedConfig.AssetsEncryptionEnabled() {
		kmsConfig := cfg.NewKMSConfig(c.KMSKeyARN, c.ProvidedEncryptService, session)
		compactAssets, err := cfg.ReadOrCreateCompactAssets(opts.AssetsDir, c.ManageCertificates, tlsBootstrappingEnabled, false, kmsConfig)
		if err != nil {
			return nil, err
		}
		stackConfig.ComputedConfig.AssetsConfig = compactAssets
	} else {
		rawAssets, _ := cfg.ReadOrCreateUnencryptedCompactAssets(opts.AssetsDir, c.ManageCertificates, tlsBootstrappingEnabled, false)
		stackConfig.ComputedConfig.AssetsConfig = rawAssets
	}

	stackConfig.StackTemplateOptions = opts

	s3Folders := model.NewS3Folders(opts.S3URI, c.ClusterName)
	stackConfig.S3URI = s3Folders.ClusterExportedStacks().URI()
	stackConfig.KubeResourcesAutosave.S3Path = s3Folders.ClusterBackups().Path()

	if opts.SkipWait {
		enabled := false
		stackConfig.WaitSignal.EnabledOverride = &enabled
	}

	return &stackConfig, nil
}

func newDefaultCluster() *ProvidedConfig {
	return &ProvidedConfig{
		WorkerNodePoolConfig: newWorkerNodePoolConfig(),
	}
}

// ClusterFromBytes Necessary for unit tests, which store configs as hardcoded strings
func ClusterFromBytes(data []byte, main *cfg.Config) (*ProvidedConfig, error) {
	c := newDefaultCluster()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("failed to parse cluster: %v", err)
	}

	if err := c.ValidateInputs(); err != nil {
		return nil, fmt.Errorf("failed to validate cluster: %v", err)
	}

	if err := c.Load(main); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *ProvidedConfig) ExternalDNSName() string {
	logger.Warn("WARN: ExternalDNSName is deprecated and will be removed in v0.9.7. Please use APIEndpoint.Name instead")
	return c.APIEndpoint.DNSName
}

func (c *ProvidedConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type t ProvidedConfig
	work := t(*newDefaultCluster())
	if err := unmarshal(&work); err != nil {
		return fmt.Errorf("failed to parse node pool config: %v", err)
	}
	*c = ProvidedConfig(work)

	return nil
}

func (c *ProvidedConfig) Load(main *cfg.Config) error {
	if c.SpotFleet.Enabled() {
		enabled := false
		c.WaitSignal.EnabledOverride = &enabled
	}

	c.WorkerNodePoolConfig = c.WorkerNodePoolConfig.WithDefaultsFrom(main.DefaultWorkerSettings)
	c.DeploymentSettings = c.DeploymentSettings.WithDefaultsFrom(main.DeploymentSettings)

	// Inherit parameters from the control plane stack
	c.KubeClusterSettings = main.KubeClusterSettings
	c.HostOS = main.HostOS
	c.Experimental.TLSBootstrap = main.DeploymentSettings.Experimental.TLSBootstrap
	c.Experimental.NodeDrainer = main.DeploymentSettings.Experimental.NodeDrainer
	c.Experimental.GpuSupport = main.DeploymentSettings.Experimental.GpuSupport
	c.Kubelet.RotateCerts = main.DeploymentSettings.Kubelet.RotateCerts
	c.Kubelet.SystemReservedResources = main.DeploymentSettings.Kubelet.SystemReservedResources
	c.Kubelet.KubeReservedResources = main.DeploymentSettings.Kubelet.KubeReservedResources

	if c.Experimental.ClusterAutoscalerSupport.Enabled {
		if !main.Addons.ClusterAutoscaler.Enabled {
			return fmt.Errorf("clusterAutoscalerSupport can't be enabled on node pools when cluster-autoscaler is not going to be deployed to the cluster")
		}
	}

	// Validate whole the inputs including inherited ones
	if err := c.validate(); err != nil {
		return err
	}

	// Default to public subnets defined in the main cluster
	// CAUTION: cluster-autoscaler Won't work if there're 2 or more subnets spanning over different AZs
	if len(c.Subnets) == 0 {
		var defaults []model.Subnet
		if c.Private {
			defaults = main.PrivateSubnets()
		} else {
			defaults = main.PublicSubnets()
		}
		if len(defaults) == 0 {
			return errors.New(`public subnets required by default for node pool missing in cluster.yaml.
define one or more public subnets in cluster.yaml or explicitly reference private subnets from node pool by specifying nodePools[].subnets[].name`)
		}
		c.Subnets = defaults
	} else {
		// Fetch subnets defined in the main cluster by name
		for i, s := range c.Subnets {
			linkedSubnet := main.FindSubnetMatching(s)
			c.Subnets[i] = linkedSubnet
		}
	}

	// Import all the managed subnets from the network stack i.e. don't create subnets inside the node pool cfn stack
	var err error
	c.Subnets, err = c.Subnets.ImportFromNetworkStack()
	if err != nil {
		return fmt.Errorf("failed to import subnets from network stack: %v", err)
	}

	anySubnetIsManaged := false
	for _, s := range c.Subnets {
		anySubnetIsManaged = anySubnetIsManaged || s.ManageSubnet()
	}

	if anySubnetIsManaged && main.ElasticFileSystemID == "" && c.ElasticFileSystemID != "" {
		return fmt.Errorf("elasticFileSystemId cannot be specified for a node pool in managed subnet(s), but was: %s", c.ElasticFileSystemID)
	}

	c.EtcdNodes = main.EtcdNodes
	c.KubeResourcesAutosave = main.KubeResourcesAutosave

	var apiEndpoint derived.APIEndpoint
	if c.APIEndpointName != "" {
		found, err := main.APIEndpoints.FindByName(c.APIEndpointName)
		if err != nil {
			return fmt.Errorf("failed to find an API endpoint named \"%s\": %v", c.APIEndpointName, err)
		}
		apiEndpoint = *found
	} else {
		if len(main.APIEndpoints) > 1 {
			return errors.New("worker.nodePools[].apiEndpointName must not be empty when there's 2 or more api endpoints under the key `apiEndpoints")
		}
		apiEndpoint = main.APIEndpoints.GetDefault()
	}

	if !apiEndpoint.LoadBalancer.ManageELBRecordSet() {
		fmt.Printf(`WARN: the worker node pool "%s" is associated to a k8s API endpoint behind the DNS name "%s" managed by YOU!
Please never point the DNS record for it to a different k8s cluster, especially when the name is a "stable" one which is shared among multiple k8s clusters for achieving blue-green deployments of k8s clusters!
kube-aws can't save users from mistakes like that
`, c.NodePoolName, apiEndpoint.DNSName)
	}
	c.APIEndpoint = apiEndpoint

	return nil
}

func ClusterFromBytesWithEncryptService(data []byte, main *cfg.Config, encryptService cfg.EncryptService) (*ProvidedConfig, error) {
	cluster, err := ClusterFromBytes(data, main)
	if err != nil {
		return nil, err
	}
	cluster.ProvidedEncryptService = encryptService
	return cluster, nil
}

// APIEndpointURL is the url of the API endpoint which is written in cloud-config-worker and used by kubelets in worker nodes
// to access the apiserver
func (c ProvidedConfig) APIEndpointURL() string {
	return fmt.Sprintf("https://%s", c.APIEndpoint.DNSName)
}

func (c ProvidedConfig) Config() (*ComputedConfig, error) {
	config := ComputedConfig{ProvidedConfig: c}

	if c.AmiId == "" {
		var err error
		if config.AMI, err = amiregistry.GetAMI(config.Region.String(), config.ReleaseChannel); err != nil {
			return nil, fmt.Errorf("failed getting AMI for config: %v", err)
		}
	} else {
		config.AMI = c.AmiId
	}

	return &config, nil
}

func (c ProvidedConfig) NodeLabels() model.NodeLabels {
	labels := c.NodeSettings.NodeLabels
	if c.ClusterAutoscalerSupport.Enabled {
		labels["kube-aws.coreos.com/cluster-autoscaler-supported"] = "true"
	}
	return labels
}

func (c ProvidedConfig) FeatureGates() model.FeatureGates {
	gates := c.NodeSettings.FeatureGates
	if gates == nil {
		gates = model.FeatureGates{}
	}
	if c.Gpu.Nvidia.IsEnabledOn(c.InstanceType) {
		gates["Accelerators"] = "true"
	}
	if c.Experimental.GpuSupport.Enabled {
		gates["DevicePlugins"] = "true"
	}
	if c.Kubelet.RotateCerts.Enabled {
		gates["RotateKubeletClientCertificate"] = "true"
	}
	//From kube 1.11 PodPriority and ExpandPersistentVolumes have become enabled by default,
	//so making sure it is not enabled if user has explicitly set them to false
	//https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG-1.11.md#changelog-since-v1110
	if !c.Experimental.Admission.Priority.Enabled {
		gates["PodPriority"] = "false"
	}
	if !c.Experimental.Admission.PersistentVolumeClaimResize.Enabled {
		gates["ExpandPersistentVolumes"] = "false"
	}
	return gates
}

func (c ProvidedConfig) WorkerDeploymentSettings() WorkerDeploymentSettings {
	return WorkerDeploymentSettings{
		WorkerNodePoolConfig: c.WorkerNodePoolConfig,
		Experimental:         c.Experimental,
		DeploymentSettings:   c.DeploymentSettings,
	}
}

func (c ProvidedConfig) ValidateInputs() error {
	if err := c.DeploymentSettings.ValidateInputs(c.NodePoolName); err != nil {
		return err
	}

	if err := c.WorkerNodePoolConfig.ValidateInputs(); err != nil {
		return err
	}

	if len(c.Subnets) > 1 && c.Autoscaling.ClusterAutoscaler.Enabled {
		return errors.New("cluster-autoscaler can't be enabled for a node pool with 2 or more subnets because allowing so" +
			"results in unreliability while scaling nodes out. ")
	}

	return nil
}

func (c ProvidedConfig) validate() error {
	if _, err := c.KubeClusterSettings.Validate(); err != nil {
		return err
	}

	if err := c.WorkerNodePoolConfig.Validate(c.Experimental); err != nil {
		return err
	}

	if err := c.DeploymentSettings.Validate(c.NodePoolName); err != nil {
		return err
	}

	if err := c.WorkerDeploymentSettings().Validate(); err != nil {
		return err
	}

	if err := c.Experimental.Validate(c.NodePoolName); err != nil {
		return err
	}

	if err := c.NodeSettings.Validate(); err != nil {
		return err
	}

	clusterNamePlaceholder := "<my-cluster-name>"
	nestedStackNamePlaceHolder := "<my-nested-stack-name>"
	replacer := strings.NewReplacer(clusterNamePlaceholder, "", nestedStackNamePlaceHolder, "")
	simulatedLcName := fmt.Sprintf("%s-%s-1N2C4K3LLBEDZ-%sLC-BC2S9P3JG2QD", clusterNamePlaceholder, nestedStackNamePlaceHolder, c.LogicalName())
	limit := 63 - len(replacer.Replace(simulatedLcName))
	if c.Experimental.AwsNodeLabels.Enabled && len(c.ClusterName+c.NodePoolName) > limit {
		return fmt.Errorf("awsNodeLabels can't be enabled for node pool because the total number of characters in clusterName(=\"%s\") + node pool's name(=\"%s\") exceeds the limit of %d", c.ClusterName, c.NodePoolName, limit)
	}

	if len(c.WorkerNodePoolConfig.IAMConfig.Role.Name) > 0 {
		if e := cfnresource.ValidateStableRoleNameLength(c.ClusterName, c.WorkerNodePoolConfig.IAMConfig.Role.Name, c.Region.String(), c.WorkerNodePoolConfig.IAMConfig.Role.StrictName); e != nil {
			return e
		}
	} else {
		if e := cfnresource.ValidateUnstableRoleNameLength(c.ClusterName, c.NestedStackName(), c.WorkerNodePoolConfig.IAMConfig.Role.Name, c.Region.String(), c.WorkerNodePoolConfig.IAMConfig.Role.StrictName); e != nil {
			return e
		}
	}

	return nil
}

// StackName returns the logical name of a CloudFormation stack resource in a root stack template
// This is not needed to be unique in an AWS account because the actual name of a nested stack is generated randomly
// by CloudFormation by including the logical name.
// This is NOT intended to be used to reference stack name from cloud-config as the target of awscli or cfn-bootstrap-tools commands e.g. `cfn-init` and `cfn-signal`
func (c ProvidedConfig) StackName() string {
	return c.NodePoolName
}

func (c ProvidedConfig) StackNameEnvFileName() string {
	return "/etc/environment"
}

func (c ProvidedConfig) StackNameEnvVarName() string {
	return "KUBE_AWS_STACK_NAME"
}

func (c ProvidedConfig) VPCRef() (string, error) {
	igw := c.InternetGateway
	// When HasIdentifier returns true, it means the VPC already exists, and we can reference it directly by ID
	if !c.VPC.HasIdentifier() {
		// Otherwise import the VPC ID from the control-plane stack
		igw.IDFromStackOutput = `{"Fn::Sub" : "${NetworkStackName}-VPC"}`
	}
	return igw.RefOrError(func() (string, error) {
		return "", fmt.Errorf("[BUG] Tried to reference VPC by its logical name")
	})
}

func (c ProvidedConfig) SecurityGroupRefs() []string {
	refs := c.WorkerDeploymentSettings().WorkerSecurityGroupRefs()

	refs = append(
		refs,
		// The security group assigned to worker nodes to allow communication to etcd nodes and controller nodes
		// which is created and maintained in the main cluster and then imported to node pools.
		`{"Fn::ImportValue" : {"Fn::Sub" : "${NetworkStackName}-WorkerSecurityGroup"}}`,
	)

	return refs
}

type WorkerDeploymentSettings struct {
	WorkerNodePoolConfig
	cfg.Experimental
	DeploymentSettings
}

func (c WorkerDeploymentSettings) WorkerSecurityGroupRefs() []string {
	refs := []string{}

	if c.Experimental.LoadBalancer.Enabled {
		for _, sgId := range c.Experimental.LoadBalancer.SecurityGroupIds {
			refs = append(refs, fmt.Sprintf(`"%s"`, sgId))
		}
	}

	if c.Experimental.TargetGroup.Enabled {
		for _, sgId := range c.Experimental.TargetGroup.SecurityGroupIds {
			refs = append(refs, fmt.Sprintf(`"%s"`, sgId))
		}
	}

	for _, sgId := range c.SecurityGroupIds {
		refs = append(refs, fmt.Sprintf(`"%s"`, sgId))
	}

	return refs
}

func (c WorkerDeploymentSettings) StackTags() map[string]string {
	tags := map[string]string{}

	for k, v := range c.DeploymentSettings.StackTags {
		tags[k] = v
	}

	return tags
}

func (c WorkerDeploymentSettings) Validate() error {
	sgRefs := c.WorkerSecurityGroupRefs()
	numSGs := len(sgRefs)

	if numSGs > 4 {
		return fmt.Errorf("number of user provided security groups must be less than or equal to 4 but was %d (actual EC2 limit is 5 but one of them is reserved for kube-aws) : %v", numSGs, sgRefs)
	}

	return nil
}
