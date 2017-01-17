package config

//go:generate go run ../../codegen/templates_gen.go DefaultClusterConfig=cluster.yaml StackTemplateTemplate=stack-template.json
//go:generate gofmt -w templates.go

import (
	"errors"
	"fmt"
	"io/ioutil"

	cfg "github.com/coreos/kube-aws/config"
	"github.com/coreos/kube-aws/coreos/amiregistry"
	"github.com/coreos/kube-aws/coreos/userdatavalidation"
	"github.com/coreos/kube-aws/filereader/jsontemplate"
	"github.com/coreos/kube-aws/filereader/userdatatemplate"
	model "github.com/coreos/kube-aws/model"
	"gopkg.in/yaml.v2"
)

type Ref struct {
	PoolName string
}

type ComputedConfig struct {
	ProvidedConfig
	// Fields computed from Cluster
	AMI       string
	TLSConfig *cfg.CompactTLSAssets
}

type ProvidedConfig struct {
	cfg.KubeClusterSettings `yaml:",inline"`
	cfg.WorkerSettings      `yaml:",inline"`
	cfg.DeploymentSettings  `yaml:",inline"`
	EtcdEndpoints           string `yaml:"etcdEndpoints,omitempty"`
	NodePoolName            string `yaml:"nodePoolName,omitempty"`
	providedEncryptService  cfg.EncryptService
}

type StackTemplateOptions struct {
	WorkerTmplFile        string
	StackTemplateTmplFile string
	TLSAssetsDir          string
}

type stackConfig struct {
	*ComputedConfig
	UserDataWorker string
}

func (c ProvidedConfig) stackConfig(opts StackTemplateOptions, compressUserData bool) (*stackConfig, error) {
	var err error
	stackConfig := stackConfig{}

	if stackConfig.ComputedConfig, err = c.Config(); err != nil {
		return nil, err
	}

	compactAssets, err := cfg.ReadOrCreateCompactTLSAssets(opts.TLSAssetsDir, cfg.KMSConfig{
		Region:         stackConfig.ComputedConfig.Region,
		KMSKeyARN:      c.KMSKeyARN,
		EncryptService: c.providedEncryptService,
	})

	stackConfig.ComputedConfig.TLSConfig = compactAssets

	if stackConfig.UserDataWorker, err = userdatatemplate.GetString(opts.WorkerTmplFile, stackConfig.ComputedConfig, compressUserData); err != nil {
		return nil, fmt.Errorf("failed to render worker cloud config: %v", err)
	}

	return &stackConfig, nil
}

func (c ProvidedConfig) ValidateUserData(opts StackTemplateOptions) error {
	stackConfig, err := c.stackConfig(opts, false)
	if err != nil {
		return fmt.Errorf("failed to create stack config: %v", err)
	}

	err = userdatavalidation.Execute([]userdatavalidation.Entry{
		{Name: "UserDataWorker", Content: stackConfig.UserDataWorker},
	})

	return err
}

func (c ProvidedConfig) RenderStackTemplate(opts StackTemplateOptions, prettyPrint bool) ([]byte, error) {
	stackConfig, err := c.stackConfig(opts, true)
	if err != nil {
		return nil, err
	}

	bytes, err := jsontemplate.GetBytes(opts.StackTemplateTmplFile, stackConfig, prettyPrint)

	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// Backwards compatibility
// TODO: Delete at 1.0
func (c *ProvidedConfig) fillLegacySettings() error {
	if c.VPCID != "" {
		if c.VPC.ID != "" {
			return errors.New("Cannot setup VPCID and VPC.ID")
		}
		c.VPC.ID = c.VPCID
	}
	if c.VPCCIDR != "" {
		c.VPC.CIDR = c.VPCCIDR
	}
	if c.RouteTableID != "" {
		if c.RouteTable.ID != "" {
			return errors.New("Cannot setup RouteTableID and RouteTable.ID")
		}
		c.RouteTable.ID = c.RouteTableID
	}

	if c.InstanceCIDR != "" && len(c.Subnets) > 0 && c.Subnets[0].InstanceCIDR != "" {
		return errors.New("Cannot setup Subnets[0].InstanceCIDR and InstanceCIDR")
	}

	if len(c.Subnets) > 0 {
		if c.AvailabilityZone != "" {
			return fmt.Errorf("The top-level availabilityZone(%s) must be empty when subnets are specified", c.AvailabilityZone)
		}
		if c.InstanceCIDR != "" {
			return fmt.Errorf("The top-level instanceCIDR(%s) must be empty when subnets are specified", c.InstanceCIDR)
		}
	}

	if len(c.Subnets) == 0 {
		if c.AvailabilityZone == "" {
			return errors.New("Must specify top-level availability zone if no subnets specified")
		}
		if c.InstanceCIDR == "" {
			c.InstanceCIDR = "10.0.1.0/24"
		}
		c.Subnets = append(c.Subnets, &model.PublicSubnet{
			Subnet: model.Subnet{
				AvailabilityZone: c.AvailabilityZone,
				InstanceCIDR:     c.InstanceCIDR,
			},
			MapPublicIp: c.MapPublicIPs,
		})
	}

	return nil
}

func ClusterFromFile(filename string) (*ProvidedConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	c, err := ClusterFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("file %s: %v", filename, err)
	}

	return c, nil
}

func NewDefaultCluster() *ProvidedConfig {
	defaults := cfg.NewDefaultCluster()

	return &ProvidedConfig{
		DeploymentSettings: defaults.DeploymentSettings,
		WorkerSettings:     defaults.WorkerSettings,
	}
}

// ClusterFromBytes Necessary for unit tests, which store configs as hardcoded strings
func ClusterFromBytes(data []byte) (*ProvidedConfig, error) {
	c := NewDefaultCluster()
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("failed to parse cluster: %v", err)
	}

	if err := c.fillLegacySettings(); err != nil {
		return nil, err
	}

	//Computed defaults
	launchSpecs := []model.LaunchSpecification{}
	for _, spec := range c.Worker.SpotFleet.LaunchSpecifications {
		if spec.RootVolumeType == "" {
			spec.RootVolumeType = c.Worker.SpotFleet.RootVolumeType
		}
		if spec.RootVolumeSize == 0 {
			spec.RootVolumeSize = c.Worker.SpotFleet.UnitRootVolumeSize * spec.WeightedCapacity
		}
		if spec.RootVolumeType == "io1" && spec.RootVolumeIOPS == 0 {
			spec.RootVolumeIOPS = c.Worker.SpotFleet.UnitRootVolumeIOPS * spec.WeightedCapacity
		}
		launchSpecs = append(launchSpecs, spec)
	}
	c.Worker.SpotFleet.LaunchSpecifications = launchSpecs

	if err := c.valid(); err != nil {
		return nil, fmt.Errorf("invalid cluster: %v", err)
	}

	return c, nil
}

func (c ProvidedConfig) Config() (*ComputedConfig, error) {
	config := ComputedConfig{ProvidedConfig: c}

	if c.AmiId == "" {
		var err error
		if config.AMI, err = amiregistry.GetAMI(config.Region, config.ReleaseChannel); err != nil {
			return nil, fmt.Errorf("failed getting AMI for config: %v", err)
		}
	} else {
		config.AMI = c.AmiId
	}

	return &config, nil
}

func (c ProvidedConfig) WorkerDeploymentSettings() cfg.WorkerDeploymentSettings {
	return cfg.WorkerDeploymentSettings{
		WorkerSettings:     c.WorkerSettings,
		DeploymentSettings: c.DeploymentSettings,
	}
}

func (c ProvidedConfig) valid() error {
	if _, err := c.DeploymentSettings.Valid(); err != nil {
		return err
	}

	if _, err := c.KubeClusterSettings.Valid(); err != nil {
		return err
	}

	if err := c.WorkerSettings.Valid(); err != nil {
		return err
	}

	if err := c.Worker.Valid(); err != nil {
		return err
	}

	if err := c.WorkerDeploymentSettings().Valid(); err != nil {
		return err
	}

	return nil
}

// CloudFormation stack name which is unique in an AWS account.
// This is intended to be used to reference stack name from cloud-config as the target of awscli or cfn-bootstrap-tools commands e.g. `cfn-init` and `cfn-signal`
func (c ProvidedConfig) StackName() string {
	return c.ClusterName + "-" + c.NodePoolName
}

func (c ComputedConfig) WorkerSecurityGroupRefs() []string {
	refs := c.WorkerDeploymentSettings().WorkerSecurityGroupRefs()

	refs = append(
		refs,
		// The security group assigned to worker nodes to allow communication to etcd nodes and controller nodes
		// which is created and maintained in the main cluster and then imported to node pools.
		fmt.Sprintf(`{"Fn::ImportValue" : {"Fn::Sub" : "%s-WorkerSecurityGroup"}}`, c.ClusterName),
	)

	return refs
}
