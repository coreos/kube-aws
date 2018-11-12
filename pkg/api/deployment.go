package api

import (
	"errors"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/kubernetes-incubator/kube-aws/netutil"
	"net"
	"strings"
)

func (s DeploymentSettings) ValidateNodePool(name string) error {
	if err := s.Experimental.Validate(name); err != nil {
		return err
	}
	return nil
}

// TODO make this less smelly by e.g. moving this to core/nodepool/config
func (c DeploymentSettings) WithDefaultsFrom(main DeploymentSettings) DeploymentSettings {
	c.ClusterName = main.ClusterName

	if c.KeyName == "" {
		c.KeyName = main.KeyName
	}

	// No defaulting for AvailabilityZone: It must be set explicitly for high availability

	// If there was a specific release channel specified for this node pool,
	// the user would want to use the latest AMI for the channel, not the latest AMI for the default release channel
	// specified in the top level of cluster.yaml
	if c.ReleaseChannel == "" {
		c.ReleaseChannel = main.ReleaseChannel

		if c.AmiId == "" {
			c.AmiId = main.AmiId
		}
	}

	if c.K8sVer == "" {
		c.K8sVer = main.K8sVer
	}

	// Use main images if not defined in nodepool configuration
	c.HyperkubeImage.MergeIfEmpty(main.HyperkubeImage)
	c.HyperkubeImage.Tag = c.K8sVer
	c.AWSCliImage.MergeIfEmpty(main.AWSCliImage)
	c.PauseImage.MergeIfEmpty(main.PauseImage)
	c.JournaldCloudWatchLogsImage.MergeIfEmpty(main.JournaldCloudWatchLogsImage)

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
	// * ElasticFileSystemID
	c.Region = main.Region
	c.ContainerRuntime = main.ContainerRuntime
	c.KMSKeyARN = main.KMSKeyARN

	// TODO Allow providing one or more elasticFileSystemId's to be mounted both per-node-pool/cluster-wide
	// TODO Allow providing elasticFileSystemId to a node pool in managed subnets.
	// Currently, per-node-pool elasticFileSystemId requires existing subnets configured by users to have appropriate MountTargets associated
	if c.ElasticFileSystemID == "" {
		c.ElasticFileSystemID = main.ElasticFileSystemID
	}

	// Inherit main CloudWatchLogging config
	c.CloudWatchLogging.MergeIfEmpty(main.CloudWatchLogging)

	// Inherit main AmazonSsmAgent config
	c.AmazonSsmAgent = main.AmazonSsmAgent

	//Inherit main KubeDns config
	c.KubeDns.MergeIfEmpty(main.KubeDns)

	//Inherit main Kubernetes config (e.g. for Kubernetes.Networking.SelfHosting etc.)
	c.Kubernetes = main.Kubernetes

	return c
}

type DeploymentValidationResult struct {
	vpcNet *net.IPNet
}

func (c DeploymentSettings) Validate() (*DeploymentValidationResult, error) {
	releaseChannelSupported := supportedReleaseChannels[c.ReleaseChannel]
	if !releaseChannelSupported {
		return nil, fmt.Errorf("releaseChannel %s is not supported", c.ReleaseChannel)
	}

	if c.KeyName == "" && len(c.SSHAuthorizedKeys) == 0 {
		return nil, errors.New("Either keyName or sshAuthorizedKeys must be set")
	}
	if c.ClusterName == "" {
		return nil, errors.New("clusterName must be set")
	}
	if c.S3URI == "" {
		return nil, errors.New("s3URI must be set")
	}
	if c.KMSKeyARN == "" && c.AssetsEncryptionEnabled() {
		return nil, errors.New("kmsKeyArn must be set")
	}

	if c.Region.IsEmpty() {
		return nil, errors.New("region must be set")
	}

	_, err := semver.NewVersion(c.K8sVer)
	if err != nil {
		return nil, errors.New("kubernetesVersion must be a valid version")
	}

	if c.KMSKeyARN != "" && !c.Region.IsEmpty() && !strings.Contains(c.KMSKeyARN, c.Region.String()) {
		return nil, errors.New("kmsKeyArn must reference the same region as the one being deployed to")
	}

	_, vpcNet, err := net.ParseCIDR(c.VPCCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid vpcCIDR: %v", err)
	}

	if len(c.Subnets) == 0 {
		if c.AvailabilityZone == "" {
			return nil, fmt.Errorf("availabilityZone must be set")
		}
		_, instanceCIDR, err := net.ParseCIDR(c.InstanceCIDR)
		if err != nil {
			return nil, fmt.Errorf("invalid instanceCIDR: %v", err)
		}
		if !vpcNet.Contains(instanceCIDR.IP) {
			return nil, fmt.Errorf("vpcCIDR (%s) does not contain instanceCIDR (%s)",
				c.VPCCIDR,
				c.InstanceCIDR,
			)
		}
	} else {
		if c.InstanceCIDR != "" {
			return nil, fmt.Errorf("The top-level instanceCIDR(%s) must be empty when subnets are specified", c.InstanceCIDR)
		}
		if c.AvailabilityZone != "" {
			return nil, fmt.Errorf("The top-level availabilityZone(%s) must be empty when subnets are specified", c.AvailabilityZone)
		}

		var instanceCIDRs = make([]*net.IPNet, 0)

		allPrivate := true
		allPublic := true
		allExistingRouteTable := true

		for i, subnet := range c.Subnets {
			if subnet.Validate(); err != nil {
				return nil, fmt.Errorf("failed to validate subnet: %v", err)
			}

			allExistingRouteTable = allExistingRouteTable && !subnet.ManageRouteTable()
			allPrivate = allPrivate && subnet.Private
			allPublic = allPublic && subnet.Public()
			if subnet.HasIdentifier() {
				continue
			}

			if subnet.AvailabilityZone == "" {
				return nil, fmt.Errorf("availabilityZone must be set for subnet #%d", i)
			}
			_, instanceCIDR, err := net.ParseCIDR(subnet.InstanceCIDR)
			if err != nil {
				return nil, fmt.Errorf("invalid instanceCIDR for subnet #%d: %v", i, err)
			}
			instanceCIDRs = append(instanceCIDRs, instanceCIDR)
			if !vpcNet.Contains(instanceCIDR.IP) {
				return nil, fmt.Errorf("vpcCIDR (%s) does not contain instanceCIDR (%s) for subnet #%d",
					c.VPCCIDR,
					c.InstanceCIDR,
					i,
				)
			}

			if !c.VPC.HasIdentifier() && (subnet.RouteTable.HasIdentifier() || c.InternetGateway.HasIdentifier()) {
				return nil, errors.New("vpcId must be specified if subnets[].routeTable.id or internetGateway.id are specified")
			}

			if subnet.ManageSubnet() && subnet.Public() && c.VPC.HasIdentifier() && subnet.ManageRouteTable() && !c.InternetGateway.HasIdentifier() {
				return nil, errors.New("internet gateway id can't be omitted when there're one or more managed public subnets in an existing VPC")
			}
		}

		// All the subnets are explicitly/implicitly(they're public by default) configured to be "public".
		// They're also configured to reuse existing route table(s).
		// However, the IGW, which won't be applied to anywhere, is specified
		if allPublic && allExistingRouteTable && c.InternetGateway.HasIdentifier() {
			return nil, errors.New("internet gateway id can't be specified when all the public subnets have existing route tables associated. kube-aws doesn't try to modify an exisinting route table to include a route to the internet gateway")
		}

		// All the subnets are explicitly configured to be "private" but the IGW, which won't be applied anywhere, is specified
		if allPrivate && c.InternetGateway.HasIdentifier() {
			return nil, errors.New("internet gateway id can't be specified when all the subnets are existing private subnets")
		}

		for i, a := range instanceCIDRs {
			for j := i + 1; j < len(instanceCIDRs); j++ {
				b := instanceCIDRs[j]
				if netutil.CidrOverlap(a, b) {
					return nil, fmt.Errorf("CIDR of subnet %d (%s) overlaps with CIDR of subnet %d (%s)", i, a, j, b)
				}
			}
		}
	}

	if err := c.Experimental.Validate("controller"); err != nil {
		return nil, err
	}

	for i, ngw := range c.NATGateways() {
		if err := ngw.Validate(); err != nil {
			return nil, fmt.Errorf("NGW %d is not valid: %v", i, err)
		}
	}

	return &DeploymentValidationResult{vpcNet: vpcNet}, nil
}

func (c DeploymentSettings) AssetsEncryptionEnabled() bool {
	return c.ManageCertificates && c.Region.SupportsKMS()
}

func (s DeploymentSettings) AllSubnets() Subnets {
	subnets := s.Subnets
	return subnets
}

func (c DeploymentSettings) FindSubnetMatching(condition Subnet) Subnet {
	for _, s := range c.Subnets {
		if s.Name == condition.Name {
			return s
		}
	}
	out := ""
	for _, s := range c.Subnets {
		out = fmt.Sprintf("%s%+v ", out, s)
	}
	panic(fmt.Errorf("No subnet matching %v found in %s", condition, out))
}

func (c DeploymentSettings) PrivateSubnets() Subnets {
	result := []Subnet{}
	for _, s := range c.Subnets {
		if s.Private {
			result = append(result, s)
		}
	}
	return result
}

func (c DeploymentSettings) PublicSubnets() Subnets {
	result := []Subnet{}
	for _, s := range c.Subnets {
		if !s.Private {
			result = append(result, s)
		}
	}
	return result
}

func (c DeploymentSettings) FindNATGatewayForPrivateSubnet(s Subnet) (*NATGateway, error) {
	for _, ngw := range c.NATGateways() {
		if ngw.IsConnectedToPrivateSubnet(s) {
			return &ngw, nil
		}
	}
	return nil, fmt.Errorf("No NATGateway found for the subnet %v", s)
}

func (c DeploymentSettings) NATGateways() []NATGateway {
	ngws := []NATGateway{}
	for _, privateSubnet := range c.PrivateSubnets() {
		var publicSubnet Subnet
		ngwConfig := privateSubnet.NATGateway
		if privateSubnet.ManageNATGateway() {
			publicSubnetFound := false
			for _, s := range c.PublicSubnets() {
				if s.AvailabilityZone == privateSubnet.AvailabilityZone {
					publicSubnet = s
					publicSubnetFound = true
					break
				}
			}
			if !publicSubnetFound {
				panic(fmt.Sprintf("No appropriate public subnet found for a non-preconfigured NAT gateway associated to private subnet %s", privateSubnet.LogicalName()))
			}
			ngw := NewManagedNATGateway(ngwConfig, privateSubnet, publicSubnet)
			ngws = append(ngws, ngw)
		} else if ngwConfig.HasIdentifier() {
			ngw := NewUnmanagedNATGateway(ngwConfig, privateSubnet)
			ngws = append(ngws, ngw)
		}
	}
	return ngws
}
