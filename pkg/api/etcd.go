package api

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	MaxQuotaBackendBytes     int = 8 * 1024 * 1024 * 1024
	DefaultQuotaBackendBytes int = 2 * 1024 * 1024 * 1024
)

type Etcd struct {
	Cluster            EtcdCluster          `yaml:",inline"`
	CustomFiles        []CustomFile         `yaml:"customFiles,omitempty"`
	CustomSystemdUnits []CustomSystemdUnit  `yaml:"customSystemdUnits,omitempty"`
	DataVolume         DataVolume           `yaml:"dataVolume,omitempty"`
	DisasterRecovery   EtcdDisasterRecovery `yaml:"disasterRecovery,omitempty"`
	VolumeMounts       []NodeVolumeMount    `yaml:"volumeMounts,omitempty"`
	EC2Instance        `yaml:",inline"`
	UserSuppliedArgs   UserSuppliedArgs `yaml:"userSuppliedArgs,omitempty"`
	IAMConfig          IAMConfig        `yaml:"iam,omitempty"`
	Nodes              []EtcdNode       `yaml:"nodes,omitempty"`
	SecurityGroupIds   []string         `yaml:"securityGroupIds"`
	Snapshot           EtcdSnapshot     `yaml:"snapshot,omitempty"`
	Subnets            Subnets          `yaml:"subnets,omitempty"`
	StackExists        bool
	UnknownKeys        `yaml:",inline"`
}

type EtcdVersion string

type EtcdDisasterRecovery struct {
	Automated bool `yaml:"automated,omitempty"`
}

type UserSuppliedArgs struct {
	QuotaBackendBytes       int `yaml:"quotaBackendBytes,omitempty"`
	AutoCompactionRetention int `yaml:"autoCompactionRetention,omitempty"`
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
		StackExists: false,
		UserSuppliedArgs: UserSuppliedArgs{
			QuotaBackendBytes: DefaultQuotaBackendBytes,
		},
	}
}

func (e Etcd) LogicalName() string {
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

func (e Etcd) SecurityGroupRefs() []string {
	refs := []string{}

	for _, id := range e.SecurityGroupIds {
		refs = append(refs, fmt.Sprintf(`"%s"`, id))
	}

	refs = append(
		refs,
		`{"Fn::ImportValue" : {"Fn::Sub" : "${NetworkStackName}-EtcdSecurityGroup"}}`,
	)

	return refs
}

func (e Etcd) SystemdUnitName() string {
	if e.Version().Is3() {
		return "etcd-member.service"
	}
	return "etcd2.service"
}

func ValidateQuotaBackendBytes(bytes int) error {
	if bytes > MaxQuotaBackendBytes {
		return fmt.Errorf("quotaBackendBytes: %v is higher than the maximum allowed value 8,589,934,592", bytes)
	}
	return nil
}

func (e Etcd) Validate() error {
	if err := ValidateVolumeMounts(e.VolumeMounts); err != nil {
		return err
	}

	if err := ValidateQuotaBackendBytes(e.UserSuppliedArgs.QuotaBackendBytes); err != nil {
		return err
	}

	return nil
}

func (e Etcd) FormatOpts() string {
	opts := []string{}
	if e.UserSuppliedArgs.QuotaBackendBytes != 0 {
		quotaFlag := []string{"--quota-backend-bytes", strconv.Itoa(e.UserSuppliedArgs.QuotaBackendBytes)}
		opts = append(opts, strings.Join(quotaFlag, "="))
	}

	if e.UserSuppliedArgs.AutoCompactionRetention != 0 {
		compactFlag := []string{"--auto-compaction-retention", strconv.Itoa(e.UserSuppliedArgs.AutoCompactionRetention)}
		opts = append(opts, strings.Join(compactFlag, "="))
	}
	return strings.Join(opts, " ")
}

// Version returns the version of etcd (e.g. `3.2.1`) to be used for this etcd cluster
func (e Etcd) Version() EtcdVersion {
	if e.Cluster.Version != "" {
		return e.Cluster.Version
	}
	return "3.2.13"
}

func (v EtcdVersion) Is3() bool {
	return strings.HasPrefix(string(v), "3")
}

func (v EtcdVersion) String() string {
	return string(v)
}
