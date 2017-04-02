package config

//go:generate go run ../../../codegen/templates_gen.go StackTemplateTemplate=stack-template.json
//go:generate gofmt -w templates.go

import (
	"fmt"
	"strings"

	"errors"
	"github.com/kubernetes-incubator/kube-aws/cfnresource"
	cfg "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/coreos/amiregistry"
	"github.com/kubernetes-incubator/kube-aws/filereader/userdatatemplate"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/model/derived"
	"gopkg.in/yaml.v2"
	"strconv"
)

type Ref struct {
	PoolName string
}

type ComputedConfig struct {
	ProvidedConfig

	// Fields computed from Cluster
	AMI string

	TLSConfig        *cfg.CompactTLSAssets
	AuthTokensConfig *cfg.CompactAuthTokens
}

type ProvidedConfig struct {
	MainClusterSettings
	cfg.KubeClusterSettings `yaml:",inline"`
	WorkerNodePoolConfig    `yaml:",inline"`
	DeploymentSettings      `yaml:",inline"`
	cfg.Experimental        `yaml:",inline"`
	Private                 bool   `yaml:"private,omitempty"`
	NodePoolName            string `yaml:"name,omitempty"`
	ProvidedEncryptService  cfg.EncryptService
}

type DeploymentSettings struct {
	cfg.DeploymentSettings `yaml:",inline"`
}

type MainClusterSettings struct {
	EtcdNodes []derived.EtcdNode
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
	return strings.Title(strings.Replace(c.StackName(), "-", "", -1))
}

func (c ProvidedConfig) StackConfig(opts StackTemplateOptions) (*StackConfig, error) {
	var err error
	stackConfig := StackConfig{}

	if stackConfig.ComputedConfig, err = c.Config(); err != nil {
		return nil, fmt.Errorf("failed to generate config : %v", err)
	}

	if stackConfig.ManageCertificates {
		if stackConfig.ComputedConfig.AssetsEncryptionEnabled() {
			compactAssets, _ := cfg.ReadOrCreateCompactTLSAssets(opts.AssetsDir, cfg.KMSConfig{
				Region:         stackConfig.ComputedConfig.Region,
				KMSKeyARN:      c.KMSKeyARN,
				EncryptService: c.ProvidedEncryptService,
			})
			stackConfig.ComputedConfig.TLSConfig = compactAssets
		} else {
			rawAssets, _ := cfg.ReadOrCreateUnencryptedCompactTLSAssets(opts.AssetsDir)
			stackConfig.ComputedConfig.TLSConfig = rawAssets
		}
	}

	if c.DeploymentSettings.Experimental.TLSBootstrap.Enabled {
		if stackConfig.ComputedConfig.AssetsEncryptionEnabled() {
			compactAuthTokens, _ := cfg.ReadOrCreateCompactAuthTokens(opts.AssetsDir, cfg.KMSConfig{
				Region:         stackConfig.ComputedConfig.Region,
				KMSKeyARN:      c.KMSKeyARN,
				EncryptService: c.ProvidedEncryptService,
			})
			stackConfig.ComputedConfig.AuthTokensConfig = compactAuthTokens
		} else {
			rawAuthTokens, _ := cfg.ReadOrCreateUnencryptedCompactAuthTokens(opts.AssetsDir)
			stackConfig.ComputedConfig.AuthTokensConfig = rawAuthTokens
		}
	} else {
		stackConfig.ComputedConfig.AuthTokensConfig = &cfg.CompactAuthTokens{}
	}

	if stackConfig.UserDataWorker, err = userdatatemplate.GetString(opts.WorkerTmplFile, stackConfig.ComputedConfig); err != nil {
		return nil, fmt.Errorf("failed to render worker cloud config: %v", err)
	}

	stackConfig.StackTemplateOptions = opts

	baseS3URI := strings.TrimSuffix(opts.S3URI, "/")
	stackConfig.S3URI = fmt.Sprintf("%s/kube-aws/clusters/%s/exported/stacks", baseS3URI, c.ClusterName)

	if opts.SkipWait {
		enabled := false
		stackConfig.WaitSignal.EnabledOverride = &enabled
	}

	return &stackConfig, nil
}

func newDefaultCluster() *ProvidedConfig {
	return &ProvidedConfig{
		WorkerNodePoolConfig: NewWorkerNodePoolConfig(),
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

func (c *ProvidedConfig) Load(main *cfg.Config) error {
	defaults := newDefaultCluster()
	if c.Count == nil {
		c.Count = defaults.Count
	}
	if c.SpotFleet.Enabled() {
		enabled := false
		c.WaitSignal.EnabledOverride = &enabled
	}

	c.WorkerNodePoolConfig = c.WorkerNodePoolConfig.WithDefaultsFrom(main.DefaultWorkerSettings)
	c.NodePoolConfig.SpotFleet = c.NodePoolConfig.SpotFleet.WithDefaults()
	c.DeploymentSettings = c.DeploymentSettings.WithDefaultsFrom(main.DeploymentSettings)

	// Inherit parameters from the control plane stack
	c.KubeClusterSettings = main.KubeClusterSettings

	// Inherit cluster TLS bootstrap config from control plane stack
	c.Experimental.TLSBootstrap = main.DeploymentSettings.Experimental.TLSBootstrap

	// Validate whole the inputs including inherited ones
	if err := c.valid(); err != nil {
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

	// Import all the managed subnets from the main cluster i.e. don't create subnets inside the node pool cfn stack
	for i, s := range c.Subnets {
		if !s.HasIdentifier() {
			stackOutputName := fmt.Sprintf(`{"Fn::ImportValue":{"Fn::Sub":"${ControlPlaneStackName}-%s"}}`, s.LogicalName())
			az := s.AvailabilityZone
			if s.Private {
				c.Subnets[i] = model.NewPrivateSubnetFromFn(az, stackOutputName)
			} else {
				c.Subnets[i] = model.NewPublicSubnetFromFn(az, stackOutputName)
			}
		}
	}

	c.EtcdNodes = main.EtcdNodes

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

func (c ProvidedConfig) WorkerDeploymentSettings() WorkerDeploymentSettings {
	return WorkerDeploymentSettings{
		WorkerNodePoolConfig: c.WorkerNodePoolConfig,
		Experimental:         c.Experimental,
		DeploymentSettings:   c.DeploymentSettings,
	}
}

func (c ProvidedConfig) ValidateInputs() error {
	if err := c.DeploymentSettings.ValidateInputs(); err != nil {
		return err
	}

	if err := c.WorkerNodePoolConfig.ValidateInputs(); err != nil {
		return err
	}

	if len(c.Subnets) > 1 && c.ClusterAutoscaler.Enabled() {
		return errors.New("cluster-autoscaler support can't be enabled for a node pool with 2 or more subnets because allowing so" +
			"results in unreliability while scaling nodes out. ")
	}

	return nil
}

func (c ProvidedConfig) valid() error {
	if _, err := c.KubeClusterSettings.Valid(); err != nil {
		return err
	}

	if err := c.WorkerNodePoolConfig.Validate(); err != nil {
		return err
	}

	if err := c.DeploymentSettings.Valid(); err != nil {
		return err
	}

	if err := c.WorkerDeploymentSettings().Valid(); err != nil {
		return err
	}

	if err := c.Experimental.Valid(); err != nil {
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

	if e := cfnresource.ValidateRoleNameLength(c.ClusterName, c.NestedStackName(), c.ManagedIamRoleName, c.Region.String()); e != nil {
		return e
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

func (c ProvidedConfig) VPCRef() string {
	//This means this VPC already exists, and we can reference it directly by ID
	if c.VPCID != "" {
		return fmt.Sprintf("%q", c.VPCID)
	}
	return `{"Fn::ImportValue" : {"Fn::Sub" : "${ControlPlaneStackName}-VPC"}}`
}

func (c ProvidedConfig) SecurityGroupRefs() []string {
	refs := c.WorkerDeploymentSettings().WorkerSecurityGroupRefs()

	refs = append(
		refs,
		// The security group assigned to worker nodes to allow communication to etcd nodes and controller nodes
		// which is created and maintained in the main cluster and then imported to node pools.
		`{"Fn::ImportValue" : {"Fn::Sub" : "${ControlPlaneStackName}-WorkerSecurityGroup"}}`,
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

	if c.NodePoolConfig.ClusterAutoscaler.Enabled() {
		tags["kube-aws:cluster-autoscaler:logical-name"] = c.NodePoolConfig.LogicalName()
		tags["kube-aws:cluster-autoscaler:min-size"] = strconv.Itoa(c.NodePoolConfig.ClusterAutoscaler.MinSize)
		tags["kube-aws:cluster-autoscaler:max-size"] = strconv.Itoa(c.NodePoolConfig.ClusterAutoscaler.MaxSize)
	}

	return tags
}

func (c WorkerDeploymentSettings) Valid() error {
	sgRefs := c.WorkerSecurityGroupRefs()
	numSGs := len(sgRefs)

	if numSGs > 4 {
		return fmt.Errorf("number of user provided security groups must be less than or equal to 4 but was %d (actual EC2 limit is 5 but one of them is reserved for kube-aws) : %v", numSGs, sgRefs)
	}

	return nil
}
