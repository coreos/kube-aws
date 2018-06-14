package derived

import (
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/model"
)

type EtcdNode interface {
	AdvertisedFQDNRef() (string, error)
	DependencyExists() bool
	DependencyRef() (string, error)
	EBSLogicalName() string
	EBSRef() string
	EIPAllocationIDRef() (string, error)
	EIPLogicalName() (string, error)
	EIPManaged() bool
	EIPRef() (string, error)
	// The name of the etcd member runs on this etcd node
	Name() string
	NetworkInterfaceIDRef() string
	NetworkInterfaceLogicalName() string
	NetworkInterfaceManaged() bool
	NetworkInterfacePrivateIPRef() string
	NetworkInterfacePrivateIPLogicalName() string
	ImportedAdvertisedFQDNRef() (string, error)
	LaunchConfigurationLogicalName() string
	LogicalName() string
	RecordSetManaged() bool
	RecordSetLogicalName() string
	SubnetRef() string
}

type etcdNodeImpl struct {
	cluster    EtcdCluster
	index      int
	config     model.EtcdNode
	subnet     model.Subnet
	natGateway model.NATGateway
}

func NewEtcdNodeDependsOnManagedNGW(cluster EtcdCluster, index int, nodeConfig model.EtcdNode, s model.Subnet, ngw model.NATGateway) EtcdNode {
	return etcdNodeImpl{
		cluster:    cluster,
		index:      index,
		config:     nodeConfig,
		subnet:     s,
		natGateway: ngw,
	}
}

func NewEtcdNode(cluster EtcdCluster, index int, nodeConfig model.EtcdNode, s model.Subnet) EtcdNode {
	return etcdNodeImpl{
		cluster: cluster,
		index:   index,
		config:  nodeConfig,
		subnet:  s,
	}
}

func (i etcdNodeImpl) Name() string {
	if i.config.Name != "" {
		return i.config.Name
	}
	return fmt.Sprintf("etcd%d", i.index)
}

func (i etcdNodeImpl) region() model.Region {
	return i.cluster.Region()
}

func (i etcdNodeImpl) customPrivateDNSName() string {
	if i.config.FQDN != "" {
		return i.config.FQDN
	}
	return fmt.Sprintf("%s.%s", i.Name(), i.cluster.InternalDomainName)
}

func (i etcdNodeImpl) privateDNSNameRef() string {
	if i.cluster.EC2InternalDomainUsed() {
		return i.defaultPrivateDNSNameRefFromIPRef(i.NetworkInterfacePrivateIPRef())
	}
	return fmt.Sprintf(`"%s"`, i.customPrivateDNSName())
}

func (i etcdNodeImpl) importedPrivateDNSNameRef() string {
	if i.cluster.EC2InternalDomainUsed() {
		return i.defaultPrivateDNSNameRefFromIPRef(fmt.Sprintf(`{ "Fn::ImportValue": {"Fn::Sub" : "${ControlPlaneStackName}-%s"} }`, i.NetworkInterfacePrivateIPLogicalName()))
	}
	return fmt.Sprintf(`"%s"`, i.customPrivateDNSName())
}

func (i etcdNodeImpl) defaultPrivateDNSNameRefFromIPRef(ipRef string) string {
	hostnameRef := fmt.Sprintf(`
	        { "Fn::Join" : [ "-",
	          [
                    "ip",
                    { "Fn::Join" : [ "-",
                      { "Fn::Split" : [ ".", %s ] }
                    ] }
                  ]
                ]}`, ipRef)
	return fmt.Sprintf(`{ "Fn::Join" : [ ".", [
                %s,
                "%s"
                ]]}`, hostnameRef, i.region().PrivateDomainName())
}

func (i etcdNodeImpl) defaultPublicDNSNameRef() (string, error) {
	eipRef, err := i.EIPRef()
	if err != nil {
		return "", fmt.Errorf("failed to determine an ec2 default public dns name: %v", err)
	}
	return i.defaultPublicDNSNameRefFromIPRef(eipRef), nil
}

func (i etcdNodeImpl) importedDefaultPublicDNSNameRef() (string, error) {
	eipLogicalName, err := i.EIPLogicalName()
	if err != nil {
		return "", fmt.Errorf("failed to determine an ec2 default public dns name: %v", err)
	}
	eipRef := fmt.Sprintf(`{ "Fn::ImportValue": {"Fn::Sub" : "${ControlPlaneStackName}-%s"} }`, eipLogicalName)
	return i.defaultPublicDNSNameRefFromIPRef(eipRef), nil
}

func (i etcdNodeImpl) defaultPublicDNSNameRefFromIPRef(ipRef string) string {
	return fmt.Sprintf(`{ "Fn::Join" : [ ".", [
                { "Fn::Join" : [ "-", [
                "ec2",
                { "Fn::Join" : [ "-", { "Fn::Split" : [ ".", %s ] } ] }
                ]]},
                "%s"
                ]]}`, ipRef, i.region().PublicComputeDomainName())
}

func (i etcdNodeImpl) AdvertisedFQDNRef() (string, error) {
	if i.cluster.NodeShouldHaveSecondaryENI() {
		return i.privateDNSNameRef(), nil
	}
	return i.defaultPublicDNSNameRef()
}

func (i etcdNodeImpl) ImportedAdvertisedFQDNRef() (string, error) {
	if i.cluster.NodeShouldHaveSecondaryENI() {
		return i.importedPrivateDNSNameRef(), nil
	}
	return i.importedDefaultPublicDNSNameRef()
}

func (i etcdNodeImpl) SubnetRef() string {
	return i.subnet.Ref()
}

func (i etcdNodeImpl) SubnetAvailabilityZone() string {
	return i.subnet.AvailabilityZone
}

func (i etcdNodeImpl) DependencyExists() bool {
	return i.subnet.Private && i.subnet.ManageRouteToNATGateway()
}

func (i etcdNodeImpl) DependencyRef() (string, error) {
	// We have to wait until the route to the NAT gateway if it doesn't exist yet(hence ManageRoute=true) or the etcd node fails due to inability to connect internet
	if i.DependencyExists() {
		name := i.subnet.NATGatewayRouteLogicalName()
		return fmt.Sprintf(`"%s"`, name), nil
	}
	return "", nil
}

func (i etcdNodeImpl) EBSLogicalName() string {
	return fmt.Sprintf("Etcd%dEBS", i.index)
}

func (i etcdNodeImpl) EBSRef() string {
	return fmt.Sprintf(`{ "Ref" : "%s" }`, i.EBSLogicalName())
}

func (i etcdNodeImpl) EIPAllocationIDRef() (string, error) {
	eipLogicalName, err := i.EIPLogicalName()
	if err != nil {
		return "", fmt.Errorf("failed to derive the ref to the allocation id of an EIP: %v", err)
	}
	return fmt.Sprintf(`{ "Fn::GetAtt" : [ "%s", "AllocationId" ] }`, eipLogicalName), nil
}

func (i etcdNodeImpl) EIPLogicalName() (string, error) {
	if !i.EIPManaged() {
		return "", fmt.Errorf("[bug] EIPLogicalName invoked when EIP is not managed. Etcd node name: %s", i.Name())
	}
	return fmt.Sprintf("Etcd%dEIP", i.index), nil
}

func (i etcdNodeImpl) EIPManaged() bool {
	return i.cluster.NodeShouldHaveEIP()
}

func (i etcdNodeImpl) EIPRef() (string, error) {
	eipLogicalName, err := i.EIPLogicalName()
	if err != nil {
		return "", fmt.Errorf("failed to derive the ref to an EIP: %v", err)
	}
	return fmt.Sprintf(`{ "Ref" : "%s" }`, eipLogicalName), nil
}

func (i etcdNodeImpl) NetworkInterfaceIDRef() string {
	return fmt.Sprintf(`{ "Ref" : "%s" }`, i.NetworkInterfaceLogicalName())
}

func (i etcdNodeImpl) NetworkInterfaceLogicalName() string {
	return fmt.Sprintf("Etcd%dENI", i.index)
}

func (i etcdNodeImpl) NetworkInterfaceManaged() bool {
	return i.cluster.NodeShouldHaveSecondaryENI()
}

func (i etcdNodeImpl) NetworkInterfacePrivateIPRef() string {
	return fmt.Sprintf(`{ "Fn::GetAtt" : [ "%s", "PrimaryPrivateIpAddress" ] }`, i.NetworkInterfaceLogicalName())
}

// NetworkInterfacePrivateIPLogicalName returns the logical name of the launch configuration specific to this etcd node
func (i etcdNodeImpl) NetworkInterfacePrivateIPLogicalName() string {
	return fmt.Sprintf("%sPrivateIP", i.LogicalName())
}

// LaunchConfigurationLogicalName returns the logical name of the launch configuration specific to this etcd node
func (i etcdNodeImpl) LaunchConfigurationLogicalName() string {
	return fmt.Sprintf("%sLC", i.LogicalName())
}

func (i etcdNodeImpl) LogicalName() string {
	return fmt.Sprintf("Etcd%d", i.index)
}

func (i etcdNodeImpl) RecordSetManaged() bool {
	return i.cluster.NodeShouldHaveSecondaryENI() && i.cluster.RecordSetsManaged()
}

func (i etcdNodeImpl) RecordSetLogicalName() string {
	return fmt.Sprintf("Etcd%dInternalRecordSet", i.index)
}
