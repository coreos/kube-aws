package config

import (
	"fmt"

	cfg "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
)

func (c DeploymentSettings) ValidateInputs() error {
	// By design, kube-aws doesn't allow customizing the following settings among node pools.
	//
	// Every node pool imports subnets from the main stack and therefore there's no need for setting:
	// * VPCID
	// * InternetGatewayID
	// * RouteTableID
	// * VPCCIDR
	// * InstanceCIDR
	// * MapPublicIPs
	// * ElasticFileSystemID
	if c.VPCID != "" {
		return fmt.Errorf("although you can't customize `vpcId` per node pool but you did specify \"%s\" in your cluster.yaml", c.VPCID)
	}
	if c.InternetGatewayID != "" {
		return fmt.Errorf("although you can't customize `internetGatewayId` per node pool but you did specify \"%s\" in your cluster.yaml", c.InternetGatewayID)
	}
	if c.RouteTableID != "" {
		return fmt.Errorf("although you can't customize `routeTableId` per node pool but you did specify \"%s\" in your cluster.yaml", c.RouteTableID)
	}
	if c.VPCCIDR != "" {
		return fmt.Errorf("although you can't customize `vpcCIDR` per node pool but you did specify \"%s\" in your cluster.yaml", c.VPCCIDR)
	}
	if c.InstanceCIDR != "" {
		return fmt.Errorf("although you can't customize `instanceCIDR` per node pool but you did specify \"%s\" in your cluster.yaml", c.InstanceCIDR)
	}
	if c.MapPublicIPs {
		return fmt.Errorf("although you can't customize `mapPublicIPs` per node pool but you did specify %v in your cluster.yaml", c.MapPublicIPs)
	}
	if c.ElasticFileSystemID != "" {
		return fmt.Errorf("although you can't customize `elasticFileSystemId` per node pool but you did specify \"%s\" in your cluster.yaml", c.ElasticFileSystemID)
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

	if err := c.Experimental.Valid(); err != nil {
		return err
	}

	return nil
}

func (s DeploymentSettings) Valid() error {
	if err := s.Experimental.Valid(); err != nil {
		return err
	}
	return nil
}

// TODO make this less smelly by e.g. moving this to core/nodepool/config
func (c DeploymentSettings) WithDefaultsFrom(main cfg.DeploymentSettings) DeploymentSettings {
	c.ClusterName = main.ClusterName

	if c.KeyName == "" {
		c.KeyName = main.KeyName
	}

	// No defaulting for AvailabilityZone: It must be set explicitly for high availability

	if c.ReleaseChannel == "" {
		c.ReleaseChannel = main.ReleaseChannel
	}

	if c.AmiId == "" {
		c.AmiId = main.AmiId
	}

	if c.K8sVer == "" {
		c.K8sVer = main.K8sVer
	}

	// Use main images if not defined in nodepool configuration
	c.HyperkubeImage.MergeIfEmpty(main.HyperkubeImage)
	c.HyperkubeImage.Tag = c.K8sVer
	c.AWSCliImage.MergeIfEmpty(main.AWSCliImage)
	c.CalicoCtlImage.MergeIfEmpty(main.CalicoCtlImage)
	c.PauseImage.MergeIfEmpty(main.PauseImage)
	c.FlannelImage.MergeIfEmpty(main.FlannelImage)

	// Inherit main TLS bootstrap config
	c.Experimental.TLSBootstrap = main.Experimental.TLSBootstrap

	if len(c.SSHAuthorizedKeys) == 0 {
		c.SSHAuthorizedKeys = main.SSHAuthorizedKeys
	}

	// And assuming that no one wants to differentiate these settings among control plane and node pools, we forbid customization of:
	c.ManageCertificates = main.ManageCertificates
	// And believing it is impossible to mix different values, we also forbid customization of:
	// * Region
	// * ContainerRuntime
	// * KMSKeyARN
	c.Region = main.Region
	c.ContainerRuntime = main.ContainerRuntime
	c.KMSKeyARN = main.KMSKeyARN

	return c
}
