package api

import (
	"errors"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/kubernetes-incubator/kube-aws/cfnresource"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/naming"
	"github.com/kubernetes-incubator/kube-aws/netutil"
	"net"
	"regexp"
	"sort"
	"strings"
)

const (
	k8sVer = "v1.11.3"

	// Experimental SelfHosting feature default images.
	kubeNetworkingSelfHostingDefaultCalicoNodeImageTag = "v3.2.3"
	kubeNetworkingSelfHostingDefaultCalicoCniImageTag  = "v3.2.3"
	kubeNetworkingSelfHostingDefaultFlannelImageTag    = "v0.10.0"
	kubeNetworkingSelfHostingDefaultFlannelCniImageTag = "v0.3.0"
	kubeNetworkingSelfHostingDefaultTyphaImageTag      = "v3.2.3"
)

func NewDefaultCluster() *Cluster {
	kubelet := Kubelet{
		RotateCerts: RotateCerts{
			Enabled: false,
		},
		SystemReservedResources: "",
		KubeReservedResources:   "",
	}
	experimental := Experimental{
		Admission: Admission{
			PodSecurityPolicy{
				Enabled: false,
			},
			AlwaysPullImages{
				Enabled: false,
			},
			DenyEscalatingExec{
				Enabled: false,
			},
			Initializers{
				Enabled: false,
			},
			Priority{
				Enabled: false,
			},
			MutatingAdmissionWebhook{
				Enabled: false,
			},
			ValidatingAdmissionWebhook{
				Enabled: false,
			},
			OwnerReferencesPermissionEnforcement{
				Enabled: false,
			},
			PersistentVolumeClaimResize{
				Enabled: false,
			},
		},
		AuditLog: AuditLog{
			Enabled:   false,
			LogPath:   "/var/log/kube-apiserver-audit.log",
			MaxAge:    30,
			MaxBackup: 1,
			MaxSize:   100,
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
			Enabled: true,
			Options: map[string]string{},
		},
		TLSBootstrap: TLSBootstrap{
			Enabled: false,
		},
		NodeAuthorizer: NodeAuthorizer{
			Enabled: false,
		},
		EphemeralImageStorage: EphemeralImageStorage{
			Enabled:    false,
			Disk:       "xvdb",
			Filesystem: "xfs",
		},
		KIAMSupport: KIAMSupport{
			Enabled:         false,
			Image:           Image{Repo: "quay.io/uswitch/kiam", Tag: "v2.8", RktPullDocker: false},
			SessionDuration: "15m",
			ServerAddresses: KIAMServerAddresses{ServerAddress: "localhost:443", AgentAddress: "kiam-server:443"},
		},
		Kube2IamSupport: Kube2IamSupport{
			Enabled: false,
		},
		GpuSupport: GpuSupport{
			Enabled:      false,
			Version:      "",
			InstallImage: "shelmangroup/coreos-nvidia-driver-installer:latest",
		},
		KubeletOpts: "",
		LoadBalancer: LoadBalancer{
			Enabled: false,
		},
		TargetGroup: TargetGroup{
			Enabled: false,
		},
		NodeDrainer: NodeDrainer{
			Enabled:      false,
			DrainTimeout: 5,
			IAMRole:      IAMRole{},
		},
		Oidc: Oidc{
			Enabled:       false,
			IssuerUrl:     "https://accounts.google.com",
			ClientId:      "kubernetes",
			UsernameClaim: "email",
			GroupsClaim:   "groups",
		},
	}

	ipvsMode := IPVSMode{
		Enabled:       false,
		Scheduler:     "rr",
		SyncPeriod:    "60s",
		MinSyncPeriod: "10s",
	}

	return &Cluster{
		DeploymentSettings: DeploymentSettings{
			ClusterName:        "kubernetes",
			VPCCIDR:            "10.0.0.0/16",
			ReleaseChannel:     "stable",
			KubeAWSVersion:     "UNKNOWN",
			K8sVer:             k8sVer,
			ContainerRuntime:   "docker",
			Subnets:            []Subnet{},
			EIPAllocationIDs:   []string{},
			Experimental:       experimental,
			Kubelet:            kubelet,
			ManageCertificates: true,
			AmazonSsmAgent: AmazonSsmAgent{
				Enabled:     false,
				DownloadUrl: "",
				Sha1Sum:     "",
			},
			CloudWatchLogging: CloudWatchLogging{
				Enabled:         false,
				RetentionInDays: 7,
				LocalStreaming: LocalStreaming{
					Enabled:  true,
					Filter:   `{ $.priority = "CRIT" || $.priority = "WARNING" && $.transport = "journal" && $.systemdUnit = "init.scope" }`,
					Interval: 60,
				},
			},
			HostOS: HostOS{
				BashPrompt: NewDefaultBashPrompt(),
				MOTDBanner: NewDefaultMOTDBanner(),
			},
			KubeProxy: KubeProxy{
				IPVSMode: ipvsMode,
			},
			KubeDns: KubeDns{
				Provider:            "kube-dns",
				NodeLocalResolver:   false,
				DeployToControllers: false,
				Autoscaler: KubeDnsAutoscaler{
					CoresPerReplica: 256,
					NodesPerReplica: 16,
					Min:             2,
				},
			},
			KubeSystemNamespaceLabels: make(map[string]string),
			KubernetesDashboard: KubernetesDashboard{
				AdminPrivileges: true,
				InsecureLogin:   false,
				Enabled:         true,
			},
			Kubernetes: Kubernetes{
				Authentication: KubernetesAuthentication{
					AWSIAM: AWSIAM{
						Enabled:           false,
						BinaryDownloadURL: `https://github.com/kubernetes-sigs/aws-iam-authenticator/releases/download/v0.3.0/heptio-authenticator-aws_0.3.0_linux_amd64`,
					},
				},
				EncryptionAtRest: EncryptionAtRest{
					Enabled: false,
				},
				Networking: Networking{
					AmazonVPC: AmazonVPC{
						Enabled: false,
					},
					SelfHosting: SelfHosting{
						Type:            "canal",
						Typha:           false,
						CalicoNodeImage: Image{Repo: "quay.io/calico/node", Tag: kubeNetworkingSelfHostingDefaultCalicoNodeImageTag, RktPullDocker: false},
						CalicoCniImage:  Image{Repo: "quay.io/calico/cni", Tag: kubeNetworkingSelfHostingDefaultCalicoCniImageTag, RktPullDocker: false},
						FlannelImage:    Image{Repo: "quay.io/coreos/flannel", Tag: kubeNetworkingSelfHostingDefaultFlannelImageTag, RktPullDocker: false},
						FlannelCniImage: Image{Repo: "quay.io/coreos/flannel-cni", Tag: kubeNetworkingSelfHostingDefaultFlannelCniImageTag, RktPullDocker: false},
						TyphaImage:      Image{Repo: "quay.io/calico/typha", Tag: kubeNetworkingSelfHostingDefaultTyphaImageTag, RktPullDocker: false},
					},
				},
			},
			CloudFormationStreaming:            true,
			HyperkubeImage:                     Image{Repo: "k8s.gcr.io/hyperkube-amd64", Tag: k8sVer, RktPullDocker: true},
			AWSCliImage:                        Image{Repo: "quay.io/coreos/awscli", Tag: "master", RktPullDocker: false},
			ClusterAutoscalerImage:             Image{Repo: "k8s.gcr.io/cluster-autoscaler", Tag: "v1.1.0", RktPullDocker: false},
			ClusterProportionalAutoscalerImage: Image{Repo: "k8s.gcr.io/cluster-proportional-autoscaler-amd64", Tag: "1.1.2", RktPullDocker: false},
			CoreDnsImage:                       Image{Repo: "coredns/coredns", Tag: "1.1.3", RktPullDocker: false},
			Kube2IAMImage:                      Image{Repo: "jtblin/kube2iam", Tag: "0.9.0", RktPullDocker: false},
			KubeDnsImage:                       Image{Repo: "k8s.gcr.io/k8s-dns-kube-dns-amd64", Tag: "1.14.7", RktPullDocker: false},
			KubeDnsMasqImage:                   Image{Repo: "k8s.gcr.io/k8s-dns-dnsmasq-nanny-amd64", Tag: "1.14.7", RktPullDocker: false},
			KubeReschedulerImage:               Image{Repo: "k8s.gcr.io/rescheduler-amd64", Tag: "v0.3.2", RktPullDocker: false},
			DnsMasqMetricsImage:                Image{Repo: "k8s.gcr.io/k8s-dns-sidecar-amd64", Tag: "1.14.7", RktPullDocker: false},
			ExecHealthzImage:                   Image{Repo: "k8s.gcr.io/exechealthz-amd64", Tag: "1.2", RktPullDocker: false},
			HelmImage:                          Image{Repo: "quay.io/kube-aws/helm", Tag: "v2.6.0", RktPullDocker: false},
			TillerImage:                        Image{Repo: "gcr.io/kubernetes-helm/tiller", Tag: "v2.7.2", RktPullDocker: false},
			HeapsterImage:                      Image{Repo: "k8s.gcr.io/heapster", Tag: "v1.5.0", RktPullDocker: false},
			MetricsServerImage:                 Image{Repo: "k8s.gcr.io/metrics-server-amd64", Tag: "v0.2.1", RktPullDocker: false},
			AddonResizerImage:                  Image{Repo: "k8s.gcr.io/addon-resizer", Tag: "1.8.1", RktPullDocker: false},
			KubernetesDashboardImage:           Image{Repo: "k8s.gcr.io/kubernetes-dashboard-amd64", Tag: "v1.8.3", RktPullDocker: false},
			PauseImage:                         Image{Repo: "k8s.gcr.io/pause-amd64", Tag: "3.1", RktPullDocker: false},
			JournaldCloudWatchLogsImage:        Image{Repo: "jollinshead/journald-cloudwatch-logs", Tag: "0.1", RktPullDocker: true},
		},
		KubeClusterSettings: KubeClusterSettings{
			PodCIDR:      "10.2.0.0/16",
			DNSServiceIP: "10.3.0.10",
			ServiceCIDR:  "10.3.0.0/24",
		},
		DefaultWorkerSettings: DefaultWorkerSettings{
			WorkerCreateTimeout:    "PT15M",
			WorkerInstanceType:     "t2.medium",
			WorkerRootVolumeType:   "gp2",
			WorkerRootVolumeIOPS:   0,
			WorkerRootVolumeSize:   30,
			WorkerSecurityGroupIds: []string{},
			WorkerTenancy:          "default",
		},
		Controller: NewDefaultController(),
		EtcdSettings: EtcdSettings{
			Etcd: NewDefaultEtcd(),
		},
		// for base cloudformation stack
		TLSCADurationDays:           365 * 10,
		TLSCertDurationDays:         365,
		RecordSetTTL:                300,
		SSHAccessAllowedSourceCIDRs: DefaultCIDRRanges(),
		CustomSettings:              make(map[string]interface{}),
		KubeResourcesAutosave: KubeResourcesAutosave{
			Enabled: false,
		},
	}
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

func (c Cluster) ControlPlaneStackName() string {
	if c.CloudFormation.StackNameOverrides.ControlPlane != "" {
		return c.CloudFormation.StackNameOverrides.ControlPlane
	}
	return controlPlaneStackName
}

func (c *Cluster) Load() error {
	cpStackName := c.ControlPlaneStackName()

	// If the user specified no subnets, we assume that a single AZ configuration with the default instanceCIDR is demanded
	if len(c.Subnets) == 0 && c.InstanceCIDR == "" {
		c.InstanceCIDR = "10.0.0.0/24"
	}

	c.HostedZoneID = withHostedZoneIDPrefix(c.HostedZoneID)

	c.ConsumeDeprecatedKeys()

	if err := c.validate(cpStackName); err != nil {
		return fmt.Errorf("invalid cluster: %v", err)
	}

	if err := c.SetDefaults(); err != nil {
		return fmt.Errorf("invalid cluster: %v", err)
	}

	return nil
}

func (c *Cluster) ConsumeDeprecatedKeys() {
	// TODO Remove in v0.9.9-rc.1
	if c.DeprecatedVPCID != "" {
		logger.Warn("vpcId is deprecated and will be removed in v0.9.9. Please use vpc.id instead")
		c.VPC.ID = c.DeprecatedVPCID
	}

	if c.DeprecatedInternetGatewayID != "" {
		logger.Warn("internetGatewayId is deprecated and will be removed in v0.9.9. Please use internetGateway.id instead")
		c.InternetGateway.ID = c.DeprecatedInternetGatewayID
	}
}

func (c *Cluster) SetDefaults() error {
	// For backward-compatibility
	if len(c.Subnets) == 0 {
		c.Subnets = []Subnet{
			NewPublicSubnet(c.AvailabilityZone, c.InstanceCIDR),
		}
	}

	for i, s := range c.Subnets {
		if s.Name == "" {
			c.Subnets[i].Name = fmt.Sprintf("Subnet%d", i)
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
		c.Controller.Subnets = c.PublicSubnets()

		if len(c.Controller.Subnets) == 0 {
			return errors.New("`controller.subnets` in cluster.yaml defaults to include only public subnets defined under `subnets`. However, there was no public subnet for that. Please define one or more public subnets under `subnets` or set `controller.subnets`.")
		}
	} else if c.Controller.Subnets.ContainsBothPrivateAndPublic() {
		return errors.New("You can not mix private and public subnets for controller nodes. Please explicitly configure controller.subnets[] to contain either public or private subnets only")
	}

	if len(c.Controller.LoadBalancer.Subnets) == 0 {
		if c.Controller.LoadBalancer.Private {
			c.Controller.LoadBalancer.Subnets = c.PrivateSubnets()
			c.Controller.LoadBalancer.Private = true
		} else {
			c.Controller.LoadBalancer.Subnets = c.PublicSubnets()
		}
	}

	if len(c.Etcd.Subnets) == 0 {
		c.Etcd.Subnets = c.PublicSubnets()

		if len(c.Etcd.Subnets) == 0 {
			return errors.New("`etcd.subnets` in cluster.yaml defaults to include only public subnets defined under `subnets`. However, there was no public subnet for that. Please define one or more public subnets under `subnets` or set `etcd.subnets`.")
		}
	} else if c.Etcd.Subnets.ContainsBothPrivateAndPublic() {
		return fmt.Errorf("You can not mix private and public subnets for etcd nodes. Please explicitly configure etcd.subnets[] to contain either public or private subnets only")
	}

	if c.ExternalDNSName != "" {
		// TODO: Deprecate externalDNSName?

		if len(c.APIEndpointConfigs) != 0 {
			return errors.New("invalid cluster: you can only specify either externalDNSName or apiEndpoints, but not both")
		}

		subnetRefs := []SubnetReference{}
		for _, s := range c.Controller.LoadBalancer.Subnets {
			subnetRefs = append(subnetRefs, SubnetReference{Name: s.Name})
		}

		c.APIEndpointConfigs = NewDefaultAPIEndpoints(
			c.ExternalDNSName,
			subnetRefs,
			c.HostedZoneID,
			c.RecordSetTTL,
			c.Controller.LoadBalancer.Private,
		)
	}

	if c.Addons.MetricsServer.Enabled {
		c.Addons.APIServerAggregator.Enabled = true
	}

	return nil
}

// Part of configuration which is shared between controller nodes and worker nodes.
// Its name is prefixed with `Kube` because it doesn't relate to etcd.
type KubeClusterSettings struct {
	APIEndpointConfigs APIEndpoints `yaml:"apiEndpoints,omitempty"`
	// Required by kubelet to locate the kube-apiserver
	ExternalDNSName string `yaml:"externalDNSName,omitempty"`
	// Required by kubelet to locate the cluster-internal dns hosted on controller nodes in the base cluster
	DNSServiceIP string `yaml:"dnsServiceIP,omitempty"`
	PodCIDR      string `yaml:"podCIDR,omitempty"`
	ServiceCIDR  string `yaml:"serviceCIDR,omitempty"`
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
	CloudFormation                        CloudFormation  `yaml:"cloudformation,omitempty"`
	ClusterName                           string          `yaml:"clusterName,omitempty"`
	S3URI                                 string          `yaml:"s3URI,omitempty"`
	DisableContainerLinuxAutomaticUpdates string          `yaml:"disableContainerLinuxAutomaticUpdates,omitempty"`
	KeyName                               string          `yaml:"keyName,omitempty"`
	Region                                Region          `yaml:",inline"`
	AvailabilityZone                      string          `yaml:"availabilityZone,omitempty"`
	ReleaseChannel                        string          `yaml:"releaseChannel,omitempty"`
	AmiId                                 string          `yaml:"amiId,omitempty"`
	DeprecatedVPCID                       string          `yaml:"vpcId,omitempty"`
	VPC                                   VPC             `yaml:"vpc,omitempty"`
	DeprecatedInternetGatewayID           string          `yaml:"internetGatewayId,omitempty"`
	InternetGateway                       InternetGateway `yaml:"internetGateway,omitempty"`
	// Required for validations like e.g. if instance cidr is contained in vpc cidr
	VPCCIDR                   string `yaml:"vpcCIDR,omitempty"`
	InstanceCIDR              string `yaml:"instanceCIDR,omitempty"`
	K8sVer                    string `yaml:"kubernetesVersion,omitempty"`
	KubeAWSVersion            string
	ContainerRuntime          string            `yaml:"containerRuntime,omitempty"`
	KMSKeyARN                 string            `yaml:"kmsKeyArn,omitempty"`
	StackTags                 map[string]string `yaml:"stackTags,omitempty"`
	Subnets                   Subnets           `yaml:"subnets,omitempty"`
	EIPAllocationIDs          []string          `yaml:"eipAllocationIDs,omitempty"`
	ElasticFileSystemID       string            `yaml:"elasticFileSystemId,omitempty"`
	SharedPersistentVolume    bool              `yaml:"sharedPersistentVolume,omitempty"`
	SSHAuthorizedKeys         []string          `yaml:"sshAuthorizedKeys,omitempty"`
	Addons                    Addons            `yaml:"addons"`
	Experimental              Experimental      `yaml:"experimental"`
	Kubelet                   Kubelet           `yaml:"kubelet"`
	ManageCertificates        bool              `yaml:"manageCertificates,omitempty"`
	WaitSignal                WaitSignal        `yaml:"waitSignal"`
	CloudWatchLogging         `yaml:"cloudWatchLogging,omitempty"`
	AmazonSsmAgent            `yaml:"amazonSsmAgent,omitempty"`
	CloudFormationStreaming   bool `yaml:"cloudFormationStreaming,omitempty"`
	KubeProxy                 `yaml:"kubeProxy,omitempty"`
	KubeDns                   `yaml:"kubeDns,omitempty"`
	KubeSystemNamespaceLabels map[string]string `yaml:"kubeSystemNamespaceLabels,omitempty"`
	KubernetesDashboard       `yaml:"kubernetesDashboard,omitempty"`
	// Images repository
	HyperkubeImage                     Image      `yaml:"hyperkubeImage,omitempty"`
	AWSCliImage                        Image      `yaml:"awsCliImage,omitempty"`
	ClusterAutoscalerImage             Image      `yaml:"clusterAutoscalerImage,omitempty"`
	ClusterProportionalAutoscalerImage Image      `yaml:"clusterProportionalAutoscalerImage,omitempty"`
	CoreDnsImage                       Image      `yaml:"coreDnsImage,omitempty"`
	Kube2IAMImage                      Image      `yaml:"kube2iamImage,omitempty"`
	KubeDnsImage                       Image      `yaml:"kubeDnsImage,omitempty"`
	KubeDnsMasqImage                   Image      `yaml:"kubeDnsMasqImage,omitempty"`
	KubeReschedulerImage               Image      `yaml:"kubeReschedulerImage,omitempty"`
	DnsMasqMetricsImage                Image      `yaml:"dnsMasqMetricsImage,omitempty"`
	ExecHealthzImage                   Image      `yaml:"execHealthzImage,omitempty"`
	HelmImage                          Image      `yaml:"helmImage,omitempty"`
	TillerImage                        Image      `yaml:"tillerImage,omitempty"`
	HeapsterImage                      Image      `yaml:"heapsterImage,omitempty"`
	MetricsServerImage                 Image      `yaml:"metricsServerImage,omitempty"`
	AddonResizerImage                  Image      `yaml:"addonResizerImage,omitempty"`
	KubernetesDashboardImage           Image      `yaml:"kubernetesDashboardImage,omitempty"`
	PauseImage                         Image      `yaml:"pauseImage,omitempty"`
	JournaldCloudWatchLogsImage        Image      `yaml:"journaldCloudWatchLogsImage,omitempty"`
	Kubernetes                         Kubernetes `yaml:"kubernetes,omitempty"`
	HostOS                             HostOS     `yaml:"hostOS,omitempty"`
}

// Part of configuration which is specific to worker nodes
type DefaultWorkerSettings struct {
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

// Part of configuration which is specific to etcd nodes
type EtcdSettings struct {
	Etcd `yaml:"etcd,omitempty"`
}

// Cluster is the container of all the configurable parameters of a kube-aws cluster, customizable via cluster.yaml
type Cluster struct {
	KubeClusterSettings   `yaml:",inline"`
	DeploymentSettings    `yaml:",inline"`
	DefaultWorkerSettings `yaml:",inline"`
	Controller            Controller `yaml:"controller,omitempty"`
	EtcdSettings          `yaml:",inline"`
	AdminAPIEndpointName  string `yaml:"adminAPIEndpointName,omitempty"`
	RecordSetTTL          int    `yaml:"recordSetTTL,omitempty"`
	TLSCADurationDays     int    `yaml:"tlsCADurationDays,omitempty"`
	TLSCertDurationDays   int    `yaml:"tlsCertDurationDays,omitempty"`
	HostedZoneID          string `yaml:"hostedZoneId,omitempty"`
	Worker                `yaml:"worker"`
	PluginConfigs         PluginConfigs `yaml:"kubeAwsPlugins,omitempty"`
	// SSHAccessAllowedSourceCIDRs is network ranges of sources you'd like SSH accesses to be allowed from, in CIDR notation
	SSHAccessAllowedSourceCIDRs CIDRRanges             `yaml:"sshAccessAllowedSourceCIDRs,omitempty"`
	CustomSettings              map[string]interface{} `yaml:"customSettings,omitempty"`
	KubeResourcesAutosave       `yaml:"kubeResourcesAutosave,omitempty"`
}

type KubernetesDashboard struct {
	AdminPrivileges  bool             `yaml:"adminPrivileges"`
	InsecureLogin    bool             `yaml:"insecureLogin"`
	Enabled          bool             `yaml:"enabled"`
	ComputeResources ComputeResources `yaml:"resources,omitempty"`
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

var supportedReleaseChannels = map[string]bool{
	"alpha":  true,
	"beta":   true,
	"stable": true,
}

func (c DeploymentSettings) ApiServerLeaseEndpointReconciler() (bool, error) {
	constraint, err := semver.NewConstraint(">= 1.9")
	if err != nil {
		return false, fmt.Errorf("[BUG] .ApiServerLeaseEndpointReconciler min version could not be parsed")
	}
	version, _ := semver.NewVersion(c.K8sVer) // already validated in Validate()
	return constraint.Check(version), nil
}

// Required by kubelet to use the consistent network plugin with the base cluster
func (c KubeClusterSettings) K8sNetworkPlugin() string {
	return "cni"
}

type StackTemplateOptions struct {
	AssetsDir             string
	ControllerTmplFile    string
	EtcdTmplFile          string
	WorkerTmplFile        string
	StackTemplateTmplFile string
	S3URI                 string
	PrettyPrint           bool
	SkipWait              bool
}

type ClusterOptions struct {
	S3URI    string
	SkipWait bool
}

func (c Cluster) StackNameEnvFileName() string {
	return "/etc/environment"
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

// APIAccessAllowedSourceCIDRsForControllerSG returns all the CIDRs of Kubernetes API endpoints that controller nodes must allow access from
func (c Cluster) APIAccessAllowedSourceCIDRsForControllerSG() []string {
	cidrs := []string{}
	seen := map[string]bool{}

	for _, e := range c.APIEndpointConfigs {
		if !e.LoadBalancer.NetworkLoadBalancer() {
			continue
		}

		ranges := e.LoadBalancer.APIAccessAllowedSourceCIDRs
		if len(ranges) > 0 {
			for _, r := range ranges {
				val := r.String()
				if _, ok := seen[val]; !ok {
					cidrs = append(cidrs, val)
					seen[val] = true
				}
			}
		}
	}

	sort.Strings(cidrs)

	return cidrs
}

func (c Cluster) ClusterAutoscalerSupportEnabled() bool {
	return c.Addons.ClusterAutoscaler.Enabled && c.Experimental.ClusterAutoscalerSupport.Enabled
}

func (c Cluster) NodeLabels() NodeLabels {
	labels := c.Controller.NodeLabels
	if c.ClusterAutoscalerSupportEnabled() {
		labels["kube-aws.coreos.com/cluster-autoscaler-supported"] = "true"
	}
	return labels
}

func (c Cluster) validate(cpStackName string) error {
	validClusterNaming := regexp.MustCompile("^[a-zA-Z0-9-:]+$")
	if !validClusterNaming.MatchString(c.ClusterName) {
		return fmt.Errorf("clusterName(=%s) is malformed. It must consist only of alphanumeric characters, colons, or hyphens", c.ClusterName)
	}

	var dnsServiceIPAddr net.IP

	if kubeClusterValidationResult, err := c.KubeClusterSettings.Validate(); err != nil {
		return err
	} else {
		dnsServiceIPAddr = kubeClusterValidationResult.dnsServiceIPAddr
	}

	var vpcNet *net.IPNet

	if deploymentValidationResult, err := c.DeploymentSettings.Validate(); err != nil {
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

	if err := c.Controller.Validate(); err != nil {
		return err
	}

	if err := c.DefaultWorkerSettings.Validate(); err != nil {
		return err
	}

	if err := c.EtcdSettings.Validate(); err != nil {
		return err
	}

	if c.WorkerTenancy != "default" && c.WorkerSpotPrice != "" {
		return fmt.Errorf("selected worker tenancy (%s) is incompatible with spot instances", c.WorkerTenancy)
	}

	clusterNamePlaceholder := "<my-cluster-name>"
	nestedStackNamePlaceHolder := "<my-nested-stack-name>"
	replacer := strings.NewReplacer(clusterNamePlaceholder, "", nestedStackNamePlaceHolder, cpStackName)
	simulatedLcName := fmt.Sprintf("%s-%s-1N2C4K3LLBEDZ-%sLC-BC2S9P3JG2QD", clusterNamePlaceholder, nestedStackNamePlaceHolder, c.Controller.LogicalName())
	limit := 63 - len(replacer.Replace(simulatedLcName))
	if c.Experimental.AwsNodeLabels.Enabled && len(c.ClusterName) > limit {
		return fmt.Errorf("awsNodeLabels can't be enabled for controllers because the total number of characters in clusterName(=\"%s\") exceeds the limit of %d", c.ClusterName, limit)
	}

	if c.Controller.InstanceType == "t2.micro" || c.Etcd.InstanceType == "t2.micro" || c.Controller.InstanceType == "t2.nano" || c.Etcd.InstanceType == "t2.nano" {
		logger.Warn(`instance types "t2.nano" and "t2.micro" are not recommended. See https://github.com/kubernetes-incubator/kube-aws/issues/258 for more information`)
	}

	if len(c.Controller.IAMConfig.Role.Name) > 0 {
		if e := cfnresource.ValidateStableRoleNameLength(c.ClusterName, c.Controller.IAMConfig.Role.Name, c.Region.String()); e != nil {
			return e
		}
	} else {
		if e := cfnresource.ValidateUnstableRoleNameLength(c.ClusterName, naming.FromStackToCfnResource(cpStackName), c.Controller.IAMConfig.Role.Name, c.Region.String()); e != nil {
			return e
		}
	}

	for _, w := range c.Worker.NodePools {
		// Validate whole the inputs
		if err := w.Validate(c.Experimental); err != nil {
			return err
		}
	}

	if c.Experimental.NodeAuthorizer.Enabled {
		if !c.Experimental.TLSBootstrap.Enabled {
			return fmt.Errorf("TLS bootstrap is required in order to enable the node authorizer")
		}
	}

	for i, e := range c.APIEndpointConfigs {
		if e.LoadBalancer.NetworkLoadBalancer() && !c.Region.SupportsNetworkLoadBalancers() {
			return fmt.Errorf("api endpoint %d is not valid: network load balancer not supported in region", i)
		}
	}

	if c.Kubernetes.Networking.SelfHosting.Type != "canal" && c.Kubernetes.Networking.SelfHosting.Type != "flannel" {
		return fmt.Errorf("networkingdaemonsets - style must be either 'canal' or 'flannel'")
	}
	if c.Kubernetes.Networking.SelfHosting.Typha && c.Kubernetes.Networking.SelfHosting.Type != "canal" {
		return fmt.Errorf("networkingdaemonsets - you can only enable typha when deploying type 'canal'")
	}

	return nil
}

type InfrastructureValidationResult struct {
	dnsServiceIPAddr net.IP
}

func (c KubeClusterSettings) Validate() (*InfrastructureValidationResult, error) {
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

func (c DefaultWorkerSettings) Validate() error {
	if c.WorkerRootVolumeType == "io1" {
		if c.WorkerRootVolumeIOPS < 100 || c.WorkerRootVolumeIOPS > 20000 {
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

	return nil
}

// Valid returns an error when there's any user error in the `etcd` settings
func (e EtcdSettings) Validate() error {
	if !e.Etcd.DataVolume.Encrypted && e.Etcd.KMSKeyARN() != "" {
		return errors.New("`etcd.kmsKeyArn` can only be specified when `etcdDataVolumeEncrypted` is enabled")
	}

	if err := e.IAMConfig.Validate(); err != nil {
		return fmt.Errorf("invalid etcd settings: %v", err)
	}

	if err := e.Etcd.Validate(); err != nil {
		return err
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
