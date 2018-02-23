package config

//go:generate go run ../../../codegen/templates_gen.go CloudConfigController=cloud-config-controller CloudConfigWorker=cloud-config-worker CloudConfigEtcd=cloud-config-etcd DefaultClusterConfig=cluster.yaml KubeConfigTemplate=kubeconfig.tmpl StackTemplateTemplate=stack-template.json
//go:generate gofmt -w templates.go
//go:generate go run ../../../codegen/files_gen.go Etcdadm=../../../etcdadm/etcdadm
//go:generate gofmt -w files.go

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/kubernetes-incubator/kube-aws/cfnresource"
	"github.com/kubernetes-incubator/kube-aws/coreos/amiregistry"
	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/model/derived"
	"github.com/kubernetes-incubator/kube-aws/netutil"
	"github.com/kubernetes-incubator/kube-aws/node"
	"github.com/kubernetes-incubator/kube-aws/plugin/pluginmodel"
	yaml "gopkg.in/yaml.v2"
)

const (
	k8sVer = "v1.9.1"

	credentialsDir = "credentials"
	userDataDir    = "userdata"
)

func NewDefaultCluster() *Cluster {
	kubelet := Kubelet{
		RotateCerts: RotateCerts{
			Enabled: false,
		},
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
		},
		AuditLog: AuditLog{
			Enabled: false,
			MaxAge:  30,
			LogPath: "/var/log/kube-apiserver-audit.log",
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
		ClusterAutoscalerSupport: model.ClusterAutoscalerSupport{
			Enabled: true,
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
		Kube2IamSupport: Kube2IamSupport{
			Enabled: false,
		},
		KubeletOpts: "",
		LoadBalancer: LoadBalancer{
			Enabled: false,
		},
		TargetGroup: TargetGroup{
			Enabled: false,
		},
		NodeDrainer: model.NodeDrainer{
			Enabled:      false,
			DrainTimeout: 5,
			IAMRole:      model.IAMRole{},
		},
		Oidc: model.Oidc{
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
			K8sVer:             k8sVer,
			ContainerRuntime:   "docker",
			Subnets:            []model.Subnet{},
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
					interval: 60,
				},
			},
			KubeProxy: KubeProxy{
				IPVSMode: ipvsMode,
			},
			KubeDns: KubeDns{
				NodeLocalResolver:   false,
				DeployToControllers: false,
				Autoscaler: KubeDnsAutoscaler{
					CoresPerReplica: 256,
					NodesPerReplica: 16,
					Min:             2,
				},
			},
			KubernetesDashboard: KubernetesDashboard{
				AdminPrivileges: true,
				InsecureLogin:   false,
			},
			CloudFormationStreaming:            true,
			HyperkubeImage:                     model.Image{Repo: "k8s.gcr.io/hyperkube-amd64", Tag: k8sVer, RktPullDocker: true},
			AWSCliImage:                        model.Image{Repo: "quay.io/coreos/awscli", Tag: "master", RktPullDocker: false},
			CalicoNodeImage:                    model.Image{Repo: "quay.io/calico/node", Tag: "v2.6.5", RktPullDocker: false},
			CalicoCniImage:                     model.Image{Repo: "quay.io/calico/cni", Tag: "v1.11.2", RktPullDocker: false},
			CalicoKubeControllersImage:         model.Image{Repo: "quay.io/calico/kube-controllers", Tag: "v1.0.2", RktPullDocker: false},
			CalicoCtlImage:                     model.Image{Repo: "quay.io/calico/ctl", Tag: "v1.6.3", RktPullDocker: false},
			ClusterAutoscalerImage:             model.Image{Repo: "k8s.gcr.io/cluster-autoscaler", Tag: "v1.1.0", RktPullDocker: false},
			ClusterProportionalAutoscalerImage: model.Image{Repo: "k8s.gcr.io/cluster-proportional-autoscaler-amd64", Tag: "1.1.2", RktPullDocker: false},
			KIAMImage:                          model.Image{Repo: "quay.io/uswitch/kiam", Tag: "v2.6", RktPullDocker: false},
			Kube2IAMImage:                      model.Image{Repo: "jtblin/kube2iam", Tag: "0.9.0", RktPullDocker: false},
			KubeDnsImage:                       model.Image{Repo: "k8s.gcr.io/k8s-dns-kube-dns-amd64", Tag: "1.14.7", RktPullDocker: false},
			KubeDnsMasqImage:                   model.Image{Repo: "k8s.gcr.io/k8s-dns-dnsmasq-nanny-amd64", Tag: "1.14.7", RktPullDocker: false},
			KubeReschedulerImage:               model.Image{Repo: "k8s.gcr.io/rescheduler-amd64", Tag: "v0.3.2", RktPullDocker: false},
			DnsMasqMetricsImage:                model.Image{Repo: "k8s.gcr.io/k8s-dns-sidecar-amd64", Tag: "1.14.7", RktPullDocker: false},
			ExecHealthzImage:                   model.Image{Repo: "k8s.gcr.io/exechealthz-amd64", Tag: "1.2", RktPullDocker: false},
			HelmImage:                          model.Image{Repo: "quay.io/kube-aws/helm", Tag: "v2.6.0", RktPullDocker: false},
			TillerImage:                        model.Image{Repo: "gcr.io/kubernetes-helm/tiller", Tag: "v2.7.2", RktPullDocker: false},
			HeapsterImage:                      model.Image{Repo: "k8s.gcr.io/heapster", Tag: "v1.5.0", RktPullDocker: false},
			MetricsServerImage:                 model.Image{Repo: "k8s.gcr.io/metrics-server-amd64", Tag: "v0.2.1", RktPullDocker: false},
			AddonResizerImage:                  model.Image{Repo: "k8s.gcr.io/addon-resizer", Tag: "1.8.1", RktPullDocker: false},
			KubernetesDashboardImage:           model.Image{Repo: "k8s.gcr.io/kubernetes-dashboard-amd64", Tag: "v1.8.1", RktPullDocker: false},
			PauseImage:                         model.Image{Repo: "k8s.gcr.io/pause-amd64", Tag: "3.1", RktPullDocker: false},
			FlannelImage:                       model.Image{Repo: "quay.io/coreos/flannel", Tag: "v0.9.1", RktPullDocker: false},
			JournaldCloudWatchLogsImage:        model.Image{Repo: "jollinshead/journald-cloudwatch-logs", Tag: "0.1", RktPullDocker: true},
		},
		KubeClusterSettings: KubeClusterSettings{
			DNSServiceIP: "10.3.0.10",
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
		TLSCADurationDays:           365 * 10,
		TLSCertDurationDays:         365,
		RecordSetTTL:                300,
		SSHAccessAllowedSourceCIDRs: model.DefaultCIDRRanges(),
		CustomSettings:              make(map[string]interface{}),
		KubeResourcesAutosave: KubeResourcesAutosave{
			Enabled: false,
		},
	}
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
	cfg, err := c.Config([]*pluginmodel.Plugin{})
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

	if err := c.validate(); err != nil {
		return fmt.Errorf("invalid cluster: %v", err)
	}

	if err := c.SetDefaults(); err != nil {
		return fmt.Errorf("invalid cluster: %v", err)
	}

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
			c.RecordSetTTL,
			c.Controller.LoadBalancer.Private,
		)
	}

	return nil
}

func (c *Cluster) ConsumeDeprecatedKeys() {
	// TODO Remove in v0.9.9-rc.1
	if c.DeprecatedVPCID != "" {
		fmt.Println("WARN: vpcId is deprecated and will be removed in v0.9.9. Please use vpc.id instead")
		c.VPC.ID = c.DeprecatedVPCID
	}

	if c.DeprecatedInternetGatewayID != "" {
		fmt.Println("WARN: internetGatewayId is deprecated and will be removed in v0.9.9. Please use internetGateway.id instead")
		c.InternetGateway.ID = c.DeprecatedInternetGatewayID
	}
}

func (c *Cluster) SetDefaults() error {
	// For backward-compatibility
	if len(c.Subnets) == 0 {
		c.Subnets = []model.Subnet{
			model.NewPublicSubnet(c.AvailabilityZone, c.InstanceCIDR),
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

	return nil
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
	CloudFormation              model.CloudFormation  `yaml:"cloudformation,omitempty"`
	ClusterName                 string                `yaml:"clusterName,omitempty"`
	KeyName                     string                `yaml:"keyName,omitempty"`
	Region                      model.Region          `yaml:",inline"`
	AvailabilityZone            string                `yaml:"availabilityZone,omitempty"`
	ReleaseChannel              string                `yaml:"releaseChannel,omitempty"`
	AmiId                       string                `yaml:"amiId,omitempty"`
	DeprecatedVPCID             string                `yaml:"vpcId,omitempty"`
	VPC                         model.VPC             `yaml:"vpc,omitempty"`
	DeprecatedInternetGatewayID string                `yaml:"internetGatewayId,omitempty"`
	InternetGateway             model.InternetGateway `yaml:"internetGateway,omitempty"`
	// Required for validations like e.g. if instance cidr is contained in vpc cidr
	VPCCIDR                 string            `yaml:"vpcCIDR,omitempty"`
	InstanceCIDR            string            `yaml:"instanceCIDR,omitempty"`
	K8sVer                  string            `yaml:"kubernetesVersion,omitempty"`
	ContainerRuntime        string            `yaml:"containerRuntime,omitempty"`
	KMSKeyARN               string            `yaml:"kmsKeyArn,omitempty"`
	StackTags               map[string]string `yaml:"stackTags,omitempty"`
	Subnets                 model.Subnets     `yaml:"subnets,omitempty"`
	EIPAllocationIDs        []string          `yaml:"eipAllocationIDs,omitempty"`
	ElasticFileSystemID     string            `yaml:"elasticFileSystemId,omitempty"`
	SharedPersistentVolume  bool              `yaml:"sharedPersistentVolume,omitempty"`
	SSHAuthorizedKeys       []string          `yaml:"sshAuthorizedKeys,omitempty"`
	Addons                  model.Addons      `yaml:"addons"`
	Experimental            Experimental      `yaml:"experimental"`
	Kubelet                 Kubelet           `yaml:"kubelet"`
	ManageCertificates      bool              `yaml:"manageCertificates,omitempty"`
	WaitSignal              WaitSignal        `yaml:"waitSignal"`
	CloudWatchLogging       `yaml:"cloudWatchLogging,omitempty"`
	AmazonSsmAgent          `yaml:"amazonSsmAgent,omitempty"`
	CloudFormationStreaming bool `yaml:"cloudFormationStreaming,omitempty"`
	KubeProxy               `yaml:"kubeProxy,omitempty"`
	KubeDns                 `yaml:"kubeDns,omitempty"`
	KubernetesDashboard     `yaml:"kubernetesDashboard,omitempty"`
	// Images repository
	HyperkubeImage                     model.Image `yaml:"hyperkubeImage,omitempty"`
	AWSCliImage                        model.Image `yaml:"awsCliImage,omitempty"`
	CalicoNodeImage                    model.Image `yaml:"calicoNodeImage,omitempty"`
	CalicoCniImage                     model.Image `yaml:"calicoCniImage,omitempty"`
	CalicoCtlImage                     model.Image `yaml:"calicoCtlImage,omitempty"`
	CalicoKubeControllersImage         model.Image `yaml:"calicoKubeControllersImage,omitempty"`
	ClusterAutoscalerImage             model.Image `yaml:"clusterAutoscalerImage,omitempty"`
	ClusterProportionalAutoscalerImage model.Image `yaml:"clusterProportionalAutoscalerImage,omitempty"`
	KIAMImage                          model.Image `yaml:"kiamImage,omitempty"`
	Kube2IAMImage                      model.Image `yaml:"kube2iamImage,omitempty"`
	KubeDnsImage                       model.Image `yaml:"kubeDnsImage,omitempty"`
	KubeDnsMasqImage                   model.Image `yaml:"kubeDnsMasqImage,omitempty"`
	KubeReschedulerImage               model.Image `yaml:"kubeReschedulerImage,omitempty"`
	DnsMasqMetricsImage                model.Image `yaml:"dnsMasqMetricsImage,omitempty"`
	ExecHealthzImage                   model.Image `yaml:"execHealthzImage,omitempty"`
	HelmImage                          model.Image `yaml:"helmImage,omitempty"`
	TillerImage                        model.Image `yaml:"tillerImage,omitempty"`
	HeapsterImage                      model.Image `yaml:"heapsterImage,omitempty"`
	MetricsServerImage                 model.Image `yaml:"metricsServerImage,omitempty"`
	AddonResizerImage                  model.Image `yaml:"addonResizerImage,omitempty"`
	KubernetesDashboardImage           model.Image `yaml:"kubernetesDashboardImage,omitempty"`
	PauseImage                         model.Image `yaml:"pauseImage,omitempty"`
	FlannelImage                       model.Image `yaml:"flannelImage,omitempty"`
	JournaldCloudWatchLogsImage        model.Image `yaml:"journaldCloudWatchLogsImage,omitempty"`
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

// Part of configuration which is specific to controller nodes
type ControllerSettings struct {
	model.Controller `yaml:"controller,omitempty"`
}

// Part of configuration which is specific to etcd nodes
type EtcdSettings struct {
	model.Etcd `yaml:"etcd,omitempty"`
}

// Part of configuration which is specific to flanneld
type FlannelSettings struct {
	PodCIDR string `yaml:"podCIDR,omitempty"`
}

// Cluster is the container of all the configurable parameters of a kube-aws cluster, customizable via cluster.yaml
type Cluster struct {
	KubeClusterSettings    `yaml:",inline"`
	DeploymentSettings     `yaml:",inline"`
	DefaultWorkerSettings  `yaml:",inline"`
	ControllerSettings     `yaml:",inline"`
	EtcdSettings           `yaml:",inline"`
	FlannelSettings        `yaml:",inline"`
	AdminAPIEndpointName   string              `yaml:"adminAPIEndpointName,omitempty"`
	ServiceCIDR            string              `yaml:"serviceCIDR,omitempty"`
	RecordSetTTL           int                 `yaml:"recordSetTTL,omitempty"`
	TLSCADurationDays      int                 `yaml:"tlsCADurationDays,omitempty"`
	TLSCertDurationDays    int                 `yaml:"tlsCertDurationDays,omitempty"`
	HostedZoneID           string              `yaml:"hostedZoneId,omitempty"`
	PluginConfigs          model.PluginConfigs `yaml:"kubeAwsPlugins,omitempty"`
	ProvidedEncryptService EncryptService
	// SSHAccessAllowedSourceCIDRs is network ranges of sources you'd like SSH accesses to be allowed from, in CIDR notation
	SSHAccessAllowedSourceCIDRs model.CIDRRanges       `yaml:"sshAccessAllowedSourceCIDRs,omitempty"`
	CustomSettings              map[string]interface{} `yaml:"customSettings,omitempty"`
	KubeResourcesAutosave       `yaml:"kubeResourcesAutosave,omitempty"`
}

// Kubelet options
type Kubelet struct {
	RotateCerts RotateCerts `yaml:"rotateCerts"`
}

type Experimental struct {
	Admission      Admission      `yaml:"admission"`
	AuditLog       AuditLog       `yaml:"auditLog"`
	Authentication Authentication `yaml:"authentication"`
	AwsEnvironment AwsEnvironment `yaml:"awsEnvironment"`
	AwsNodeLabels  AwsNodeLabels  `yaml:"awsNodeLabels"`
	// When cluster-autoscaler support is enabled, not only controller nodes but this node pool is also given
	// a node label and IAM permissions to run cluster-autoscaler
	ClusterAutoscalerSupport    model.ClusterAutoscalerSupport `yaml:"clusterAutoscalerSupport"`
	TLSBootstrap                TLSBootstrap                   `yaml:"tlsBootstrap"`
	NodeAuthorizer              NodeAuthorizer                 `yaml:"nodeAuthorizer"`
	EphemeralImageStorage       EphemeralImageStorage          `yaml:"ephemeralImageStorage"`
	KIAMSupport                 KIAMSupport                    `yaml:"kiamSupport,omitempty"`
	Kube2IamSupport             Kube2IamSupport                `yaml:"kube2IamSupport,omitempty"`
	KubeletOpts                 string                         `yaml:"kubeletOpts,omitempty"`
	LoadBalancer                LoadBalancer                   `yaml:"loadBalancer"`
	TargetGroup                 TargetGroup                    `yaml:"targetGroup"`
	NodeDrainer                 model.NodeDrainer              `yaml:"nodeDrainer"`
	Oidc                        model.Oidc                     `yaml:"oidc"`
	DisableSecurityGroupIngress bool                           `yaml:"disableSecurityGroupIngress"`
	NodeMonitorGracePeriod      string                         `yaml:"nodeMonitorGracePeriod"`
	model.UnknownKeys           `yaml:",inline"`
}

type Admission struct {
	PodSecurityPolicy          PodSecurityPolicy          `yaml:"podSecurityPolicy"`
	AlwaysPullImages           AlwaysPullImages           `yaml:"alwaysPullImages"`
	DenyEscalatingExec         DenyEscalatingExec         `yaml:"denyEscalatingExec"`
	Initializers               Initializers               `yaml:"initializers"`
	Priority                   Priority                   `yaml:"priority"`
	MutatingAdmissionWebhook   MutatingAdmissionWebhook   `yaml:"mutatingAdmissionWebhook"`
	ValidatingAdmissionWebhook ValidatingAdmissionWebhook `yaml:"validatingAdmissionWebhook"`
}

type AlwaysPullImages struct {
	Enabled bool `yaml:"enabled"`
}

type PodSecurityPolicy struct {
	Enabled bool `yaml:"enabled"`
}

type DenyEscalatingExec struct {
	Enabled bool `yaml:"enabled"`
}

type Initializers struct {
	Enabled bool `yaml:"enabled"`
}

type Priority struct {
	Enabled bool `yaml:"enabled"`
}

type MutatingAdmissionWebhook struct {
	Enabled bool `yaml:"enabled"`
}

type ValidatingAdmissionWebhook struct {
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

type TLSBootstrap struct {
	Enabled bool `yaml:"enabled"`
}

type RotateCerts struct {
	Enabled bool `yaml:"enabled"`
}

type NodeAuthorizer struct {
	Enabled bool `yaml:"enabled"`
}

type EphemeralImageStorage struct {
	Enabled    bool   `yaml:"enabled"`
	Disk       string `yaml:"disk"`
	Filesystem string `yaml:"filesystem"`
}

type KIAMSupport struct {
	Enabled bool `yaml:"enabled"`
}

type Kube2IamSupport struct {
	Enabled bool `yaml:"enabled"`
}

type KubeResourcesAutosave struct {
	Enabled bool `yaml:"enabled"`
	S3Path  string
}

type AmazonSsmAgent struct {
	Enabled     bool   `yaml:"enabled"`
	DownloadUrl string `yaml:"downloadUrl"`
	Sha1Sum     string `yaml:"sha1sum"`
}

type CloudWatchLogging struct {
	Enabled         bool `yaml:"enabled"`
	RetentionInDays int  `yaml:"retentionInDays"`
	LocalStreaming  `yaml:"localStreaming"`
}

type LocalStreaming struct {
	Enabled  bool   `yaml:"enabled"`
	Filter   string `yaml:"filter"`
	interval int    `yaml:"interval"`
}

func (c *LocalStreaming) Interval() int64 {
	// Convert from seconds to milliseconds (and return as int64 type)
	return int64(c.interval * 1000)
}

func (c *CloudWatchLogging) MergeIfEmpty(other CloudWatchLogging) {
	if c.Enabled == false && c.RetentionInDays == 0 {
		c.Enabled = other.Enabled
		c.RetentionInDays = other.RetentionInDays
	}
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

type KubeProxy struct {
	IPVSMode IPVSMode `yaml:"ipvsMode"`
}

type IPVSMode struct {
	Enabled       bool   `yaml:"enabled"`
	Scheduler     string `yaml:"scheduler"`
	SyncPeriod    string `yaml:"syncPeriod"`
	MinSyncPeriod string `yaml:"minSyncPeriod"`
}

type KubeDnsAutoscaler struct {
	CoresPerReplica int `yaml:"coresPerReplica"`
	NodesPerReplica int `yaml:"nodesPerReplica"`
	Min             int `yaml:"min"`
}

type KubeDns struct {
	NodeLocalResolver   bool              `yaml:"nodeLocalResolver"`
	DeployToControllers bool              `yaml:"deployToControllers"`
	Autoscaler          KubeDnsAutoscaler `yaml:"autoscaler"`
}

func (c *KubeDns) MergeIfEmpty(other KubeDns) {
	if c.NodeLocalResolver == false && c.DeployToControllers == false {
		c.NodeLocalResolver = other.NodeLocalResolver
		c.DeployToControllers = other.DeployToControllers
	}
}

type KubernetesDashboard struct {
	AdminPrivileges bool `yaml:"adminPrivileges"`
	InsecureLogin   bool `yaml:"insecureLogin"`
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

// AdminAPIEndpointURL is the url of the API endpoint which is written in kubeconfig and used to by admins
func (c *Config) AdminAPIEndpointURL() string {
	return fmt.Sprintf("https://%s", c.AdminAPIEndpoint.DNSName)
}

// Required by kubelet to use the consistent network plugin with the base cluster
func (c KubeClusterSettings) K8sNetworkPlugin() string {
	return "cni"
}

func (c Cluster) Config(extra ...[]*pluginmodel.Plugin) (*Config, error) {
	pluginMap := map[string]*pluginmodel.Plugin{}
	plugins := []*pluginmodel.Plugin{}
	if len(extra) > 0 {
		plugins = extra[0]
		for _, p := range plugins {
			pluginMap[p.SettingKey()] = p
		}
	}

	config := Config{
		Cluster:          c,
		KubeAwsPlugins:   pluginMap,
		APIServerFlags:   pluginmodel.APIServerFlags{},
		APIServerVolumes: pluginmodel.APIServerVolumes{},
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

	apiEndpointNames := []string{}
	for _, e := range apiEndpoints {
		apiEndpointNames = append(apiEndpointNames, e.Name)
	}

	var adminAPIEndpoint derived.APIEndpoint
	if c.AdminAPIEndpointName != "" {
		found, err := apiEndpoints.FindByName(c.AdminAPIEndpointName)
		if err != nil {
			return nil, fmt.Errorf("failed to find an API endpoint named \"%s\": %v", c.AdminAPIEndpointName, err)
		}
		adminAPIEndpoint = *found
	} else {
		if len(apiEndpoints) > 1 {
			return nil, fmt.Errorf(
				"adminAPIEndpointName must not be empty when there's 2 or more api endpoints under the key `apiEndpoints`. Specify one of: %s",
				strings.Join(apiEndpointNames, ", "),
			)
		}
		adminAPIEndpoint = apiEndpoints.GetDefault()
	}
	config.AdminAPIEndpoint = adminAPIEndpoint

	return &config, nil
}

func (c *Cluster) EtcdCluster() derived.EtcdCluster {
	etcdNetwork := derived.NewNetwork(c.Etcd.Subnets, c.NATGateways())
	return derived.NewEtcdCluster(c.Etcd.Cluster, c.Region, etcdNetwork, c.Etcd.Count)
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

func (c Cluster) StackConfig(opts StackTemplateOptions, extra ...[]*pluginmodel.Plugin) (*StackConfig, error) {
	plugins := []*pluginmodel.Plugin{}
	if len(extra) > 0 {
		plugins = extra[0]
	}

	var err error
	stackConfig := StackConfig{
		ExtraCfnResources: map[string]interface{}{},
	}

	if stackConfig.Config, err = c.Config(plugins); err != nil {
		return nil, err
	}

	var compactAssets *CompactAssets

	if c.AssetsEncryptionEnabled() {
		compactAssets, err = ReadOrCreateCompactAssets(opts.AssetsDir, c.ManageCertificates, c.Experimental.TLSBootstrap.Enabled, c.Experimental.KIAMSupport.Enabled, KMSConfig{
			Region:         stackConfig.Config.Region,
			KMSKeyARN:      c.KMSKeyARN,
			EncryptService: c.ProvidedEncryptService,
		})
		if err != nil {
			return nil, err
		}

		stackConfig.Config.AssetsConfig = compactAssets
	} else {
		rawAssets, err := ReadOrCreateUnencryptedCompactAssets(opts.AssetsDir, c.ManageCertificates, c.Experimental.TLSBootstrap.Enabled, c.Experimental.KIAMSupport.Enabled)
		if err != nil {
			return nil, err
		}

		stackConfig.Config.AssetsConfig = rawAssets
	}

	stackConfig.StackTemplateOptions = opts
	stackConfig.S3URI = strings.TrimSuffix(opts.S3URI, "/")

	if opts.SkipWait {
		enabled := false
		stackConfig.WaitSignal.EnabledOverride = &enabled
	}

	return &stackConfig, nil
}

type InitialConfig struct {
	AmiId            string
	AvailabilityZone string
	ClusterName      string
	ExternalDNSName  string
	HostedZoneID     string
	KMSKeyARN        string
	KeyName          string
	NoRecordSet      bool
	Region           model.Region
}

// Config contains configuration parameters available when rendering userdata injected into a controller or an etcd node from golang text templates
type Config struct {
	Cluster

	AdminAPIEndpoint derived.APIEndpoint
	APIEndpoints     derived.APIEndpoints

	// EtcdNodes is the golang-representation of etcd nodes, which is used to differentiate unique etcd nodes
	// This is used to simplify templating of the control-plane stack template.
	EtcdNodes []derived.EtcdNode

	AssetsConfig *CompactAssets

	KubeAwsPlugins map[string]*pluginmodel.Plugin

	APIServerVolumes pluginmodel.APIServerVolumes
	APIServerFlags   pluginmodel.APIServerFlags
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

func (c Config) VPCLogicalName() (string, error) {
	if c.VPC.HasIdentifier() {
		return "", fmt.Errorf("[BUG] .VPCLogicalName should not be called in stack template when vpc id is specified")
	}
	return vpcLogicalName, nil
}

func (c Config) VPCID() (string, error) {
	fmt.Println("WARN: .VPCID in stack template is deprecated and will be removed in v0.9.9. Please use .VPC.ID instead")
	if !c.VPC.HasIdentifier() {
		return "", fmt.Errorf("[BUG] .VPCID should not be called in stack template when vpc.id(FromStackOutput) is specified. Use .VPCManaged instead.")
	}
	return c.VPC.ID, nil
}

func (c Config) VPCManaged() bool {
	return !c.VPC.HasIdentifier()
}

func (c Config) VPCRef() (string, error) {
	return c.VPC.RefOrError(c.VPCLogicalName)
}

func (c Config) InternetGatewayLogicalName() string {
	return internetGatewayLogicalName
}

func (c Config) InternetGatewayRef() string {
	return c.InternetGateway.Ref(c.InternetGatewayLogicalName)
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

// NestedStackName returns a sanitized name of this control-plane which is usable as a valid cloudformation nested stack name
func (c Cluster) NestedStackName() string {
	// Convert stack name into something valid as a cfn resource name or
	// we'll end up with cfn errors like "Template format error: Resource name test5-controlplane is non alphanumeric"
	return strings.Title(strings.Replace(c.StackName(), "-", "", -1))
}

func (c Cluster) NodeLabels() model.NodeLabels {
	labels := c.NodeSettings.NodeLabels
	if c.Addons.ClusterAutoscaler.Enabled {
		labels["kube-aws.coreos.com/cluster-autoscaler-supported"] = "true"
	}
	return labels
}

// Etcdadm returns the content of the etcdadm script to be embedded into cloud-config-etcd
func (c *Config) Etcdadm() (string, error) {
	return gzipcompressor.CompressData(Etcdadm)
}

func (c Cluster) validate() error {
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

	if err := c.ControllerSettings.Validate(); err != nil {
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
	replacer := strings.NewReplacer(clusterNamePlaceholder, "", nestedStackNamePlaceHolder, c.StackName())
	simulatedLcName := fmt.Sprintf("%s-%s-1N2C4K3LLBEDZ-%sLC-BC2S9P3JG2QD", clusterNamePlaceholder, nestedStackNamePlaceHolder, c.Controller.LogicalName())
	limit := 63 - len(replacer.Replace(simulatedLcName))
	if c.Experimental.AwsNodeLabels.Enabled && len(c.ClusterName) > limit {
		return fmt.Errorf("awsNodeLabels can't be enabled for controllers because the total number of characters in clusterName(=\"%s\") exceeds the limit of %d", c.ClusterName, limit)
	}

	if c.Controller.InstanceType == "t2.micro" || c.Etcd.InstanceType == "t2.micro" || c.Controller.InstanceType == "t2.nano" || c.Etcd.InstanceType == "t2.nano" {
		fmt.Println(`WARNING: instance types "t2.nano" and "t2.micro" are not recommended. See https://github.com/kubernetes-incubator/kube-aws/issues/258 for more information`)
	}

	if len(c.Controller.IAMConfig.Role.Name) > 0 {
		if e := cfnresource.ValidateStableRoleNameLength(c.Controller.IAMConfig.Role.Name, c.Region.String()); e != nil {
			return e
		}
	} else {
		if e := cfnresource.ValidateUnstableRoleNameLength(c.ClusterName, c.NestedStackName(), c.Controller.IAMConfig.Role.Name, c.Region.String()); e != nil {
			return e
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
	if c.KMSKeyARN == "" && c.AssetsEncryptionEnabled() {
		return nil, errors.New("kmsKeyArn must be set")
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

	if err := c.Experimental.Validate(); err != nil {
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

func (s DeploymentSettings) AllSubnets() model.Subnets {
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

func (c DeploymentSettings) PrivateSubnets() model.Subnets {
	result := []model.Subnet{}
	for _, s := range c.Subnets {
		if s.Private {
			result = append(result, s)
		}
	}
	return result
}

func (c DeploymentSettings) PublicSubnets() model.Subnets {
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

func (c ControllerSettings) Validate() error {
	controller := c.Controller
	rootVolume := controller.RootVolume

	if rootVolume.Type == "io1" {
		if rootVolume.IOPS < 100 || rootVolume.IOPS > 20000 {
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
func (e EtcdSettings) Validate() error {
	if !e.Etcd.DataVolume.Encrypted && e.Etcd.KMSKeyARN() != "" {
		return errors.New("`etcd.kmsKeyArn` can only be specified when `etcdDataVolumeEncrypted` is enabled")
	}

	if err := e.IAMConfig.Validate(); err != nil {
		return fmt.Errorf("invalid etcd settings: %v", err)
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

func (c Experimental) Validate() error {
	if err := c.NodeDrainer.Validate(); err != nil {
		return err
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

type kubernetesManifestPlugin struct {
	Manifests []pluggedInKubernetesManifest
}

func (p kubernetesManifestPlugin) ManifestListFile() node.UploadedFile {
	paths := []string{}
	for _, m := range p.Manifests {
		paths = append(paths, m.ManifestFile.Path)
	}
	bytes := []byte(strings.Join(paths, "\n"))
	return node.UploadedFile{
		Path:    p.listFilePath(),
		Content: node.NewUploadedFileContent(bytes),
	}
}

func (p kubernetesManifestPlugin) listFilePath() string {
	return "/srv/kube-aws/plugins/kubernetes-manifests"
}

func (p kubernetesManifestPlugin) Directory() string {
	return filepath.Dir(p.listFilePath())
}

type pluggedInKubernetesManifest struct {
	ManifestFile node.UploadedFile
}

type helmReleasePlugin struct {
	Releases []pluggedInHelmRelease
}

func (p helmReleasePlugin) ReleaseListFile() node.UploadedFile {
	paths := []string{}
	for _, r := range p.Releases {
		paths = append(paths, r.ReleaseFile.Path)
	}
	bytes := []byte(strings.Join(paths, "\n"))
	return node.UploadedFile{
		Path:    p.listFilePath(),
		Content: node.NewUploadedFileContent(bytes),
	}
}

func (p helmReleasePlugin) listFilePath() string {
	return "/srv/kube-aws/plugins/helm-releases"
}

func (p helmReleasePlugin) Directory() string {
	return filepath.Dir(p.listFilePath())
}

type pluggedInHelmRelease struct {
	ValuesFile  node.UploadedFile
	ReleaseFile node.UploadedFile
}

func (c *Config) KubernetesManifestPlugin() kubernetesManifestPlugin {
	manifests := []pluggedInKubernetesManifest{}
	for pluginName, _ := range c.PluginConfigs {
		plugin, ok := c.KubeAwsPlugins[pluginName]
		if !ok {
			panic(fmt.Errorf("Plugin %s is requested but not loaded. Probably a typo in the plugin name inside cluster.yaml?", pluginName))
		}
		for _, manifestConfig := range plugin.Configuration.Kubernetes.Manifests {
			bytes := []byte(manifestConfig.Contents.Inline)
			m := pluggedInKubernetesManifest{
				ManifestFile: node.UploadedFile{
					Path:    filepath.Join("/srv/kube-aws/plugins", plugin.Metadata.Name, manifestConfig.Name),
					Content: node.NewUploadedFileContent(bytes),
				},
			}
			manifests = append(manifests, m)
		}
	}
	p := kubernetesManifestPlugin{
		Manifests: manifests,
	}
	return p
}

func (c *Config) HelmReleasePlugin() helmReleasePlugin {
	releases := []pluggedInHelmRelease{}
	for pluginName, _ := range c.PluginConfigs {
		plugin := c.KubeAwsPlugins[pluginName]
		for _, releaseConfig := range plugin.Configuration.Helm.Releases {
			valuesFilePath := filepath.Join("/srv/kube-aws/plugins", plugin.Metadata.Name, "helm", "releases", releaseConfig.Name, "values.yaml")
			valuesFileContent, err := json.Marshal(releaseConfig.Values)
			if err != nil {
				panic(fmt.Errorf("Unexpected error in HelmReleasePlugin: %v", err))
			}
			releaseFileData := map[string]interface{}{
				"values": map[string]string{
					"file": valuesFilePath,
				},
				"chart": map[string]string{
					"name":    releaseConfig.Name,
					"version": releaseConfig.Version,
				},
			}
			releaseFilePath := filepath.Join("/srv/kube-aws/plugins", plugin.Metadata.Name, "helm", "releases", releaseConfig.Name, "release.json")
			releaseFileContent, err := json.Marshal(releaseFileData)
			if err != nil {
				panic(fmt.Errorf("Unexpected error in HelmReleasePlugin: %v", err))
			}
			r := pluggedInHelmRelease{
				ValuesFile: node.UploadedFile{
					Path:    valuesFilePath,
					Content: node.NewUploadedFileContent(valuesFileContent),
				},
				ReleaseFile: node.UploadedFile{
					Path:    releaseFilePath,
					Content: node.NewUploadedFileContent(releaseFileContent),
				},
			}
			releases = append(releases, r)
		}
	}
	p := helmReleasePlugin{}
	return p
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
