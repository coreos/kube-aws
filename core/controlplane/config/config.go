package config

//go:generate go run ../../../codegen/templates_gen.go CloudConfigController=cloud-config-controller CloudConfigWorker=cloud-config-worker CloudConfigEtcd=cloud-config-etcd DefaultClusterConfig=cluster.yaml KubeConfigTemplate=kubeconfig.tmpl StackTemplateTemplate=stack-template.json
//go:generate gofmt -w templates.go
//go:generate go run ../../../codegen/files_gen.go Etcdadm=../../../etcdadm/etcdadm
//go:generate gofmt -w files.go

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/coreos/go-semver/semver"
	"github.com/kubernetes-incubator/kube-aws/cfnresource"
	"github.com/kubernetes-incubator/kube-aws/coreos/amiregistry"
	"github.com/kubernetes-incubator/kube-aws/filereader/userdatatemplate"
	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/model/derived"
	"github.com/kubernetes-incubator/kube-aws/netutil"
	yaml "gopkg.in/yaml.v2"
)

const (
	k8sVer = "v1.5.5_coreos.0"

	credentialsDir = "credentials"
	userDataDir    = "userdata"
)

func NewDefaultCluster() *Cluster {
	experimental := Experimental{
		Admission: Admission{
			PodSecurityPolicy{
				Enabled: false,
			},
		},
		AuditLog: AuditLog{
			Enabled: false,
			MaxAge:  30,
			LogPath: "/dev/stdout",
		},
		Authentication: Authentication{
			Webhook{
				Enabled:  false,
				CacheTTL: "5m0s",
				Config:   "",
			},
		},
		AwsEnvironment: AwsEnvironment{
			Enabled: false,
		},
		AwsNodeLabels: AwsNodeLabels{
			Enabled: false,
		},
		ClusterAutoscalerSupport: ClusterAutoscalerSupport{
			Enabled: false,
		},
		TLSBootstrap: TLSBootstrap{
			Enabled: false,
		},
		EphemeralImageStorage: EphemeralImageStorage{
			Enabled:    false,
			Disk:       "xvdb",
			Filesystem: "xfs",
		},
		Kube2IamSupport: Kube2IamSupport{
			Enabled: false,
		},
		LoadBalancer: LoadBalancer{
			Enabled: false,
		},
		TargetGroup: TargetGroup{
			Enabled: false,
		},
		NodeDrainer: NodeDrainer{
			Enabled: false,
		},
		NodeLabels: NodeLabels{},
		Plugins: Plugins{
			Rbac: Rbac{
				Enabled: false,
			},
		},
		Taints: []Taint{},
	}

	return &Cluster{
		DeploymentSettings: DeploymentSettings{
			ClusterName:                 "kubernetes",
			VPCCIDR:                     "10.0.0.0/16",
			ReleaseChannel:              "stable",
			K8sVer:                      k8sVer,
			ContainerRuntime:            "docker",
			Subnets:                     []model.Subnet{},
			EIPAllocationIDs:            []string{},
			MapPublicIPs:                true,
			Experimental:                experimental,
			ManageCertificates:          true,
			HyperkubeImage:              model.Image{Repo: "quay.io/coreos/hyperkube", Tag: k8sVer, RktPullDocker: false},
			AWSCliImage:                 model.Image{Repo: "quay.io/coreos/awscli", Tag: "master", RktPullDocker: false},
			CalicoNodeImage:             model.Image{Repo: "quay.io/calico/node", Tag: "v1.0.2", RktPullDocker: false},
			CalicoCniImage:              model.Image{Repo: "quay.io/calico/cni", Tag: "v1.5.6", RktPullDocker: false},
			CalicoPolicyControllerImage: model.Image{Repo: "quay.io/calico/kube-policy-controller", Tag: "v0.5.4", RktPullDocker: false},
			ClusterAutoscalerImage:      model.Image{Repo: "gcr.io/google_containers/cluster-proportional-autoscaler-amd64", Tag: "1.0.0", RktPullDocker: false},
			KubeDnsImage:                model.Image{Repo: "gcr.io/google_containers/kubedns-amd64", Tag: "1.9", RktPullDocker: false},
			KubeDnsMasqImage:            model.Image{Repo: "gcr.io/google_containers/kube-dnsmasq-amd64", Tag: "1.4", RktPullDocker: false},
			KubeReschedulerImage:        model.Image{Repo: "gcr.io/google-containers/rescheduler", Tag: "v0.2.2", RktPullDocker: false},
			DnsMasqMetricsImage:         model.Image{Repo: "gcr.io/google_containers/dnsmasq-metrics-amd64", Tag: "1.0", RktPullDocker: false},
			ExecHealthzImage:            model.Image{Repo: "gcr.io/google_containers/exechealthz-amd64", Tag: "1.2", RktPullDocker: false},
			HeapsterImage:               model.Image{Repo: "gcr.io/google_containers/heapster", Tag: "v1.3.0", RktPullDocker: false},
			AddonResizerImage:           model.Image{Repo: "gcr.io/google_containers/addon-resizer", Tag: "1.6", RktPullDocker: false},
			KubeDashboardImage:          model.Image{Repo: "gcr.io/google_containers/kubernetes-dashboard-amd64", Tag: "v1.5.1", RktPullDocker: false},
			CalicoCtlImage:              model.Image{Repo: "calico/ctl", Tag: "v1.0.0", RktPullDocker: false},
			PauseImage:                  model.Image{Repo: "gcr.io/google_containers/pause-amd64", Tag: "3.0", RktPullDocker: false},
			FlannelImage:                model.Image{Repo: "quay.io/coreos/flannel", Tag: "v0.6.2", RktPullDocker: false},
		},
		KubeClusterSettings: KubeClusterSettings{
			DNSServiceIP: "10.3.0.10",
		},
		DefaultWorkerSettings: DefaultWorkerSettings{
			WorkerCount:            0,
			WorkerCreateTimeout:    "PT15M",
			WorkerInstanceType:     "t2.medium",
			WorkerRootVolumeType:   "gp2",
			WorkerRootVolumeIOPS:   0,
			WorkerRootVolumeSize:   30,
			WorkerSecurityGroupIds: []string{},
			WorkerTenancy:          "default",
		},
		ControllerSettings: ControllerSettings{
			Controller: model.NewDefaultController(),
		},
		EtcdSettings: EtcdSettings{
			Etcd: model.NewDefaultEtcd(),
		},
		FlannelSettings: FlannelSettings{
			PodCIDR: "10.2.0.0/16",
		},
		// for kube-apiserver
		ServiceCIDR: "10.3.0.0/24",
		// for base cloudformation stack
		TLSCADurationDays:   365 * 10,
		TLSCertDurationDays: 365,
		CreateRecordSet:     false,
		RecordSetTTL:        300,
		CustomSettings:      make(map[string]interface{}),
	}
}

func newDefaultClusterWithDeps(encSvc EncryptService) *Cluster {
	cluster := NewDefaultCluster()
	cluster.HyperkubeImage.Tag = cluster.K8sVer
	cluster.ProvidedEncryptService = encSvc
	return cluster
}

func ClusterFromFile(filename string) (*Cluster, error) {
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

// ClusterFromBytes Necessary for unit tests, which store configs as hardcoded strings
func ClusterFromBytes(data []byte) (*Cluster, error) {
	c := NewDefaultCluster()

	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("failed to parse cluster: %v", err)
	}

	c.HyperkubeImage.Tag = c.K8sVer

	if err := c.Load(); err != nil {
		return nil, err
	}

	return c, nil
}

func ConfigFromBytes(data []byte) (*Config, error) {
	c, err := ClusterFromBytes(data)
	if err != nil {
		return nil, err
	}
	cfg, err := c.Config()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Cluster) Load() error {
	// If the user specified no subnets, we assume that a single AZ configuration with the default instanceCIDR is demanded
	if len(c.Subnets) == 0 && c.InstanceCIDR == "" {
		c.InstanceCIDR = "10.0.0.0/24"
	}

	c.HostedZoneID = withHostedZoneIDPrefix(c.HostedZoneID)

	c.ConsumeDeprecatedKeys()

	if err := c.valid(); err != nil {
		return fmt.Errorf("invalid cluster: %v", err)
	}

	c.SetDefaults()

	if c.ExternalDNSName != "" {
		// TODO: Deprecate externalDNSName?

		if len(c.APIEndpointConfigs) != 0 {
			return errors.New("invalid cluster: you can only specify either externalDNSName or apiEndpoints, but not both")
		}

		subnetRefs := []model.SubnetReference{}
		for _, s := range c.Controller.LoadBalancer.Subnets {
			subnetRefs = append(subnetRefs, model.SubnetReference{Name: s.Name})
		}

		c.APIEndpointConfigs = model.NewDefaultAPIEndpoints(
			c.ExternalDNSName,
			subnetRefs,
			c.HostedZoneID,
			c.CreateRecordSet,
			c.RecordSetTTL,
			c.Controller.LoadBalancer.Private,
		)
	}

	return nil
}

func (c *Cluster) ConsumeDeprecatedKeys() {
	// TODO Remove deprecated keys in v0.9.7
	if c.DeprecatedControllerCount != nil {
		fmt.Println("WARN: controllerCount is deprecated and will be removed in v0.9.7. Please use controller.count instead")
		c.Controller.Count = *c.DeprecatedControllerCount
	}
	if c.DeprecatedControllerTenancy != nil {
		fmt.Println("WARN: controllerTenancy is deprecated and will be removed in v0.9.7. Please use controller.tenancy instead")
		c.Controller.Tenancy = *c.DeprecatedControllerTenancy
	}
	if c.DeprecatedControllerInstanceType != nil {
		fmt.Println("WARN: controllerInstanceType is deprecated and will be removed in v0.9.7. Please use controller.instanceType instead")
		c.Controller.InstanceType = *c.DeprecatedControllerInstanceType
	}
	if c.DeprecatedControllerCreateTimeout != nil {
		fmt.Println("WARN: controllerCreateTimeout is deprecated and will be removed in v0.9.7. Please use controller.createTimeout instead")
		c.Controller.CreateTimeout = *c.DeprecatedControllerCreateTimeout
	}
	if c.DeprecatedControllerRootVolumeIOPS != nil {
		fmt.Println("WARN: controllerRootVolumeIOPS is deprecated and will be removed in v0.9.7. Please use controller.rootVolume.iops instead")
		c.Controller.RootVolume.IOPS = *c.DeprecatedControllerRootVolumeIOPS
	}
	if c.DeprecatedControllerRootVolumeSize != nil {
		fmt.Println("WARN: controllerRootVolumeSize is deprecated and will be removed in v0.9.7. Please use controller.rootVolume.size instead")
		c.Controller.RootVolume.Size = *c.DeprecatedControllerRootVolumeSize
	}
	if c.DeprecatedControllerRootVolumeType != nil {
		fmt.Println("WARN: controllerRootVolumeType is deprecated and will be removed in v0.9.7. Please use controller.rootVolume.type instead")
		c.Controller.RootVolume.Type = *c.DeprecatedControllerRootVolumeType
	}

	if c.DeprecatedEtcdCount != nil {
		fmt.Println("WARN: etcdCount is deprecated and will be removed in v0.9.7. Please use etcd.count instead")
		c.Etcd.Count = *c.DeprecatedEtcdCount
	}
	if c.DeprecatedEtcdTenancy != nil {
		fmt.Println("WARN: etcdTenancy is deprecated and will be removed in v0.9.7. Please use etcd.tenancy instead")
		c.Etcd.Tenancy = *c.DeprecatedEtcdTenancy
	}
	if c.DeprecatedEtcdInstanceType != nil {
		fmt.Println("WARN: etcdInstanceType is deprecated and will be removed in v0.9.7. Please use etcd.instanceType instead")
		c.Etcd.InstanceType = *c.DeprecatedEtcdInstanceType
	}
	//if c.DeprecatedEtcdCreateTimeout != nil {
	//	c.Etcd.CreateTimeout = *c.DeprecatedEtcdCreateTimeout
	//}
	if c.DeprecatedEtcdRootVolumeIOPS != nil {
		fmt.Println("WARN: etcdRootVolumeIOPS is deprecated and will be removed in v0.9.7. Please use etcd.rootVolume.iops instead")
		c.Etcd.RootVolume.IOPS = *c.DeprecatedEtcdRootVolumeIOPS
	}
	if c.DeprecatedEtcdRootVolumeSize != nil {
		fmt.Println("WARN: etcdRootVolumeSize is deprecated and will be removed in v0.9.7. Please use etcd.rootVolume.size instead")
		c.Etcd.RootVolume.Size = *c.DeprecatedEtcdRootVolumeSize
	}
	if c.DeprecatedEtcdRootVolumeType != nil {
		fmt.Println("WARN: etcdRootVolumeType is deprecated and will be removed in v0.9.7. Please use etcd.rootVolume.type instead")
		c.Etcd.RootVolume.Type = *c.DeprecatedEtcdRootVolumeType
	}
	if c.DeprecatedEtcdDataVolumeIOPS != nil {
		fmt.Println("WARN: etcdDataVolumeIOPS is deprecated and will be removed in v0.9.7. Please use etcd.dataVolume.iops instead")
		c.Etcd.DataVolume.IOPS = *c.DeprecatedEtcdDataVolumeIOPS
	}
	if c.DeprecatedEtcdDataVolumeSize != nil {
		fmt.Println("WARN: etcdDataVolumeSize is deprecated and will be removed in v0.9.7. Please use etcd.dataVolume.size instead")
		c.Etcd.DataVolume.Size = *c.DeprecatedEtcdDataVolumeSize
	}
	if c.DeprecatedEtcdDataVolumeType != nil {
		fmt.Println("WARN: etcdDataVolumeType is deprecated and will be removed in v0.9.7. Please use etcd.dataVolume.type instead")
		c.Etcd.DataVolume.Type = *c.DeprecatedEtcdDataVolumeType
	}
	if c.DeprecatedEtcdDataVolumeEphemeral != nil {
		fmt.Println("WARN: etcdDataVolumeEphemeral is deprecated and will be removed in v0.9.7. Please use etcd.dataVolume.ephemeral instead")
		c.Etcd.DataVolume.Ephemeral = *c.DeprecatedEtcdDataVolumeEphemeral
	}
	if c.DeprecatedEtcdDataVolumeEncrypted != nil {
		fmt.Println("WARN: etcdDataVolumeEncrypted is deprecated and will be removed in v0.9.7. Please use etcd.dataVolume.encrypted instead")
		c.Etcd.DataVolume.Encrypted = *c.DeprecatedEtcdDataVolumeEncrypted
	}
}

func (c *Cluster) SetDefaults() {
	// For backward-compatibility
	if len(c.Subnets) == 0 {
		c.Subnets = []model.Subnet{
			model.NewPublicSubnet(c.AvailabilityZone, c.InstanceCIDR),
		}
	}

	privateTopologyImplied := c.RouteTableID != "" && !c.MapPublicIPs
	publicTopologyImplied := c.RouteTableID != "" && c.MapPublicIPs

	for i, s := range c.Subnets {
		if s.Name == "" {
			c.Subnets[i].Name = fmt.Sprintf("Subnet%d", i)
		}

		// DEPRECATED AND REMOVED IN THE FUTURE
		// See https://github.com/kubernetes-incubator/kube-aws/pull/284#issuecomment-275998862
		//
		// This implies a deployment to an existing VPC with a route table with a preconfigured Internet Gateway
		// and all the subnets created by kube-aws are public
		if publicTopologyImplied {
			c.Subnets[i].RouteTable.ID = c.RouteTableID
			if s.Private {
				panic(fmt.Sprintf("mapPublicIPs(=%v) and subnets[%d].private(=%v) conflicts: %+v", c.MapPublicIPs, i, s.Private, s))
			}
			c.Subnets[i].Private = false
		}

		// DEPRECATED AND REMOVED IN THE FUTURE
		// See https://github.com/kubernetes-incubator/kube-aws/pull/284#issuecomment-275998862
		//
		// This implies a deployment to an existing VPC with a route table with a preconfigured NAT Gateway
		// and all the subnets created by kube-aws are private
		if privateTopologyImplied {
			c.Subnets[i].RouteTable.ID = c.RouteTableID
			if s.Private {
				panic(fmt.Sprintf("mapPublicIPs(=%v) and subnets[%d].private(=%v) conflicts. You don't need to set true to both of them. If you want to make all the subnets private, make mapPublicIPs false. If you want to make only part of subnets private, make subnets[].private true accordingly: %+v", c.MapPublicIPs, i, s.Private, s))
			}
			c.Subnets[i].Private = true
		}
	}

	for i, s := range c.Controller.Subnets {
		linkedSubnet := c.FindSubnetMatching(s)
		c.Controller.Subnets[i] = linkedSubnet
	}

	for i, s := range c.Controller.LoadBalancer.Subnets {
		linkedSubnet := c.FindSubnetMatching(s)
		c.Controller.LoadBalancer.Subnets[i] = linkedSubnet
	}

	for i, s := range c.Etcd.Subnets {
		linkedSubnet := c.FindSubnetMatching(s)
		c.Etcd.Subnets[i] = linkedSubnet
	}

	if len(c.Controller.Subnets) == 0 {
		if privateTopologyImplied {
			c.Controller.Subnets = c.PrivateSubnets()
		} else {
			c.Controller.Subnets = c.PublicSubnets()
		}
	}

	if len(c.Controller.LoadBalancer.Subnets) == 0 {
		if c.Controller.LoadBalancer.Private || privateTopologyImplied {
			c.Controller.LoadBalancer.Subnets = c.PrivateSubnets()
			c.Controller.LoadBalancer.Private = true
		} else {
			c.Controller.LoadBalancer.Subnets = c.PublicSubnets()
		}
	}

	if len(c.Etcd.Subnets) == 0 {
		if privateTopologyImplied {
			c.Etcd.Subnets = c.PrivateSubnets()
		} else {
			c.Etcd.Subnets = c.PublicSubnets()
		}
	}
}

func ClusterFromBytesWithEncryptService(data []byte, encryptService EncryptService) (*Cluster, error) {
	cluster, err := ClusterFromBytes(data)
	if err != nil {
		return nil, err
	}
	cluster.ProvidedEncryptService = encryptService
	return cluster, nil
}

// Part of configuration which is shared between controller nodes and worker nodes.
// Its name is prefixed with `Kube` because it doesn't relate to etcd.
type KubeClusterSettings struct {
	APIEndpointConfigs model.APIEndpoints `yaml:"apiEndpoints,omitempty"`
	// Required by kubelet to locate the kube-apiserver
	ExternalDNSName string `yaml:"externalDNSName,omitempty"`
	// Required by kubelet to locate the cluster-internal dns hosted on controller nodes in the base cluster
	DNSServiceIP string `yaml:"dnsServiceIP,omitempty"`
	UseCalico    bool   `yaml:"useCalico,omitempty"`
}

// Part of configuration which can't be provided via user input but is computed from user input
type ComputedDeploymentSettings struct {
	AMI string
}

// Part of configuration which can be customized for each type/group of nodes(etcd/controller/worker/) by its nature.
//
// Please beware that it is described as just "by its nature".
// Whether it can actually be customized or not depends on you use node pools or not.
// If you've chosen to create a single cluster including all the worker, controller, etcd nodes within a single cfn stack,
// you can't customize per group of nodes.
// If you've chosen to create e.g. a separate node pool for each type of worker nodes,
// you can customize per node pool.
//
// Though it is highly configurable, it's basically users' responsibility to provide `correct` values if they're going beyond the defaults.
type DeploymentSettings struct {
	ComputedDeploymentSettings
	ClusterName       string       `yaml:"clusterName,omitempty"`
	KeyName           string       `yaml:"keyName,omitempty"`
	Region            model.Region `yaml:",inline"`
	AvailabilityZone  string       `yaml:"availabilityZone,omitempty"`
	ReleaseChannel    string       `yaml:"releaseChannel,omitempty"`
	AmiId             string       `yaml:"amiId,omitempty"`
	VPCID             string       `yaml:"vpcId,omitempty"`
	InternetGatewayID string       `yaml:"internetGatewayId,omitempty"`
	RouteTableID      string       `yaml:"routeTableId,omitempty"`
	// Required for validations like e.g. if instance cidr is contained in vpc cidr
	VPCCIDR             string            `yaml:"vpcCIDR,omitempty"`
	InstanceCIDR        string            `yaml:"instanceCIDR,omitempty"`
	K8sVer              string            `yaml:"kubernetesVersion,omitempty"`
	ContainerRuntime    string            `yaml:"containerRuntime,omitempty"`
	KMSKeyARN           string            `yaml:"kmsKeyArn,omitempty"`
	StackTags           map[string]string `yaml:"stackTags,omitempty"`
	Subnets             []model.Subnet    `yaml:"subnets,omitempty"`
	EIPAllocationIDs    []string          `yaml:"eipAllocationIDs,omitempty"`
	MapPublicIPs        bool              `yaml:"mapPublicIPs,omitempty"`
	ElasticFileSystemID string            `yaml:"elasticFileSystemId,omitempty"`
	SSHAuthorizedKeys   []string          `yaml:"sshAuthorizedKeys,omitempty"`
	Addons              model.Addons      `yaml:"addons"`
	Experimental        Experimental      `yaml:"experimental"`
	ManageCertificates  bool              `yaml:"manageCertificates,omitempty"`
	WaitSignal          WaitSignal        `yaml:"waitSignal"`

	// Images repository
	HyperkubeImage              model.Image `yaml:"hyperkubeImage,omitempty"`
	AWSCliImage                 model.Image `yaml:"awsCliImage,omitempty"`
	CalicoNodeImage             model.Image `yaml:"calicoNodeImage,omitempty"`
	CalicoCniImage              model.Image `yaml:"calicoCniImage,omitempty"`
	CalicoCtlImage              model.Image `yaml:"calicoCtlImage,omitempty"`
	CalicoPolicyControllerImage model.Image `yaml:"calicoPolicyControllerImage,omitempty"`
	ClusterAutoscalerImage      model.Image `yaml:"clusterAutoscalerImage,omitempty"`
	KubeDnsImage                model.Image `yaml:"kubeDnsImage,omitempty"`
	KubeDnsMasqImage            model.Image `yaml:"kubeDnsMasqImage,omitempty"`
	KubeReschedulerImage        model.Image `yaml:"kubeReschedulerImage,omitempty"`
	DnsMasqMetricsImage         model.Image `yaml:"dnsMasqMetricsImage,omitempty"`
	ExecHealthzImage            model.Image `yaml:"execHealthzImage,omitempty"`
	HeapsterImage               model.Image `yaml:"heapsterImage,omitempty"`
	AddonResizerImage           model.Image `yaml:"addonResizerImage,omitempty"`
	KubeDashboardImage          model.Image `yaml:"kubeDashboardImage,omitempty"`
	PauseImage                  model.Image `yaml:"pauseImage,omitempty"`
	FlannelImage                model.Image `yaml:"flannelImage,omitempty"`
}

// Part of configuration which is specific to worker nodes
type DefaultWorkerSettings struct {
	WorkerCount            int      `yaml:"workerCount,omitempty"`
	WorkerCreateTimeout    string   `yaml:"workerCreateTimeout,omitempty"`
	WorkerInstanceType     string   `yaml:"workerInstanceType,omitempty"`
	WorkerRootVolumeType   string   `yaml:"workerRootVolumeType,omitempty"`
	WorkerRootVolumeIOPS   int      `yaml:"workerRootVolumeIOPS,omitempty"`
	WorkerRootVolumeSize   int      `yaml:"workerRootVolumeSize,omitempty"`
	WorkerSpotPrice        string   `yaml:"workerSpotPrice,omitempty"`
	WorkerSecurityGroupIds []string `yaml:"workerSecurityGroupIds,omitempty"`
	WorkerTenancy          string   `yaml:"workerTenancy,omitempty"`
	WorkerTopologyPrivate  bool     `yaml:"workerTopologyPrivate,omitempty"`
}

// Part of configuration which is specific to controller nodes
type ControllerSettings struct {
	model.Controller                   `yaml:"controller,omitempty"`
	DeprecatedControllerCount          *int    `yaml:"controllerCount,omitempty"`
	DeprecatedControllerCreateTimeout  *string `yaml:"controllerCreateTimeout,omitempty"`
	DeprecatedControllerInstanceType   *string `yaml:"controllerInstanceType,omitempty"`
	DeprecatedControllerRootVolumeType *string `yaml:"controllerRootVolumeType,omitempty"`
	DeprecatedControllerRootVolumeIOPS *int    `yaml:"controllerRootVolumeIOPS,omitempty"`
	DeprecatedControllerRootVolumeSize *int    `yaml:"controllerRootVolumeSize,omitempty"`
	DeprecatedControllerTenancy        *string `yaml:"controllerTenancy,omitempty"`
}

func (c ControllerSettings) ControllerCount() int {
	fmt.Println("WARN: ControllerCount is deprecated and will be removed in v0.9.7. Please use Controller.Count instead")
	return c.Controller.Count
}

func (c ControllerSettings) ControllerCreateTimeout() string {
	fmt.Println("WARN: ControllerCreateTimeout is deprecated and will be removed in v0.9.7. Please use Controller.CreateTimeout instead")
	return c.Controller.CreateTimeout
}

func (c ControllerSettings) ControllerInstanceType() string {
	fmt.Println("WARN: ControllerInstanceType is deprecated and will be removed in v0.9.7. Please use Controller.InstanceType instead")
	return c.Controller.InstanceType
}

func (c ControllerSettings) ControllerRootVolumeType() string {
	fmt.Println("WARN: ControllerRootVolumeType is deprecated and will be removed in v0.9.7. Please use Controller.RootVolume.Type instead")
	return c.Controller.RootVolume.Type
}

func (c ControllerSettings) ControllerRootVolumeIOPS() int {
	fmt.Println("WARN: ControllerRootVolumeIOPS is deprecated and will be removed in v0.9.7. Please use Controller.RootVolume.IOPS instead")
	return c.Controller.RootVolume.IOPS
}

func (c ControllerSettings) ControllerRootVolumeSize() int {
	fmt.Println("WARN: ControllerRootVolumeSize is deprecated and will be removed in v0.9.7. Please use Controller.RootVolume.Size instead")
	return c.Controller.RootVolume.Size
}

func (c ControllerSettings) ControllerTenancy() string {
	fmt.Println("WARN: ControllerTenancy is deprecated and will be removed in v0.9.7. Please use Controller.Tenancy instead")
	return c.Controller.Tenancy
}

// Part of configuration which is specific to etcd nodes
type EtcdSettings struct {
	model.Etcd                        `yaml:"etcd,omitempty"`
	DeprecatedEtcdCount               *int    `yaml:"etcdCount"`
	DeprecatedEtcdInstanceType        *string `yaml:"etcdInstanceType,omitempty"`
	DeprecatedEtcdRootVolumeSize      *int    `yaml:"etcdRootVolumeSize,omitempty"`
	DeprecatedEtcdRootVolumeType      *string `yaml:"etcdRootVolumeType,omitempty"`
	DeprecatedEtcdRootVolumeIOPS      *int    `yaml:"etcdRootVolumeIOPS,omitempty"`
	DeprecatedEtcdDataVolumeSize      *int    `yaml:"etcdDataVolumeSize,omitempty"`
	DeprecatedEtcdDataVolumeType      *string `yaml:"etcdDataVolumeType,omitempty"`
	DeprecatedEtcdDataVolumeIOPS      *int    `yaml:"etcdDataVolumeIOPS,omitempty"`
	DeprecatedEtcdDataVolumeEphemeral *bool   `yaml:"etcdDataVolumeEphemeral,omitempty"`
	DeprecatedEtcdDataVolumeEncrypted *bool   `yaml:"etcdDataVolumeEncrypted,omitempty"`
	DeprecatedEtcdTenancy             *string `yaml:"etcdTenancy,omitempty"`
}

func (e EtcdSettings) EtcdCount() int {
	fmt.Println("WARN: EtcdCount is deprecated and will be removed in v0.9.7. Please use Etcd.Count instead")
	return e.Etcd.Count
}

func (e EtcdSettings) EtcdInstanceType() string {
	fmt.Println("WARN: EtcdInstanceType is deprecated and will be removed in v0.9.7. Please use Etcd.InstanceType instead")
	return e.Etcd.InstanceType
}

func (e EtcdSettings) EtcdRootVolumeSize() int {
	fmt.Println("WARN: EtcdRootVolumeSize is deprecated and will be removed in v0.9.7. Please use Etcd.RootVolume.Size instead")
	return e.Etcd.RootVolume.Size
}

func (e EtcdSettings) EtcdRootVolumeType() string {
	fmt.Println("WARN: EtcdRootVolumeType is deprecated and will be removed in v0.9.7. Please use Etcd.RootVolume.Type instead")
	return e.Etcd.RootVolume.Type
}

func (e EtcdSettings) EtcdRootVolumeIOPS() int {
	fmt.Println("WARN: EtcdRootVolumeIOPS is deprecated and will be removed in v0.9.7. Please use Etcd.RootVolume.IOPS instead")
	return e.Etcd.RootVolume.IOPS
}

func (e EtcdSettings) EtcdDataVolumeSize() int {
	fmt.Println("WARN: EtcdDataVolumeSize is deprecated and will be removed in v0.9.7. Please use Etcd.DataVolume.Size instead")
	return e.Etcd.DataVolume.Size
}

func (e EtcdSettings) EtcdDataVolumeType() string {
	fmt.Println("WARN: EtcdDataVolumeType is deprecated and will be removed in v0.9.7. Please use Etcd.DataVolume.Type instead")
	return e.Etcd.DataVolume.Type
}

func (e EtcdSettings) EtcdDataVolumeIOPS() int {
	fmt.Println("WARN: EtcdDataVolumeIOPS is deprecated and will be removed in v0.9.7. Please use Etcd.DataVolume.IOPS instead")
	return e.Etcd.DataVolume.IOPS
}

func (e EtcdSettings) EtcdDataVolumeEphemeral() bool {
	fmt.Println("WARN: EtcdDataVolumeEphemeral is deprecated and will be removed in v0.9.7. Please use Etcd.DataVolume.Ephemeral instead")
	return e.Etcd.DataVolume.Ephemeral
}

func (e EtcdSettings) EtcdDataVolumeEncrypted() bool {
	fmt.Println("WARN: EtcdDataVolumeEncrypted is deprecated and will be removed in v0.9.7. Please use Etcd.DataVolume.Encrypted instead")
	return e.Etcd.DataVolume.Encrypted
}

func (e EtcdSettings) EtcdTenancy() string {
	fmt.Println("WARN: EtcdTenancy is deprecated and will be removed in v0.9.7. Please use Etcd.Tenancy instead")
	return e.Etcd.Tenancy
}

// Part of configuration which is specific to flanneld
type FlannelSettings struct {
	PodCIDR string `yaml:"podCIDR,omitempty"`
}

type Cluster struct {
	KubeClusterSettings    `yaml:",inline"`
	DeploymentSettings     `yaml:",inline"`
	DefaultWorkerSettings  `yaml:",inline"`
	ControllerSettings     `yaml:",inline"`
	EtcdSettings           `yaml:",inline"`
	FlannelSettings        `yaml:",inline"`
	ServiceCIDR            string `yaml:"serviceCIDR,omitempty"`
	CreateRecordSet        bool   `yaml:"createRecordSet,omitempty"`
	RecordSetTTL           int    `yaml:"recordSetTTL,omitempty"`
	TLSCADurationDays      int    `yaml:"tlsCADurationDays,omitempty"`
	TLSCertDurationDays    int    `yaml:"tlsCertDurationDays,omitempty"`
	HostedZoneID           string `yaml:"hostedZoneId,omitempty"`
	ProvidedEncryptService EncryptService
	CustomSettings         map[string]interface{} `yaml:"customSettings,omitempty"`
}

type Experimental struct {
	Admission                   Admission                `yaml:"admission"`
	AuditLog                    AuditLog                 `yaml:"auditLog"`
	Authentication              Authentication           `yaml:"authentication"`
	AwsEnvironment              AwsEnvironment           `yaml:"awsEnvironment"`
	AwsNodeLabels               AwsNodeLabels            `yaml:"awsNodeLabels"`
	ClusterAutoscalerSupport    ClusterAutoscalerSupport `yaml:"clusterAutoscalerSupport"`
	TLSBootstrap                TLSBootstrap             `yaml:"tlsBootstrap"`
	EphemeralImageStorage       EphemeralImageStorage    `yaml:"ephemeralImageStorage"`
	Kube2IamSupport             Kube2IamSupport          `yaml:"kube2IamSupport,omitempty"`
	LoadBalancer                LoadBalancer             `yaml:"loadBalancer"`
	TargetGroup                 TargetGroup              `yaml:"targetGroup"`
	NodeDrainer                 NodeDrainer              `yaml:"nodeDrainer"`
	NodeLabels                  NodeLabels               `yaml:"nodeLabels"`
	Plugins                     Plugins                  `yaml:"plugins"`
	DisableSecurityGroupIngress bool                     `yaml:"disableSecurityGroupIngress"`
	NodeMonitorGracePeriod      string                   `yaml:"nodeMonitorGracePeriod"`
	Taints                      []Taint                  `yaml:"taints"`
	model.UnknownKeys           `yaml:",inline"`
}

type Admission struct {
	PodSecurityPolicy PodSecurityPolicy `yaml:"podSecurityPolicy"`
}

type PodSecurityPolicy struct {
	Enabled bool `yaml:"enabled"`
}

type AuditLog struct {
	Enabled bool   `yaml:"enabled"`
	MaxAge  int    `yaml:"maxage"`
	LogPath string `yaml:"logpath"`
}

type Authentication struct {
	Webhook Webhook `yaml:"webhook"`
}

type Webhook struct {
	Enabled  bool   `yaml:"enabled"`
	CacheTTL string `yaml:"cacheTTL"`
	Config   string `yaml:"configBase64"`
}

type AwsEnvironment struct {
	Enabled     bool              `yaml:"enabled"`
	Environment map[string]string `yaml:"environment"`
}

type AwsNodeLabels struct {
	Enabled bool `yaml:"enabled"`
}

type ClusterAutoscalerSupport struct {
	Enabled bool `yaml:"enabled"`
}

type TLSBootstrap struct {
	Enabled bool `yaml:"enabled"`
}

type EphemeralImageStorage struct {
	Enabled    bool   `yaml:"enabled"`
	Disk       string `yaml:"disk"`
	Filesystem string `yaml:"filesystem"`
}

type Kube2IamSupport struct {
	Enabled bool `yaml:"enabled"`
}

type NodeDrainer struct {
	Enabled bool `yaml:"enabled"`
}

type NodeLabels map[string]string

func (l NodeLabels) Enabled() bool {
	return len(l) > 0
}

// Returns key=value pairs separated by ',' to be passed to kubelet's `--node-labels` flag
func (l NodeLabels) String() string {
	labels := []string{}
	keys := []string{}
	for k, _ := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := l[k]
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(labels, ",")
}

type LoadBalancer struct {
	Enabled          bool     `yaml:"enabled"`
	Names            []string `yaml:"names"`
	SecurityGroupIds []string `yaml:"securityGroupIds"`
}

type TargetGroup struct {
	Enabled          bool     `yaml:"enabled"`
	Arns             []string `yaml:"arns"`
	SecurityGroupIds []string `yaml:"securityGroupIds"`
}

type Plugins struct {
	Rbac Rbac `yaml:"rbac"`
}

type Rbac struct {
	Enabled bool `yaml:"enabled"`
}

type Taint struct {
	Key    string `yaml:"key"`
	Value  string `yaml:"value"`
	Effect string `yaml:"effect"`
}

func (t Taint) String() string {
	return fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect)
}

type WaitSignal struct {
	// WaitSignal is enabled by default. If you'd like to explicitly disable it, set this to `false`.
	// Keeping this `nil` results in the WaitSignal to be enabled.
	EnabledOverride      *bool `yaml:"enabled"`
	MaxBatchSizeOverride *int  `yaml:"maxBatchSize"`
}

func (s WaitSignal) Enabled() bool {
	if s.EnabledOverride != nil {
		return *s.EnabledOverride
	}
	return true
}

func (s WaitSignal) MaxBatchSize() int {
	if s.MaxBatchSizeOverride != nil {
		return *s.MaxBatchSizeOverride
	}
	return 1
}

const (
	vpcLogicalName             = "VPC"
	internetGatewayLogicalName = "InternetGateway"
)

var supportedReleaseChannels = map[string]bool{
	"alpha":  true,
	"beta":   true,
	"stable": true,
}

func (c ControllerSettings) MinControllerCount() int {
	if c.Controller.AutoScalingGroup.MinSize == nil {
		return c.Controller.Count
	}
	return *c.Controller.AutoScalingGroup.MinSize
}

func (c ControllerSettings) MaxControllerCount() int {
	if c.Controller.AutoScalingGroup.MaxSize == 0 {
		return c.Controller.Count
	}
	return c.Controller.AutoScalingGroup.MaxSize
}

func (c ControllerSettings) ControllerRollingUpdateMinInstancesInService() int {
	if c.AutoScalingGroup.RollingUpdateMinInstancesInService == nil {
		return c.MaxControllerCount() - 1
	}
	return *c.AutoScalingGroup.RollingUpdateMinInstancesInService
}

// Required by kubelet to locate the apiserver
func (c KubeClusterSettings) APIServerEndpoint() string {
	return fmt.Sprintf("https://%s", c.ExternalDNSName)
}

// Required by kubelet to use the consistent network plugin with the base cluster
func (c KubeClusterSettings) K8sNetworkPlugin() string {
	return "cni"
}

func (c Cluster) Config() (*Config, error) {
	config := Config{Cluster: c}

	// Check if we are running CoreOS 1151.0.0 or greater when using rkt as
	// runtime. Proceed regardless if running alpha. TODO(pb) delete when rkt
	// works well with stable.
	if config.ContainerRuntime == "rkt" && config.ReleaseChannel != "alpha" {
		minVersion := semver.Version{Major: 1151}

		ok, err := releaseVersionIsGreaterThan(minVersion, config.ReleaseChannel)
		if err != nil {
			return nil, err
		}

		if !ok {
			return nil, fmt.Errorf("The container runtime is 'rkt' but the latest CoreOS version for the %s channel is less then the minimum version %s. Please select the 'alpha' release channel to use the rkt runtime.", config.ReleaseChannel, minVersion)
		}
	}

	if c.AmiId == "" {
		var err error
		if config.AMI, err = amiregistry.GetAMI(config.Region.String(), config.ReleaseChannel); err != nil {
			return nil, fmt.Errorf("failed getting AMI for config: %v", err)
		}
	} else {
		config.AMI = c.AmiId
	}

	var err error
	config.EtcdNodes, err = derived.NewEtcdNodes(c.Etcd.Nodes, c.EtcdCluster())
	if err != nil {
		return nil, fmt.Errorf("failed to derived etcd nodes configuration: %v", err)
	}

	// Populate top-level subnets to model
	if len(config.Subnets) > 0 {
		if config.ControllerSettings.MinControllerCount() > 0 && len(config.ControllerSettings.Subnets) == 0 {
			config.ControllerSettings.Subnets = config.Subnets
		}
	}

	apiEndpoints, err := derived.NewAPIEndpoints(c.APIEndpointConfigs, c.Subnets)
	if err != nil {
		return nil, fmt.Errorf("invalid cluster: %v", err)
	}

	config.APIEndpoints = apiEndpoints

	return &config, nil
}

func (c *Cluster) EtcdCluster() derived.EtcdCluster {
	etcdNetwork := derived.NewNetwork(c.Etcd.Subnets, c.NATGateways())
	return derived.NewEtcdCluster(c.Etcd.Cluster, c.Region, etcdNetwork, c.Etcd.Count)
}

// releaseVersionIsGreaterThan will return true if the supplied version is greater then
// or equal to the current CoreOS release indicated by the given release
// channel.
func releaseVersionIsGreaterThan(minVersion semver.Version, release string) (bool, error) {
	metaData, err := amiregistry.GetAMIData(release)
	if err != nil {
		return false, fmt.Errorf("Unable to retrieve current release channel version: %v", err)
	}

	version, ok := metaData["release_info"]["version"]
	if !ok {
		return false, fmt.Errorf("Error parsing image metadata for version")
	}

	current, err := semver.NewVersion(version)
	if err != nil {
		return false, fmt.Errorf("Error parsing semver from image version %v", err)
	}

	if current.LessThan(minVersion) {
		return false, nil
	}

	return true, nil
}

type StackTemplateOptions struct {
	AssetsDir             string
	ControllerTmplFile    string
	EtcdTmplFile          string
	StackTemplateTmplFile string
	S3URI                 string
	PrettyPrint           bool
	SkipWait              bool
}

func (c Cluster) StackConfig(opts StackTemplateOptions) (*StackConfig, error) {
	var err error
	stackConfig := StackConfig{}

	if stackConfig.Config, err = c.Config(); err != nil {
		return nil, err
	}

	var compactAssets *CompactTLSAssets
	var compactAuthTokens *CompactAuthTokens

	if c.AssetsEncryptionEnabled() {
		compactAuthTokens, err = ReadOrCreateCompactAuthTokens(opts.AssetsDir, KMSConfig{
			Region:         stackConfig.Config.Region,
			KMSKeyARN:      c.KMSKeyARN,
			EncryptService: c.ProvidedEncryptService,
		})
		if err != nil {
			return nil, err
		}
		stackConfig.Config.AuthTokensConfig = compactAuthTokens
	} else {
		rawAuthTokens, err := ReadOrCreateUnencryptedCompactAuthTokens(opts.AssetsDir)
		if err != nil {
			return nil, err
		}
		stackConfig.Config.AuthTokensConfig = rawAuthTokens
	}
	if c.ManageCertificates {
		if c.AssetsEncryptionEnabled() {
			compactAssets, err = ReadOrCreateCompactTLSAssets(opts.AssetsDir, KMSConfig{
				Region:         stackConfig.Config.Region,
				KMSKeyARN:      c.KMSKeyARN,
				EncryptService: c.ProvidedEncryptService,
			})

			stackConfig.Config.TLSConfig = compactAssets
		} else {
			rawAssets, err := ReadOrCreateUnencryptedCompactTLSAssets(opts.AssetsDir)
			if err != nil {
				return nil, err
			}

			stackConfig.Config.TLSConfig = rawAssets
		}
	}

	if stackConfig.UserDataController, err = userdatatemplate.GetString(opts.ControllerTmplFile, stackConfig.Config); err != nil {
		return nil, fmt.Errorf("failed to render controller cloud config: %v", err)
	}
	if stackConfig.UserDataEtcd, err = userdatatemplate.GetString(opts.EtcdTmplFile, stackConfig.Config); err != nil {
		return nil, fmt.Errorf("failed to render etcd cloud config: %v", err)
	}
	if len(stackConfig.Config.AuthTokensConfig.KubeletBootstrapToken) == 0 && c.DeploymentSettings.Experimental.TLSBootstrap.Enabled {
		bootstrapRecord, err := RandomBootstrapTokenRecord()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("kubelet bootstrap token not found in ./credentials/tokens.csv\n\nTo fix this, append the following line to ./credentials.tokens.csv:\n%s", bootstrapRecord)
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

type Config struct {
	Cluster

	APIEndpoints derived.APIEndpoints

	EtcdNodes []derived.EtcdNode

	AuthTokensConfig *CompactAuthTokens
	TLSConfig        *CompactTLSAssets
}

// StackName returns the logical name of a CloudFormation stack resource in a root stack template
// This is not needed to be unique in an AWS account because the actual name of a nested stack is generated randomly
// by CloudFormation by including the logical name.
// This is NOT intended to be used to reference stack name from cloud-config as the target of awscli or cfn-bootstrap-tools commands e.g. `cfn-init` and `cfn-signal`
func (c Cluster) StackName() string {
	return "control-plane"
}

func (c Cluster) StackNameEnvVarName() string {
	return "KUBE_AWS_STACK_NAME"
}

func (c Cluster) EtcdNodeEnvFileName() string {
	return "/var/run/coreos/etcd-node.env"
}

func (c Cluster) EtcdIndexEnvVarName() string {
	return "KUBE_AWS_ETCD_INDEX"
}

func (c Config) VPCLogicalName() string {
	return vpcLogicalName
}

func (c Config) VPCRef() string {
	if c.VPCID != "" {
		return fmt.Sprintf("%q", c.VPCID)
	} else {
		return fmt.Sprintf(`{ "Ref" : %q }`, c.VPCLogicalName())
	}
}

func (c Config) InternetGatewayLogicalName() string {
	return internetGatewayLogicalName
}

func (c Config) InternetGatewayRef() string {
	if c.InternetGatewayID != "" {
		return fmt.Sprintf("%q", c.InternetGatewayID)
	} else {
		return fmt.Sprintf(`{ "Ref" : %q }`, c.InternetGatewayLogicalName())
	}
}

// ExternalDNSNames returns all the DNS names of Kubernetes API endpoints should be covered in the TLS cert for k8s API
func (c Cluster) ExternalDNSNames() []string {
	names := []string{}

	if c.ExternalDNSName != "" {
		names = append(names, c.ExternalDNSName)
	}

	for _, e := range c.APIEndpointConfigs {
		names = append(names, e.DNSName)
	}

	sort.Strings(names)

	return names
}

// NestedStackName returns a sanitized name of this control-plane which is usable as a valid cloudformation nested stack name
func (c Cluster) NestedStackName() string {
	// Convert stack name into something valid as a cfn resource name or
	// we'll end up with cfn errors like "Template format error: Resource name test5-controlplane is non alphanumeric"
	return strings.Title(strings.Replace(c.StackName(), "-", "", -1))
}

// Etcdadm returns the content of the etcdadm script to be embedded into cloud-config-etcd
func (c *Config) Etcdadm() (string, error) {
	return gzipcompressor.CompressData(Etcdadm)
}

func (c Cluster) valid() error {
	validClusterNaming := regexp.MustCompile("^[a-zA-Z0-9-:]+$")
	if !validClusterNaming.MatchString(c.ClusterName) {
		return fmt.Errorf("clusterName(=%s) is malformed. It must consist only of alphanumeric characters, colons, or hyphens", c.ClusterName)
	}

	if c.CreateRecordSet {
		if c.HostedZoneID == "" {
			return errors.New("hostedZoneID must be specified when createRecordSet is true")
		}

		if c.RecordSetTTL < 1 {
			return errors.New("TTL must be at least 1 second")
		}
	} else {
		if c.RecordSetTTL != NewDefaultCluster().RecordSetTTL {
			return errors.New(
				"recordSetTTL should not be modified when createRecordSet is false",
			)
		}

		if c.HostedZoneID != "" {
			return errors.New(
				"hostedZoneId should not be modified when createRecordSet is false",
			)
		}
	}

	var dnsServiceIPAddr net.IP

	if kubeClusterValidationResult, err := c.KubeClusterSettings.Valid(); err != nil {
		return err
	} else {
		dnsServiceIPAddr = kubeClusterValidationResult.dnsServiceIPAddr
	}

	var vpcNet *net.IPNet

	if deploymentValidationResult, err := c.DeploymentSettings.Valid(); err != nil {
		return err
	} else {
		vpcNet = deploymentValidationResult.vpcNet
	}

	_, podNet, err := net.ParseCIDR(c.PodCIDR)
	if err != nil {
		return fmt.Errorf("invalid podCIDR: %v", err)
	}

	_, serviceNet, err := net.ParseCIDR(c.ServiceCIDR)
	if err != nil {
		return fmt.Errorf("invalid serviceCIDR: %v", err)
	}
	if netutil.CidrOverlap(serviceNet, vpcNet) {
		return fmt.Errorf("vpcCIDR (%s) overlaps with serviceCIDR (%s)", c.VPCCIDR, c.ServiceCIDR)
	}
	if netutil.CidrOverlap(podNet, vpcNet) {
		return fmt.Errorf("vpcCIDR (%s) overlaps with podCIDR (%s)", c.VPCCIDR, c.PodCIDR)
	}
	if netutil.CidrOverlap(serviceNet, podNet) {
		return fmt.Errorf("serviceCIDR (%s) overlaps with podCIDR (%s)", c.ServiceCIDR, c.PodCIDR)
	}

	kubernetesServiceIPAddr := netutil.IncrementIP(serviceNet.IP)
	if !serviceNet.Contains(kubernetesServiceIPAddr) {
		return fmt.Errorf("serviceCIDR (%s) does not contain kubernetesServiceIP (%s)", c.ServiceCIDR, kubernetesServiceIPAddr)
	}

	if !serviceNet.Contains(dnsServiceIPAddr) {
		return fmt.Errorf("serviceCIDR (%s) does not contain dnsServiceIP (%s)", c.ServiceCIDR, c.DNSServiceIP)
	}

	if dnsServiceIPAddr.Equal(kubernetesServiceIPAddr) {
		return fmt.Errorf("dnsServiceIp conflicts with kubernetesServiceIp (%s)", dnsServiceIPAddr)
	}

	if err := c.ControllerSettings.Valid(); err != nil {
		return err
	}

	if err := c.DefaultWorkerSettings.Valid(); err != nil {
		return err
	}

	if err := c.EtcdSettings.Valid(); err != nil {
		return err
	}

	if c.WorkerTenancy != "default" && c.WorkerSpotPrice != "" {
		return fmt.Errorf("selected worker tenancy (%s) is incompatible with spot instances", c.WorkerTenancy)
	}

	clusterNamePlaceholder := "<my-cluster-name>"
	nestedStackNamePlaceHolder := "<my-nested-stack-name>"
	replacer := strings.NewReplacer(clusterNamePlaceholder, "", nestedStackNamePlaceHolder, c.StackName())
	simulatedLcName := fmt.Sprintf("%s-%s-1N2C4K3LLBEDZ-%sLC-BC2S9P3JG2QD", clusterNamePlaceholder, nestedStackNamePlaceHolder, c.Controller.LogicalName())
	limit := 63 - len(replacer.Replace(simulatedLcName))
	if c.Experimental.AwsNodeLabels.Enabled && len(c.ClusterName) > limit {
		return fmt.Errorf("awsNodeLabels can't be enabled for controllers because the total number of characters in clusterName(=\"%s\") exceeds the limit of %d", c.ClusterName, limit)
	}

	if c.Controller.InstanceType == "t2.micro" || c.Etcd.InstanceType == "t2.micro" || c.Controller.InstanceType == "t2.nano" || c.Etcd.InstanceType == "t2.nano" {
		fmt.Println(`WARNING: instance types "t2.nano" and "t2.micro" are not recommended. See https://github.com/kubernetes-incubator/kube-aws/issues/258 for more information`)
	}

	if c.Experimental.TLSBootstrap.Enabled && !c.Experimental.Plugins.Rbac.Enabled {
		fmt.Println(`WARNING: enabling cluster-level TLS bootstrapping without RBAC is not recommended. See https://kubernetes.io/docs/admin/kubelet-tls-bootstrapping/ for more information`)
	}

	if e := cfnresource.ValidateRoleNameLength(c.ClusterName, c.NestedStackName(), c.Controller.ManagedIamRoleName, c.Region.String()); e != nil {
		return e
	}

	return nil
}

type InfrastructureValidationResult struct {
	dnsServiceIPAddr net.IP
}

func (c KubeClusterSettings) Valid() (*InfrastructureValidationResult, error) {
	if c.ExternalDNSName == "" && len(c.APIEndpointConfigs) == 0 {
		return nil, errors.New("Either externalDNSName or apiEndpoints must be set")
	}

	if err := c.APIEndpointConfigs.Validate(); err != nil {
		return nil, err
	}

	dnsServiceIPAddr := net.ParseIP(c.DNSServiceIP)
	if dnsServiceIPAddr == nil {
		return nil, fmt.Errorf("Invalid dnsServiceIP: %s", c.DNSServiceIP)
	}

	return &InfrastructureValidationResult{dnsServiceIPAddr: dnsServiceIPAddr}, nil
}

type DeploymentValidationResult struct {
	vpcNet *net.IPNet
}

func (c DeploymentSettings) Valid() (*DeploymentValidationResult, error) {
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
	if c.KMSKeyARN == "" && c.AssetsEncryptionEnabled() {
		return nil, errors.New("kmsKeyArn must be set")
	}

	if c.VPCID == "" && (c.RouteTableID != "" || c.InternetGatewayID != "") {
		return nil, errors.New("vpcId must be specified if routeTableId or internetGatewayId are specified")
	}

	if c.Region.IsEmpty() {
		return nil, errors.New("region must be set")
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

		for i, subnet := range c.Subnets {
			if subnet.Validate(); err != nil {
				return nil, fmt.Errorf("failed to validate subnet: %v", err)
			}
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

			if subnet.RouteTableID() != "" && c.RouteTableID != "" {
				return nil, fmt.Errorf("either subnets[].routeTable.id(%s) or routeTableId(%s) but not both can be specified", subnet.RouteTableID(), c.RouteTableID)
			}

			allPrivate = allPrivate && subnet.Private
			allPublic = allPublic && subnet.Public()
		}

		if c.RouteTableID != "" && !allPublic && !allPrivate {
			return nil, fmt.Errorf("network topology including both private and public subnets specified while the single route table(%s) is also specified. You must differentiate the route table at least between private and public subnets. Use subets[].routeTable.id instead of routeTableId for that.", c.RouteTableID)
		}

		for i, a := range instanceCIDRs {
			for j, b := range instanceCIDRs[i+1:] {
				if netutil.CidrOverlap(a, b) {
					return nil, fmt.Errorf("CIDR of subnet %d (%s) overlaps with CIDR of subnet %d (%s)", i, a, j, b)
				}
			}
		}
	}

	if err := c.Experimental.Valid(); err != nil {
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

func (s DeploymentSettings) AllSubnets() []model.Subnet {
	subnets := s.Subnets
	return subnets
}

func (c DeploymentSettings) FindSubnetMatching(condition model.Subnet) model.Subnet {
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

func (c DeploymentSettings) PrivateSubnets() []model.Subnet {
	result := []model.Subnet{}
	for _, s := range c.Subnets {
		if s.Private {
			result = append(result, s)
		}
	}
	return result
}

func (c DeploymentSettings) PublicSubnets() []model.Subnet {
	result := []model.Subnet{}
	for _, s := range c.Subnets {
		if !s.Private {
			result = append(result, s)
		}
	}
	return result
}

func (c DeploymentSettings) FindNATGatewayForPrivateSubnet(s model.Subnet) (*model.NATGateway, error) {
	for _, ngw := range c.NATGateways() {
		if ngw.IsConnectedToPrivateSubnet(s) {
			return &ngw, nil
		}
	}
	return nil, fmt.Errorf("No NATGateway found for the subnet %v", s)
}

func (c DeploymentSettings) NATGateways() []model.NATGateway {
	ngws := []model.NATGateway{}
	for _, privateSubnet := range c.PrivateSubnets() {
		var publicSubnet model.Subnet
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
			ngw := model.NewManagedNATGateway(ngwConfig, privateSubnet, publicSubnet)
			ngws = append(ngws, ngw)
		} else if ngwConfig.HasIdentifier() {
			ngw := model.NewUnmanagedNATGateway(ngwConfig, privateSubnet)
			ngws = append(ngws, ngw)
		}
	}
	return ngws
}

func (c DefaultWorkerSettings) Valid() error {
	if c.WorkerRootVolumeType == "io1" {
		if c.WorkerRootVolumeIOPS < 100 || c.WorkerRootVolumeIOPS > 2000 {
			return fmt.Errorf("invalid workerRootVolumeIOPS: %d", c.WorkerRootVolumeIOPS)
		}
	} else {
		if c.WorkerRootVolumeIOPS != 0 {
			return fmt.Errorf("invalid workerRootVolumeIOPS for volume type '%s': %d", c.WorkerRootVolumeType, c.WorkerRootVolumeIOPS)
		}

		if c.WorkerRootVolumeType != "standard" && c.WorkerRootVolumeType != "gp2" {
			return fmt.Errorf("invalid workerRootVolumeType: %s", c.WorkerRootVolumeType)
		}
	}

	if c.WorkerCount != 0 {
		return errors.New("`workerCount` is removed. Set worker.nodePools[].count per node pool instead")
	}

	return nil
}

func (c ControllerSettings) Valid() error {
	controller := c.Controller
	rootVolume := controller.RootVolume

	if rootVolume.Type == "io1" {
		if rootVolume.IOPS < 100 || rootVolume.IOPS > 2000 {
			return fmt.Errorf("invalid controller.rootVolume.iops: %d", rootVolume.IOPS)
		}
	} else {
		if rootVolume.IOPS != 0 {
			return fmt.Errorf("invalid controller.rootVolume.iops for type \"%s\": %d", rootVolume.Type, rootVolume.IOPS)
		}

		if rootVolume.Type != "standard" && rootVolume.Type != "gp2" {
			return fmt.Errorf("invalid controller.rootVolume.type: %s in %+v", rootVolume.Type, c)
		}
	}

	if controller.Count < 0 {
		return fmt.Errorf("`controller.count` must be zero or greater if specified or otherwrise omitted, but was: %d", controller.Count)
	}
	// one is the default Controller.Count
	asg := c.AutoScalingGroup
	if controller.Count != model.DefaultControllerCount && (asg.MinSize != nil && *asg.MinSize != 0 || asg.MaxSize != 0) {
		return errors.New("`controller.autoScalingGroup.minSize` and `controller.autoScalingGroup.maxSize` can only be specified without `controller.count`")
	}

	if err := controller.Validate(); err != nil {
		return err
	}

	return nil
}

// Valid returns an error when there's any user error in the `etcd` settings
func (e EtcdSettings) Valid() error {
	if !e.Etcd.DataVolume.Encrypted && e.Etcd.KMSKeyARN() != "" {
		return errors.New("`etcd.kmsKeyArn` can only be specified when `etcdDataVolumeEncrypted` is enabled")
	}

	if e.Etcd.Version().Is3() {
		if e.Etcd.DisasterRecovery.Automated && !e.Etcd.Snapshot.Automated {
			return errors.New("`etcd.disasterRecovery.automated` is set to true but `etcd.snapshot.automated` is not - automated disaster recovery requires snapshot to be also automated")
		}
	} else {
		if e.Etcd.DisasterRecovery.Automated {
			return errors.New("`etcd.disasterRecovery.automated` is set to true for enabling automated disaster recovery. However the feature is available only for etcd version 3")
		}
		if e.Etcd.Snapshot.Automated {
			return errors.New("`etcd.snapshot.automated` is set to true for enabling automated snapshot. However the feature is available only for etcd version 3")
		}
	}

	return nil
}

func (c Experimental) Valid() error {
	for _, taint := range c.Taints {
		if taint.Effect != "NoSchedule" && taint.Effect != "PreferNoSchedule" {
			return fmt.Errorf("Effect must be NoSchedule or PreferNoSchedule, but was %s", taint.Effect)
		}
	}

	return nil
}

/*
Returns the availability zones referenced by the cluster configuration
*/
func (c *Cluster) AvailabilityZones() []string {
	if len(c.Subnets) == 0 {
		return []string{c.AvailabilityZone}
	}

	result := []string{}
	seen := map[string]bool{}
	for _, s := range c.Subnets {
		val := s.AvailabilityZone
		if _, ok := seen[val]; !ok {
			result = append(result, val)
			seen[val] = true
		}
	}
	return result
}

/*
Validates the an existing VPC and it's existing subnets do not conflict with this
cluster configuration
*/
func (c *Cluster) ValidateExistingVPC(existingVPCCIDR string, existingSubnetCIDRS []string) error {
	_, existingVPC, err := net.ParseCIDR(existingVPCCIDR)
	if err != nil {
		return fmt.Errorf("error parsing existing vpc cidr %s : %v", existingVPCCIDR, err)
	}

	existingSubnets := make([]*net.IPNet, len(existingSubnetCIDRS))
	for i, existingSubnetCIDR := range existingSubnetCIDRS {
		_, existingSubnets[i], err = net.ParseCIDR(existingSubnetCIDR)
		if err != nil {
			return fmt.Errorf(
				"error parsing existing subnet cidr %s : %v",
				existingSubnetCIDR,
				err,
			)
		}
	}

	_, vpcNet, err := net.ParseCIDR(c.VPCCIDR)
	if err != nil {
		return fmt.Errorf("error parsing vpc cidr %s: %v", c.VPCCIDR, err)
	}

	//Verify that existing vpc CIDR matches declared vpc CIDR
	if vpcNet.String() != existingVPC.String() {
		return fmt.Errorf(
			"declared vpcCidr %s does not match existing vpc cidr %s",
			vpcNet,
			existingVPC,
		)
	}

	// Loop through all subnets
	// Note: legacy instanceCIDR/availabilityZone stuff has already been marshalled into this format
	for _, subnet := range c.Subnets {
		if subnet.ID != "" {
			continue
		} else {
			_, instanceNet, err := net.ParseCIDR(subnet.InstanceCIDR)
			if err != nil {
				return fmt.Errorf("error parsing instances cidr %s : %v", c.InstanceCIDR, err)
			}

			//Loop through all existing subnets in the VPC and look for conflicting CIDRS
			for _, existingSubnet := range existingSubnets {
				if netutil.CidrOverlap(instanceNet, existingSubnet) {
					return fmt.Errorf(
						"instance cidr (%s) conflicts with existing subnet cidr=%s",
						instanceNet,
						existingSubnet,
					)
				}
			}
		}
	}

	return nil
}

// ManageELBLogicalNames returns all the logical names of the cfn resources corresponding to ELBs managed by kube-aws for API endpoints
func (c *Config) ManagedELBLogicalNames() []string {
	return c.APIEndpoints.ManagedELBLogicalNames()
}

func WithTrailingDot(s string) string {
	if s == "" {
		return s
	}
	lastRune, _ := utf8.DecodeLastRuneInString(s)
	if lastRune != rune('.') {
		return s + "."
	}
	return s
}

const hostedZoneIDPrefix = "/hostedzone/"

func withHostedZoneIDPrefix(id string) string {
	if id == "" {
		return ""
	}
	if !strings.HasPrefix(id, hostedZoneIDPrefix) {
		return fmt.Sprintf("%s%s", hostedZoneIDPrefix, id)
	}
	return id
}
