package model

import (
	"errors"
	"strings"
)

type Etcd struct {
	Cluster          EtcdCluster          `yaml:",inline"`
	DataVolume       DataVolume           `yaml:"dataVolume,omitempty"`
	DisasterRecovery EtcdDisasterRecovery `yaml:"disasterRecovery,omitempty"`
	Snapshot         EtcdSnapshot         `yaml:"snapshot,omitempty"`
	EC2Instance      `yaml:",inline"`
	Nodes            []EtcdNode `yaml:"nodes,omitempty"`
	Subnets          []Subnet   `yaml:"subnets,omitempty"`
	UnknownKeys      `yaml:",inline"`
}

type EtcdVersion string

type EtcdDisasterRecovery struct {
	Automated bool `yaml:"automated,omitempty"`
}

// Supported returns true when the disaster recovery feature provided by etcdadm can be enabled on the specified version of etcd
func (r EtcdDisasterRecovery) SupportsEtcdVersion(etcdVersion EtcdVersion) bool {
	return etcdVersion.Is3()
}

func (r EtcdDisasterRecovery) IsAutomatedForEtcdVersion(etcdVersion EtcdVersion) bool {
	return etcdVersion.Is3() && r.Automated
}

type EtcdSnapshot struct {
	Automated bool `yaml:"automated,omitempty"`
}

func (s EtcdSnapshot) IsAutomatedForEtcdVersion(etcdVersion EtcdVersion) bool {
	return etcdVersion.Is3() && s.Automated
}

func NewDefaultEtcd() Etcd {
	return Etcd{
		EC2Instance: EC2Instance{
			Count:        1,
			InstanceType: "t2.medium",
			RootVolume: RootVolume{
				Size: 30,
				Type: "gp2",
				IOPS: 0,
			},
			Tenancy: "default",
		},
		DataVolume: DataVolume{
			Size: 30,
			Type: "gp2",
			IOPS: 0,
		},
	}
}

func (i Etcd) LogicalName() string {
	return "Etcd"
}

// NameTagKey returns the key of the tag used to identify the name of the etcd member of an EBS volume
func (e Etcd) NameTagKey() string {
	return "kube-aws:etcd:name"
}

// AdvertisedFQDNTagKey returns the key of the tag used to identify the advertised hostname of the etcd member of an EBS volume
func (e Etcd) AdvertisedFQDNTagKey() string {
	return "kube-aws:etcd:advertised-hostname"
}

// EIPAllocationIDTagKey returns the key of the tag used to identify the EIP for the etcd member of an EBS volume
func (e Etcd) EIPAllocationIDTagKey() string {
	return "kube-aws:etcd:eip-allocation-id"
}

// NetworkInterfaceIDTagKey returns the key of the tag used to identify the ENI for the etcd member of an EBS volume
func (e Etcd) NetworkInterfaceIDTagKey() string {
	return "kube-aws:etcd:network-interface-id"
}

// NetworkInterfaceDeviceIndex represents that the network interface at index 1 is reserved by kube-aws for etcd peer communication
// Please submit a feature request if this is inconvenient for you
func (e Etcd) NetworkInterfaceDeviceIndex() int {
	return 1
}

func (e Etcd) NodeShouldHaveEIP() bool {
	return e.Cluster.NodeShouldHaveEIP()
}

func (e Etcd) NodeShouldHaveSecondaryENI() bool {
	return e.Cluster.NodeShouldHaveSecondaryENI()
}

func (e Etcd) HostedZoneManaged() bool {
	return e.Cluster.hostedZoneManaged()
}

func (e Etcd) HostedZoneRef() (string, error) {
	return e.Cluster.HostedZone.RefOrError(func() (string, error) {
		return e.HostedZoneLogicalName()
	})
}

func (e Etcd) InternalDomainName() (string, error) {
	return e.Cluster.InternalDomainName, nil
}

func (e Etcd) HostedZoneLogicalName() (string, error) {
	if !e.Cluster.hostedZoneManaged() {
		return "", errors.New("[bug] HostedZoneLogicalName called for an etcd cluster without a managed hosted zone")
	}
	return "EtcdHostedZone", nil
}

func (e Etcd) KMSKeyARN() string {
	return e.Cluster.KMSKeyARN
}

func (e Etcd) SystemdUnitName() string {
	if e.Version().Is3() {
		return "etcd-member.service"
	}
	return "etcd2.service"
}

// Version returns the version of etcd (e.g. `2`, `3`, `3.1.3`) to be used for this etcd cluster
func (e Etcd) Version() EtcdVersion {
	if e.Cluster.Version != "" {
		return e.Cluster.Version
	}
	if e.Cluster.Version == "3" {
		return "3.1.3"
	}
	return "2"
}

func (v EtcdVersion) Is3() bool {
	return strings.HasPrefix(string(v), "3")
}

func (v EtcdVersion) String() string {
	return string(v)
}
