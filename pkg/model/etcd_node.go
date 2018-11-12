package model

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
)

type EtcdNode struct {
	cluster    EtcdCluster
	index      int
	config     api.EtcdNode
	subnet     api.Subnet
	natGateway api.NATGateway
}

func NewEtcdNodeDependsOnManagedNGW(cluster EtcdCluster, index int, nodeConfig api.EtcdNode, s api.Subnet, ngw api.NATGateway) EtcdNode {
	return EtcdNode{
		cluster:    cluster,
		index:      index,
		config:     nodeConfig,
		subnet:     s,
		natGateway: ngw,
	}
}

func NewEtcdNode(cluster EtcdCluster, index int, nodeConfig api.EtcdNode, s api.Subnet) EtcdNode {
	return EtcdNode{
		cluster: cluster,
		index:   index,
		config:  nodeConfig,
		subnet:  s,
	}
}

func (i EtcdNode) Name() string {
	if i.config.Name != "" {
		return i.config.Name
	}
	return fmt.Sprintf("etcd%d", i.index)
}

func (i EtcdNode) region() api.Region {
	return i.cluster.Region()
}

func (i EtcdNode) customPrivateDNSName() string {
	if i.config.FQDN != "" {
		return i.config.FQDN
	}
	return fmt.Sprintf("%s.%s", i.Name(), i.cluster.InternalDomainName)
}

func (i EtcdNode) privateDNSNameRef() string {
	if i.cluster.EC2InternalDomainUsed() {
		return i.defaultPrivateDNSNameRefFromIPRef(i.NetworkInterfacePrivateIPRef())
	}
	return fmt.Sprintf(`"%s"`, i.customPrivateDNSName())
}

func (i EtcdNode) importedPrivateDNSNameRef() string {
	if i.cluster.EC2InternalDomainUsed() {
		return i.defaultPrivateDNSNameRefFromIPRef(fmt.Sprintf(`{ "Fn::ImportValue": {"Fn::Sub" : "${EtcdStackName}-%s"} }`, i.NetworkInterfacePrivateIPLogicalName()))
	}
	return fmt.Sprintf(`"%s"`, i.customPrivateDNSName())
}

func (i EtcdNode) defaultPrivateDNSNameRefFromIPRef(ipRef string) string {
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

func (i EtcdNode) defaultPublicDNSNameRef() (string, error) {
	eipRef, err := i.EIPRef()
	if err != nil {
		return "", fmt.Errorf("failed to determine an ec2 default public dns name: %v", err)
	}
	return i.defaultPublicDNSNameRefFromIPRef(eipRef), nil
}

func (i EtcdNode) importedDefaultPublicDNSNameRef() (string, error) {
	eipLogicalName, err := i.EIPLogicalName()
	if err != nil {
		return "", fmt.Errorf("failed to determine an ec2 default public dns name: %v", err)
	}
	eipRef := fmt.Sprintf(`{ "Fn::ImportValue": {"Fn::Sub" : "${EtcdStackName}-%s"} }`, eipLogicalName)
	return i.defaultPublicDNSNameRefFromIPRef(eipRef), nil
}

func (i EtcdNode) defaultPublicDNSNameRefFromIPRef(ipRef string) string {
	return fmt.Sprintf(`{ "Fn::Join" : [ ".", [
                { "Fn::Join" : [ "-", [
                "ec2",
                { "Fn::Join" : [ "-", { "Fn::Split" : [ ".", %s ] } ] }
                ]]},
                "%s"
                ]]}`, ipRef, i.region().PublicComputeDomainName())
}

func (i EtcdNode) AdvertisedFQDNRef() (string, error) {
	if i.cluster.NodeShouldHaveSecondaryENI() {
		return i.privateDNSNameRef(), nil
	}
	return i.defaultPublicDNSNameRef()
}

func (i EtcdNode) ImportedAdvertisedFQDNRef() (string, error) {
	if i.cluster.NodeShouldHaveSecondaryENI() {
		return i.importedPrivateDNSNameRef(), nil
	}
	return i.importedDefaultPublicDNSNameRef()
}

func (i EtcdNode) SubnetRef() string {
	return i.subnet.Ref()
}

func (i EtcdNode) SubnetAvailabilityZone() string {
	return i.subnet.AvailabilityZone
}

func (i EtcdNode) DependencyExists() bool {
	return i.subnet.Private && i.subnet.ManageRouteToNATGateway()
}

func (i EtcdNode) DependencyRef() (string, error) {
	// We have to wait until the route to the NAT gateway if it doesn't exist yet(hence ManageRoute=true) or the etcd node fails due to inability to connect internet
	if i.DependencyExists() {
		name := i.subnet.NATGatewayRouteLogicalName()
		return fmt.Sprintf(`"%s"`, name), nil
	}
	return "", nil
}

func (i EtcdNode) EBSLogicalName() string {
	return fmt.Sprintf("Etcd%dEBS", i.index)
}

func (i EtcdNode) EBSRef() string {
	return fmt.Sprintf(`{ "Ref" : "%s" }`, i.EBSLogicalName())
}

func (i EtcdNode) EIPAllocationIDRef() (string, error) {
	eipLogicalName, err := i.EIPLogicalName()
	if err != nil {
		return "", fmt.Errorf("failed to derive the ref to the allocation id of an EIP: %v", err)
	}
	return fmt.Sprintf(`{ "Fn::GetAtt" : [ "%s", "AllocationId" ] }`, eipLogicalName), nil
}

func (i EtcdNode) EIPLogicalName() (string, error) {
	if !i.EIPManaged() {
		return "", fmt.Errorf("[bug] EIPLogicalName invoked when EIP is not managed. Etcd node name: %s", i.Name())
	}
	return fmt.Sprintf("Etcd%dEIP", i.index), nil
}

func (i EtcdNode) EIPManaged() bool {
	return i.cluster.NodeShouldHaveEIP()
}

func (i EtcdNode) EIPRef() (string, error) {
	eipLogicalName, err := i.EIPLogicalName()
	if err != nil {
		return "", fmt.Errorf("failed to derive the ref to an EIP: %v", err)
	}
	return fmt.Sprintf(`{ "Ref" : "%s" }`, eipLogicalName), nil
}

func (i EtcdNode) NetworkInterfaceIDRef() string {
	return fmt.Sprintf(`{ "Ref" : "%s" }`, i.NetworkInterfaceLogicalName())
}

func (i EtcdNode) NetworkInterfaceLogicalName() string {
	return fmt.Sprintf("Etcd%dENI", i.index)
}

func (i EtcdNode) NetworkInterfaceManaged() bool {
	return i.cluster.NodeShouldHaveSecondaryENI()
}

func (i EtcdNode) NetworkInterfacePrivateIPRef() string {
	return fmt.Sprintf(`{ "Fn::GetAtt" : [ "%s", "PrimaryPrivateIpAddress" ] }`, i.NetworkInterfaceLogicalName())
}

// NetworkInterfacePrivateIPLogicalName returns the logical name of the launch configuration specific to this etcd node
func (i EtcdNode) NetworkInterfacePrivateIPLogicalName() string {
	return fmt.Sprintf("%sPrivateIP", i.LogicalName())
}

// LaunchConfigurationLogicalName returns the logical name of the launch configuration specific to this etcd node
func (i EtcdNode) LaunchConfigurationLogicalName() string {
	return fmt.Sprintf("%sLC", i.LogicalName())
}

func (i EtcdNode) LogicalName() string {
	return fmt.Sprintf("Etcd%d", i.index)
}

func (i EtcdNode) RecordSetManaged() bool {
	return i.cluster.NodeShouldHaveSecondaryENI() && i.cluster.RecordSetsManaged()
}

func (i EtcdNode) RecordSetLogicalName() string {
	return fmt.Sprintf("Etcd%dInternalRecordSet", i.index)
}
