package config

//go:generate go run ../codegen/templates_gen.go CloudConfigController=cloud-config-controller CloudConfigWorker=cloud-config-worker CloudConfigEtcd=cloud-config-etcd DefaultClusterConfig=cluster.yaml KubeConfigTemplate=kubeconfig.tmpl StackTemplateTemplate=stack-template.json
//go:generate gofmt -w templates.go

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"unicode/utf8"

	"github.com/coreos/go-semver/semver"
	"github.com/coreos/kube-aws/coreos/amiregistry"
	"github.com/coreos/kube-aws/coreos/userdatavalidation"
	"github.com/coreos/kube-aws/filereader/jsontemplate"
	"github.com/coreos/kube-aws/filereader/userdatatemplate"
	model "github.com/coreos/kube-aws/model"
	"github.com/coreos/kube-aws/netutil"
	yaml "gopkg.in/yaml.v2"
)

const (
	credentialsDir = "credentials"
	userDataDir    = "userdata"
)

func NewDefaultCluster() *Cluster {
	experimental := Experimental{
		AuditLog{
			Enabled: false,
			MaxAge:  30,
			LogPath: "/dev/stdout",
		},
		AwsEnvironment{
			Enabled: false,
		},
		EphemeralImageStorage{
			Enabled:    false,
			Disk:       "xvdb",
			Filesystem: "xfs",
		},
		LoadBalancer{
			Enabled: false,
		},
		NodeDrainer{
			Enabled: false,
		},
		NodeLabel{
			Enabled: false,
		},
		Plugins{
			Rbac{
				Enabled: false,
			},
		},
		[]Taint{},
		WaitSignal{
			Enabled:      false,
			MaxBatchSize: 1,
		},
	}

	return &Cluster{
		DeploymentSettings: DeploymentSettings{
			ClusterName:        "kubernetes",
			VPCCIDR:            "10.0.0.0/16",
			ReleaseChannel:     "stable",
			K8sVer:             "v1.4.6_coreos.0",
			HyperkubeImageRepo: "quay.io/coreos/hyperkube",
			AWSCliImageRepo:    "quay.io/coreos/awscli",
			AWSCliTag:          "master",
			ContainerRuntime:   "docker",
			Subnets:            []*Subnet{},
			MapPublicIPs:       true,
			Experimental:       experimental,
		},
		KubeClusterSettings: KubeClusterSettings{
			DNSServiceIP: "10.3.0.10",
		},
		WorkerSettings: WorkerSettings{
			WorkerCount:            1,
			WorkerCreateTimeout:    "PT15M",
			WorkerInstanceType:     "m3.medium",
			WorkerRootVolumeType:   "gp2",
			WorkerRootVolumeIOPS:   0,
			WorkerRootVolumeSize:   30,
			WorkerSecurityGroupIds: []string{},
		},
		ControllerSettings: ControllerSettings{
			ControllerCount:          1,
			ControllerCreateTimeout:  "PT15M",
			ControllerInstanceType:   "m3.medium",
			ControllerRootVolumeType: "gp2",
			ControllerRootVolumeIOPS: 0,
			ControllerRootVolumeSize: 30,
		},
		EtcdSettings: EtcdSettings{
			EtcdCount:          1,
			EtcdInstanceType:   "m3.medium",
			EtcdRootVolumeSize: 30,
			EtcdRootVolumeType: "gp2",
			EtcdRootVolumeIOPS: 0,
			EtcdDataVolumeSize: 30,
			EtcdDataVolumeType: "gp2",
			EtcdDataVolumeIOPS: 0,
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
	}
}

func newDefaultClusterWithDeps(encSvc EncryptService) *Cluster {
	cluster := NewDefaultCluster()
	cluster.providedEncryptService = encSvc
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

	// HostedZone needs to end with a '.', amazon will not append it for you.
	// as it will with RecordSets
	c.HostedZone = WithTrailingDot(c.HostedZone)

	// If the user specified no subnets, we assume that a single AZ configuration with the default instanceCIDR is demanded
	if len(c.Subnets) == 0 && c.InstanceCIDR == "" {
		c.InstanceCIDR = "10.0.0.0/24"
	}

	c.HostedZoneID = withHostedZoneIDPrefix(c.HostedZoneID)

	if err := c.valid(); err != nil {
		return nil, fmt.Errorf("invalid cluster: %v", err)
	}

	// For backward-compatibility
	if len(c.Subnets) == 0 {
		c.Subnets = []*Subnet{
			{
				AvailabilityZone: c.AvailabilityZone,
				InstanceCIDR:     c.InstanceCIDR,
			},
		}
	}

	return c, nil
}

// Part of configuration which is shared between controller nodes and worker nodes.
// Its name is prefixed with `Kube` because it doesn't relate to etcd.
type KubeClusterSettings struct {
	// Required by kubelet to locate the kube-apiserver
	ExternalDNSName string `yaml:"externalDNSName,omitempty"`
	// Required by kubelet to locate the cluster-internal dns hosted on controller nodes in the base cluster
	DNSServiceIP string `yaml:"dnsServiceIP,omitempty"`
	UseCalico    bool   `yaml:"useCalico,omitempty"`
}

// Part of configuration which can't be provided via user input but is computed from user input
type ComputedDeploymentSettings struct {
	AMI           string
	IsChinaRegion bool
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
	ClusterName      string `yaml:"clusterName,omitempty"`
	KeyName          string `yaml:"keyName,omitempty"`
	Region           string `yaml:"region,omitempty"`
	AvailabilityZone string `yaml:"availabilityZone,omitempty"`
	ReleaseChannel   string `yaml:"releaseChannel,omitempty"`
	AmiId            string `yaml:"amiId,omitempty"`
	VPCID            string `yaml:"vpcId,omitempty"`
	RouteTableID     string `yaml:"routeTableId,omitempty"`
	// Required for validations like e.g. if instance cidr is contained in vpc cidr
	VPCCIDR             string            `yaml:"vpcCIDR,omitempty"`
	InstanceCIDR        string            `yaml:"instanceCIDR,omitempty"`
	K8sVer              string            `yaml:"kubernetesVersion,omitempty"`
	HyperkubeImageRepo  string            `yaml:"hyperkubeImageRepo,omitempty"`
	AWSCliImageRepo     string            `yaml:"awsCliImageRepo,omitempty"`
	AWSCliTag           string            `yaml:"awsCliTag,omitempty"`
	ContainerRuntime    string            `yaml:"containerRuntime,omitempty"`
	KMSKeyARN           string            `yaml:"kmsKeyArn,omitempty"`
	StackTags           map[string]string `yaml:"stackTags,omitempty"`
	Subnets             []*Subnet         `yaml:"subnets,omitempty"`
	MapPublicIPs        bool              `yaml:"mapPublicIPs,omitempty"`
	ElasticFileSystemID string            `yaml:"elasticFileSystemId,omitempty"`
	SSHAuthorizedKeys   []string          `yaml:"sshAuthorizedKeys,omitempty"`
	Experimental        Experimental      `yaml:"experimental"`
}

// Part of configuration which is specific to worker nodes
type WorkerSettings struct {
	model.Worker           `yaml:"worker,omitempty"`
	WorkerCount            int      `yaml:"workerCount,omitempty"`
	WorkerCreateTimeout    string   `yaml:"workerCreateTimeout,omitempty"`
	WorkerInstanceType     string   `yaml:"workerInstanceType,omitempty"`
	WorkerRootVolumeType   string   `yaml:"workerRootVolumeType,omitempty"`
	WorkerRootVolumeIOPS   int      `yaml:"workerRootVolumeIOPS,omitempty"`
	WorkerRootVolumeSize   int      `yaml:"workerRootVolumeSize,omitempty"`
	WorkerSpotPrice        string   `yaml:"workerSpotPrice,omitempty"`
	WorkerSecurityGroupIds []string `yaml:"workerSecurityGroupIds,omitempty"`
}

// Part of configuration which is specific to controller nodes
type ControllerSettings struct {
	model.Controller         `yaml:"controller,omitempty"`
	ControllerCount          int    `yaml:"controllerCount,omitempty"`
	ControllerCreateTimeout  string `yaml:"controllerCreateTimeout,omitempty"`
	ControllerInstanceType   string `yaml:"controllerInstanceType,omitempty"`
	ControllerRootVolumeType string `yaml:"controllerRootVolumeType,omitempty"`
	ControllerRootVolumeIOPS int    `yaml:"controllerRootVolumeIOPS,omitempty"`
	ControllerRootVolumeSize int    `yaml:"controllerRootVolumeSize,omitempty"`
}

// Part of configuration which is specific to etcd nodes
type EtcdSettings struct {
	EtcdCount               int    `yaml:"etcdCount"`
	EtcdInstanceType        string `yaml:"etcdInstanceType,omitempty"`
	EtcdRootVolumeSize      int    `yaml:"etcdRootVolumeSize,omitempty"`
	EtcdRootVolumeType      string `yaml:"etcdRootVolumeType,omitempty"`
	EtcdRootVolumeIOPS      int    `yaml:"etcdRootVolumeIOPS,omitempty"`
	EtcdDataVolumeSize      int    `yaml:"etcdDataVolumeSize,omitempty"`
	EtcdDataVolumeType      string `yaml:"etcdDataVolumeType,omitempty"`
	EtcdDataVolumeIOPS      int    `yaml:"etcdDataVolumeIOPS,omitempty"`
	EtcdDataVolumeEphemeral bool   `yaml:"etcdDataVolumEphemeral,omitempty"`
}

// Part of configuration which is specific to flanneld
type FlannelSettings struct {
	PodCIDR string `yaml:"podCIDR,omitempty"`
}

type Cluster struct {
	KubeClusterSettings    `yaml:",inline"`
	DeploymentSettings     `yaml:",inline"`
	WorkerSettings         `yaml:",inline"`
	ControllerSettings     `yaml:",inline"`
	EtcdSettings           `yaml:",inline"`
	FlannelSettings        `yaml:",inline"`
	ServiceCIDR            string `yaml:"serviceCIDR,omitempty"`
	CreateRecordSet        bool   `yaml:"createRecordSet,omitempty"`
	RecordSetTTL           int    `yaml:"recordSetTTL,omitempty"`
	TLSCADurationDays      int    `yaml:"tlsCADurationDays,omitempty"`
	TLSCertDurationDays    int    `yaml:"tlsCertDurationDays,omitempty"`
	HostedZone             string `yaml:"hostedZone,omitempty"`
	HostedZoneID           string `yaml:"hostedZoneId,omitempty"`
	providedEncryptService EncryptService
}

type Subnet struct {
	AvailabilityZone  string `yaml:"availabilityZone,omitempty"`
	InstanceCIDR      string `yaml:"instanceCIDR,omitempty"`
	lastAllocatedAddr *net.IP
}

type Experimental struct {
	AuditLog              AuditLog              `yaml:"auditLog"`
	AwsEnvironment        AwsEnvironment        `yaml:"awsEnvironment"`
	EphemeralImageStorage EphemeralImageStorage `yaml:"ephemeralImageStorage"`
	LoadBalancer          LoadBalancer          `yaml:"loadBalancer"`
	NodeDrainer           NodeDrainer           `yaml:"nodeDrainer"`
	NodeLabel             NodeLabel             `yaml:"nodeLabel"`
	Plugins               Plugins               `yaml:"plugins"`
	Taints                []Taint               `yaml:"taints"`
	WaitSignal            WaitSignal            `yaml:"waitSignal"`
}

type AwsEnvironment struct {
	Enabled     bool              `yaml:"enabled"`
	Environment map[string]string `yaml:"environment"`
}

type AuditLog struct {
	Enabled bool   `yaml:"enabled"`
	MaxAge  int    `yaml:"maxage"`
	LogPath string `yaml:"logpath"`
}

type EphemeralImageStorage struct {
	Enabled    bool   `yaml:"enabled"`
	Disk       string `yaml:"disk"`
	Filesystem string `yaml:"filesystem"`
}

type NodeDrainer struct {
	Enabled bool `yaml:"enabled"`
}

type NodeLabel struct {
	Enabled bool `yaml:"enabled"`
}

type LoadBalancer struct {
	Enabled          bool     `yaml:"enabled"`
	Names            []string `yaml:"names"`
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
	Enabled      bool `yaml:"enabled"`
	MaxBatchSize int  `yaml:"maxBatchSize"`
}

const (
	vpcLogicalName = "VPC"
)

var supportedReleaseChannels = map[string]bool{
	"alpha":  true,
	"beta":   true,
	"stable": true,
}

func (c WorkerSettings) MinWorkerCount() int {
	if c.Worker.AutoScalingGroup.MinSize == 0 {
		return c.WorkerCount
	}
	return c.Worker.AutoScalingGroup.MinSize
}

func (c WorkerSettings) MaxWorkerCount() int {
	if c.Worker.AutoScalingGroup.MaxSize == 0 {
		return c.WorkerCount
	}
	return c.Worker.AutoScalingGroup.MaxSize
}

func (c WorkerSettings) WorkerRollingUpdateMinInstancesInService() int {
	if c.AutoScalingGroup.RollingUpdateMinInstancesInService == 0 {
		return c.MaxWorkerCount() - 1
	}
	return c.AutoScalingGroup.RollingUpdateMinInstancesInService
}

func (c ControllerSettings) MinControllerCount() int {
	if c.Controller.AutoScalingGroup.MinSize == 0 {
		return c.ControllerCount
	}
	return c.Controller.AutoScalingGroup.MinSize
}

func (c ControllerSettings) MaxControllerCount() int {
	if c.Controller.AutoScalingGroup.MaxSize == 0 {
		return c.ControllerCount
	}
	return c.Controller.AutoScalingGroup.MaxSize
}

func (c ControllerSettings) ControllerRollingUpdateMinInstancesInService() int {
	if c.AutoScalingGroup.RollingUpdateMinInstancesInService == 0 {
		return c.MaxControllerCount() - 1
	}
	return c.AutoScalingGroup.RollingUpdateMinInstancesInService
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
		if config.AMI, err = amiregistry.GetAMI(config.Region, config.ReleaseChannel); err != nil {
			return nil, fmt.Errorf("failed getting AMI for config: %v", err)
		}
	} else {
		config.AMI = c.AmiId
	}

	//Set logical name constants
	config.VPCLogicalName = vpcLogicalName

	//Set reference strings

	//Assume VPC does not exist, reference by logical name
	config.VPCRef = fmt.Sprintf(`{ "Ref" : %q }`, config.VPCLogicalName)
	if config.VPCID != "" {
		//This means this VPC already exists, and we can reference it directly by ID
		config.VPCRef = fmt.Sprintf("%q", config.VPCID)
	}

	config.EtcdInstances = make([]etcdInstance, config.EtcdCount)
	var etcdEndpoints, etcdInitialCluster bytes.Buffer

	// Reset lastAllocatedAddr or we'll end up returning different cluster config w/ inconsistent static private ips
	// for each time we call this function `cluster.Config()`
	for _, subnet := range config.Subnets {
		subnet.lastAllocatedAddr = nil
	}

	for etcdIndex := 0; etcdIndex < config.EtcdCount; etcdIndex++ {

		//Round-robbin etcd instances across all available subnets
		subnetIndex := etcdIndex % len(config.Subnets)
		subnet := config.Subnets[subnetIndex]

		_, subnetCIDR, err := net.ParseCIDR(subnet.InstanceCIDR)
		if err != nil {
			return nil, fmt.Errorf("error parsing subnet instance cidr %s: %v", subnet.InstanceCIDR, err)
		}

		if subnet.lastAllocatedAddr == nil {
			ip := subnetCIDR.IP
			//TODO:(chom) this is sloppy, but "soon-ish" etcd with be self-hosted so we'll leave this be
			for i := 0; i < 3; i++ {
				ip = netutil.IncrementIP(ip)
			}
			subnet.lastAllocatedAddr = &ip
		}

		nextAddr := netutil.IncrementIP(*subnet.lastAllocatedAddr)
		subnet.lastAllocatedAddr = &nextAddr
		instance := etcdInstance{
			IPAddress:   *subnet.lastAllocatedAddr,
			SubnetIndex: subnetIndex,
		}

		//TODO(chom): validate we're not overflowing the address space
		//This is complicated, must also factor in DHCP addresses
		//for ASG components

		//Punt on this- we're going to have an answer for dynamic etcd clusters at some point. Then we can either throw
		//the instances in an ASG and use DHCP like all other instances, or simply self-host on cluster

		config.EtcdInstances[etcdIndex] = instance

		//TODO: ipv6 support
		if len(instance.IPAddress) != 4 {
			return nil, fmt.Errorf("Non ipv4 address for etcd node: %v", instance.IPAddress)
		}

		//http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-instance-addressing.html#concepts-private-addresses

		var dnsSuffix string
		if config.Region == "us-east-1" {
			// a special DNS suffix for the original AWS region!
			dnsSuffix = "ec2.internal"
		} else {
			dnsSuffix = fmt.Sprintf("%s.compute.internal", config.Region)
		}

		hostname := fmt.Sprintf("ip-%d-%d-%d-%d.%s",
			instance.IPAddress[0],
			instance.IPAddress[1],
			instance.IPAddress[2],
			instance.IPAddress[3],
			dnsSuffix,
		)

		fmt.Fprintf(&etcdEndpoints, "https://%s:2379", hostname)
		fmt.Fprintf(&etcdInitialCluster, "%s=https://%s:2380", hostname, hostname)
		if etcdIndex < config.EtcdCount-1 {
			fmt.Fprintf(&etcdEndpoints, ",")
			fmt.Fprintf(&etcdInitialCluster, ",")
		}
	}
	config.EtcdEndpoints = etcdEndpoints.String()
	config.EtcdInitialCluster = etcdInitialCluster.String()

	config.IsChinaRegion = strings.HasPrefix(config.Region, "cn")

	return &config, nil
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
	TLSAssetsDir          string
	ControllerTmplFile    string
	WorkerTmplFile        string
	EtcdTmplFile          string
	StackTemplateTmplFile string
}

type stackConfig struct {
	*Config
	UserDataWorker        string
	UserDataController    string
	UserDataEtcd          string
	ControllerSubnetIndex int
}

func (c Cluster) stackConfig(opts StackTemplateOptions, compressUserData bool) (*stackConfig, error) {
	var err error
	stackConfig := stackConfig{}

	if stackConfig.Config, err = c.Config(); err != nil {
		return nil, err
	}

	compactAssets, err := ReadOrCreateCompactTLSAssets(opts.TLSAssetsDir, KMSConfig{
		Region:         stackConfig.Config.Region,
		KMSKeyARN:      c.KMSKeyARN,
		EncryptService: c.providedEncryptService,
	})

	stackConfig.Config.TLSConfig = compactAssets

	if stackConfig.UserDataWorker, err = userdatatemplate.GetString(opts.WorkerTmplFile, stackConfig.Config, compressUserData); err != nil {
		return nil, fmt.Errorf("failed to render worker cloud config: %v", err)
	}
	if stackConfig.UserDataController, err = userdatatemplate.GetString(opts.ControllerTmplFile, stackConfig.Config, compressUserData); err != nil {
		return nil, fmt.Errorf("failed to render controller cloud config: %v", err)
	}
	if stackConfig.UserDataEtcd, err = userdatatemplate.GetString(opts.EtcdTmplFile, stackConfig.Config, compressUserData); err != nil {
		return nil, fmt.Errorf("failed to render etcd cloud config: %v", err)
	}

	return &stackConfig, nil
}

func (c Cluster) ValidateUserData(opts StackTemplateOptions) error {
	stackConfig, err := c.stackConfig(opts, false)
	if err != nil {
		return err
	}

	err = userdatavalidation.Execute([]userdatavalidation.Entry{
		{"UserDataWorker", stackConfig.UserDataWorker},
		{"UserDataController", stackConfig.UserDataController},
		{"UserDataEtcd", stackConfig.UserDataEtcd},
	})

	return err
}

func (c Cluster) RenderStackTemplate(opts StackTemplateOptions) ([]byte, error) {
	stackConfig, err := c.stackConfig(opts, true)
	if err != nil {
		return nil, err
	}

	bytes, err := jsontemplate.GetBytes(opts.StackTemplateTmplFile, stackConfig)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

type etcdInstance struct {
	IPAddress   net.IP
	SubnetIndex int
}

type Config struct {
	Cluster

	EtcdEndpoints      string
	EtcdInitialCluster string
	EtcdInstances      []etcdInstance

	// Encoded TLS assets
	TLSConfig *CompactTLSAssets

	//Logical names of dynamic resources
	VPCLogicalName string

	//Reference strings for dynamic resources
	VPCRef string
}

func (c Cluster) valid() error {
	if c.CreateRecordSet {
		if c.HostedZone == "" && c.HostedZoneID == "" {
			return errors.New("hostedZone or hostedZoneID must be specified createRecordSet is true")
		}
		if c.HostedZone != "" && c.HostedZoneID != "" {
			return errors.New("hostedZone and hostedZoneID cannot both be specified")
		}

		if c.HostedZone != "" {
			fmt.Printf("Warning: the 'hostedZone' parameter is deprecated. Use 'hostedZoneId' instead\n")
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

	if err := c.WorkerSettings.Valid(); err != nil {
		return err
	}

	if err := c.WorkerDeploymentSettings().Valid(); err != nil {
		return err
	}

	return nil
}

type InfrastructureValidationResult struct {
	dnsServiceIPAddr net.IP
}

func (c KubeClusterSettings) Valid() (*InfrastructureValidationResult, error) {
	if c.ExternalDNSName == "" {
		return nil, errors.New("externalDNSName must be set")
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

	if c.KeyName == "" {
		return nil, errors.New("keyName must be set")
	}
	if c.ClusterName == "" {
		return nil, errors.New("clusterName must be set")
	}
	if c.KMSKeyARN == "" {
		return nil, errors.New("kmsKeyArn must be set")
	}

	if c.VPCID == "" && c.RouteTableID != "" {
		return nil, errors.New("vpcId must be specified if routeTableId is specified")
	}

	if c.Region == "" {
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
		for i, subnet := range c.Subnets {
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

	return &DeploymentValidationResult{vpcNet: vpcNet}, nil
}

func (c WorkerSettings) Valid() error {
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

	if c.WorkerCount < 0 {
		return fmt.Errorf("`workerCount` must be zero or greater if specified")
	}
	// one is the default WorkerCount
	if c.WorkerCount != 1 && (c.AutoScalingGroup.MinSize != 0 || c.AutoScalingGroup.MaxSize != 0) {
		return fmt.Errorf("`worker.autoScalingGroup.minSize` and `worker.autoScalingGroup.maxSize` can only be specified without `workerCount`")
	}
	if err := c.AutoScalingGroup.Valid(); err != nil {
		return err
	}

	return nil
}

func (c ControllerSettings) Valid() error {
	if c.ControllerRootVolumeType == "io1" {
		if c.ControllerRootVolumeIOPS < 100 || c.ControllerRootVolumeIOPS > 2000 {
			return fmt.Errorf("invalid controllerRootVolumeIOPS: %d", c.ControllerRootVolumeIOPS)
		}
	} else {
		if c.ControllerRootVolumeIOPS != 0 {
			return fmt.Errorf("invalid controllerRootVolumeIOPS for volume type '%s': %d", c.ControllerRootVolumeType, c.ControllerRootVolumeIOPS)
		}

		if c.ControllerRootVolumeType != "standard" && c.ControllerRootVolumeType != "gp2" {
			return fmt.Errorf("invalid controllerRootVolumeType: %s", c.ControllerRootVolumeType)
		}
	}

	if c.ControllerCount < 0 {
		return fmt.Errorf("`controllerCount` must be zero or greater if specified")
	}
	// one is the default ControllerCount
	if c.ControllerCount != 1 && (c.AutoScalingGroup.MinSize != 0 || c.AutoScalingGroup.MaxSize != 0) {
		return fmt.Errorf("`controller.autoScalingGroup.minSize` and `controller.autoScalingGroup.maxSize` can only be specified without `controllerCount`")
	}
	if err := c.AutoScalingGroup.Valid(); err != nil {
		return err
	}

	return nil
}

func (c Experimental) Valid() error {
	for _, taint := range c.Taints {
		if taint.Effect != "NoSchedule" && taint.Effect != "PreferNoSchedule" {
			return fmt.Errorf("Effect must be NoSchdule or PreferNoSchedule, but was %s", taint.Effect)
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

	azs := make([]string, len(c.Subnets))
	for i := range azs {
		azs[i] = c.Subnets[i].AvailabilityZone
	}

	return azs
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

	return nil
}

func (c *Cluster) WorkerDeploymentSettings() WorkerDeploymentSettings {
	return WorkerDeploymentSettings{
		WorkerSettings:     c.WorkerSettings,
		DeploymentSettings: c.DeploymentSettings,
	}
}

type WorkerDeploymentSettings struct {
	WorkerSettings
	DeploymentSettings
}

func (c *Cluster) WorkerSecurityGroupRefs() []string {
	return c.WorkerDeploymentSettings().WorkerSecurityGroupRefs()
}

func (c WorkerDeploymentSettings) WorkerSecurityGroupRefs() []string {
	refs := []string{}

	if c.Experimental.LoadBalancer.Enabled {
		for _, sgId := range c.Experimental.LoadBalancer.SecurityGroupIds {
			refs = append(refs, fmt.Sprintf(`"%s"`, sgId))
		}
	}

	for _, sgId := range c.WorkerSecurityGroupIds {
		refs = append(refs, fmt.Sprintf(`"%s"`, sgId))
	}

	return refs
}

func (c WorkerDeploymentSettings) Valid() error {
	sgRefs := c.WorkerSecurityGroupRefs()
	numSGs := len(sgRefs)

	if numSGs > 4 {
		return fmt.Errorf("number of user provided security groups must be less than or equal to 4 but was %d (actual EC2 limit is 5 but one of them is reserved for kube-aws) : %v", numSGs, sgRefs)
	}

	return nil
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
