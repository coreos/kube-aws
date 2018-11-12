package api

import (
	"fmt"

	"errors"
	"github.com/kubernetes-incubator/kube-aws/logger"
)

type WorkerNodePool struct {
	Experimental        `yaml:",inline"`
	Kubelet             `yaml:",inline"`
	KubeClusterSettings `yaml:",inline"`
	DeploymentSettings  `yaml:",inline"`

	Plugins      PluginConfigs `yaml:"kubeAwsPlugins,omitempty"`
	Private      bool          `yaml:"private,omitempty"`
	NodePoolName string        `yaml:"name,omitempty"`

	APIEndpointName           string           `yaml:"apiEndpointName,omitempty"`
	Autoscaling               Autoscaling      `yaml:"autoscaling,omitempty"`
	AutoScalingGroup          AutoScalingGroup `yaml:"autoScalingGroup,omitempty"`
	SpotFleet                 SpotFleet        `yaml:"spotFleet,omitempty"`
	EC2Instance               `yaml:",inline"`
	IAMConfig                 IAMConfig              `yaml:"iam,omitempty"`
	SpotPrice                 string                 `yaml:"spotPrice,omitempty"`
	SecurityGroupIds          []string               `yaml:"securityGroupIds,omitempty"`
	CustomSettings            map[string]interface{} `yaml:"customSettings,omitempty"`
	VolumeMounts              []NodeVolumeMount      `yaml:"volumeMounts,omitempty"`
	Raid0Mounts               []Raid0Mount           `yaml:"raid0Mounts,omitempty"`
	NodeSettings              `yaml:",inline"`
	NodeStatusUpdateFrequency string              `yaml:"nodeStatusUpdateFrequency"`
	CustomFiles               []CustomFile        `yaml:"customFiles,omitempty"`
	CustomSystemdUnits        []CustomSystemdUnit `yaml:"customSystemdUnits,omitempty"`
	Gpu                       Gpu                 `yaml:"gpu"`
	NodePoolRollingStrategy   string              `yaml:"nodePoolRollingStrategy,omitempty"`
	UnknownKeys               `yaml:",inline"`
}

func (c *WorkerNodePool) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type t WorkerNodePool
	work := t(NewDefaultNodePoolConfig())
	if err := unmarshal(&work); err != nil {
		return fmt.Errorf("failed to parse node pool config: %v", err)
	}
	*c = WorkerNodePool(work)

	return nil
}

type ClusterAutoscaler struct {
	Enabled     bool `yaml:"enabled,omitempty"`
	UnknownKeys `yaml:",inline"`
}

func (a ClusterAutoscaler) AutoDiscoveryTagKey() string {
	return "k8s.io/cluster-autoscaler/enabled"
}

func NewDefaultNodePoolConfig() WorkerNodePool {
	return WorkerNodePool{
		SpotFleet: newDefaultSpotFleet(),
		EC2Instance: EC2Instance{
			Count:         1,
			CreateTimeout: "PT15M",
			InstanceType:  "t2.medium",
			RootVolume: RootVolume{
				Type: "gp2",
				IOPS: 0,
				Size: 30,
			},
			Tenancy: "default",
		},
		NodeSettings:     newNodeSettings(),
		SecurityGroupIds: []string{},
		Gpu:              newDefaultGpu(),
	}
}

func newDefaultSpotFleet() SpotFleet {
	return SpotFleet{
		SpotPrice:          "0.06",
		UnitRootVolumeSize: 30,
		RootVolumeType:     "gp2",
		LaunchSpecifications: []LaunchSpecification{
			NewLaunchSpecification(1, "c4.large"),
			NewLaunchSpecification(2, "c4.xlarge"),
		},
	}
}

func (c WorkerNodePool) LogicalName() string {
	return "Workers"
}

func (c WorkerNodePool) LaunchConfigurationLogicalName() string {
	return c.LogicalName() + "LC"
}

func (c WorkerNodePool) validate(experimentalGpuSupportEnabled bool) error {
	// one is the default WorkerCount
	if c.Count != 1 && (c.AutoScalingGroup.MinSize != nil && *c.AutoScalingGroup.MinSize != 0 || c.AutoScalingGroup.MaxSize != 0) {
		return fmt.Errorf("`worker.autoScalingGroup.minSize` and `worker.autoScalingGroup.maxSize` can only be specified without `count`=%d", c.Count)
	}

	if err := c.AutoScalingGroup.Validate(); err != nil {
		return err
	}

	if c.Tenancy != "default" && c.SpotFleet.Enabled() {
		return fmt.Errorf("selected worker tenancy (%s) is incompatible with spot fleet", c.Tenancy)
	}

	if c.Tenancy != "default" && c.SpotPrice != "" {
		return fmt.Errorf("selected worker tenancy (%s) is incompatible with spot instances", c.Tenancy)
	}

	if err := c.RootVolume.Validate(); err != nil {
		return err
	}

	if err := c.SpotFleet.Validate(); c.SpotFleet.Enabled() && err != nil {
		return err
	}

	if err := ValidateVolumeMounts(c.VolumeMounts); err != nil {
		return err
	}

	// c.VolumeMounts are supplied to check for device and path overlaps with contents of c.Raid0Mounts.
	if err := ValidateRaid0Mounts(c.VolumeMounts, c.Raid0Mounts); err != nil {
		return err
	}

	if c.InstanceType == "t2.micro" || c.InstanceType == "t2.nano" {
		logger.Warnf(`instance types "t2.nano" and "t2.micro" are not recommended. See https://github.com/kubernetes-incubator/kube-aws/issues/258 for more information`)
	}

	if err := c.IAMConfig.Validate(); err != nil {
		return err
	}

	if err := c.Gpu.Validate(c.InstanceType, experimentalGpuSupportEnabled); err != nil {
		return err
	}

	// By design, kube-aws doesn't allow customizing the following settings among node pools.
	//
	// Every node pool imports subnets from the main stack and therefore there's no need for setting:
	// * VPC.ID(FromStackOutput)
	// * InternetGateway.ID(FromStackOutput)
	// * RouteTableID
	// * VPCCIDR
	// * InstanceCIDR
	// * MapPublicIPs
	// * ElasticFileSystemID
	if c.VPC.HasIdentifier() {
		return fmt.Errorf("although you can't customize VPC per node pool but you did specify \"%v\" in your cluster.yaml", c.VPC)
	}
	if c.InternetGateway.HasIdentifier() {
		return fmt.Errorf("although you can't customize internet gateway per node pool but you did specify \"%v\" in your cluster.yaml", c.InternetGateway)
	}
	if c.VPCCIDR != "" {
		return fmt.Errorf("although you can't customize `vpcCIDR` per node pool but you did specify \"%s\" in your cluster.yaml", c.VPCCIDR)
	}
	if c.InstanceCIDR != "" {
		return fmt.Errorf("although you can't customize `instanceCIDR` per node pool but you did specify \"%s\" in your cluster.yaml", c.InstanceCIDR)
	}

	// Believing it is impossible to mix different values, we also forbid customization of:
	// * Region
	// * ContainerRuntime
	// * KMSKeyARN

	if !c.Region.IsEmpty() {
		return fmt.Errorf("although you can't customize `region` per node pool but you did specify \"%s\" in your cluster.yaml", c.Region)
	}
	if c.ContainerRuntime != "" {
		return fmt.Errorf("although you can't customize `containerRuntime` per node pool but you did specify \"%s\" in your cluster.yaml", c.ContainerRuntime)
	}
	if c.KMSKeyARN != "" {
		return fmt.Errorf("although you can't customize `kmsKeyArn` per node pool but you did specify \"%s\" in your cluster.yaml", c.KMSKeyARN)
	}

	if err := c.Experimental.Validate(c.NodePoolName); err != nil {
		return err
	}

	if len(c.Subnets) > 1 && c.Autoscaling.ClusterAutoscaler.Enabled {
		return errors.New("cluster-autoscaler can't be enabled for a node pool with 2 or more subnets because allowing so" +
			"results in unreliability while scaling nodes out. ")
	}

	return nil
}

func (c WorkerNodePool) MinCount() int {
	if c.AutoScalingGroup.MinSize == nil {
		return c.Count
	}
	return *c.AutoScalingGroup.MinSize
}

func (c WorkerNodePool) MaxCount() int {
	if c.AutoScalingGroup.MaxSize == 0 {
		return c.MinCount()
	}
	return c.AutoScalingGroup.MaxSize
}

func (c WorkerNodePool) RollingUpdateMinInstancesInService() int {
	if c.AutoScalingGroup.RollingUpdateMinInstancesInService == nil {
		if c.MaxCount() > 0 {
			return c.MaxCount() - 1
		}
		return 0
	}
	return *c.AutoScalingGroup.RollingUpdateMinInstancesInService
}

func (c WorkerNodePool) Validate(experimental Experimental) error {
	return c.validate(experimental.GpuSupport.Enabled)
}

func (c WorkerNodePool) WithDefaultsFrom(main DefaultWorkerSettings) WorkerNodePool {
	if c.RootVolume.Type == "" {
		c.RootVolume.Type = main.WorkerRootVolumeType
	}

	if c.RootVolume.IOPS == 0 && c.RootVolume.Type == "io1" {
		c.RootVolume.IOPS = main.WorkerRootVolumeIOPS
	}

	if c.SpotFleet.RootVolumeType == "" {
		c.SpotFleet.RootVolumeType = c.RootVolume.Type
	}

	if c.RootVolume.Size == 0 {
		c.RootVolume.Size = main.WorkerRootVolumeSize
	}

	if c.Tenancy == "" {
		c.Tenancy = main.WorkerTenancy
	}

	if c.InstanceType == "" {
		c.InstanceType = main.WorkerInstanceType
	}

	if c.CreateTimeout == "" {
		c.CreateTimeout = main.WorkerCreateTimeout
	}

	return c
}
