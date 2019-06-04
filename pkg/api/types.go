package api

import "fmt"

type Worker struct {
	APIEndpointName         string           `yaml:"apiEndpointName,omitempty"`
	NodePools               []WorkerNodePool `yaml:"nodePools,omitempty"`
	NodePoolRollingStrategy string           `yaml:"nodePoolRollingStrategy,omitempty"`
	UnknownKeys             `yaml:",inline"`
}

// Kubelet options
type Kubelet struct {
	SystemReservedResources string                 `yaml:"systemReserved,omitempty"`
	KubeReservedResources   string                 `yaml:"kubeReserved,omitempty"`
	Kubeconfig              string                 `yaml:"kubeconfig,omitempty"`
	Mounts                  []ContainerVolumeMount `yaml:"mounts,omitempty"`
	Flags                   CommandLineFlags       `yaml:"flags,omitempty"`
}

type Experimental struct {
	Admission                   Admission             `yaml:"admission"`
	AuditLog                    AuditLog              `yaml:"auditLog"`
	Authentication              Authentication        `yaml:"authentication"`
	AwsEnvironment              AwsEnvironment        `yaml:"awsEnvironment"`
	AwsNodeLabels               AwsNodeLabels         `yaml:"awsNodeLabels"`
	EphemeralImageStorage       EphemeralImageStorage `yaml:"ephemeralImageStorage"`
	KIAMSupport                 KIAMSupport           `yaml:"kiamSupport,omitempty"`
	Kube2IamSupport             Kube2IamSupport       `yaml:"kube2IamSupport,omitempty"`
	GpuSupport                  GpuSupport            `yaml:"gpuSupport,omitempty"`
	KubeletOpts                 string                `yaml:"kubeletOpts,omitempty"`
	LoadBalancer                LoadBalancer          `yaml:"loadBalancer"`
	TargetGroup                 TargetGroup           `yaml:"targetGroup"`
	NodeDrainer                 NodeDrainer           `yaml:"nodeDrainer"`
	Oidc                        Oidc                  `yaml:"oidc"`
	DisableSecurityGroupIngress bool                  `yaml:"disableSecurityGroupIngress"`
	NodeMonitorGracePeriod      string                `yaml:"nodeMonitorGracePeriod"`
	UnknownKeys                 `yaml:",inline"`
}

func (c Experimental) Validate(name string) error {
	if err := c.NodeDrainer.Validate(); err != nil {
		return err
	}

	if c.Kube2IamSupport.Enabled && c.KIAMSupport.Enabled {
		return fmt.Errorf("at '%s', you can enable kube2IamSupport or kiamSupport, but not both", name)
	}

	return nil
}

type Admission struct {
	AlwaysPullImages                     AlwaysPullImages                     `yaml:"alwaysPullImages"`
	Initializers                         Initializers                         `yaml:"initializers"`
	OwnerReferencesPermissionEnforcement OwnerReferencesPermissionEnforcement `yaml:"ownerReferencesPermissionEnforcement"`
}

type AlwaysPullImages struct {
	Enabled bool `yaml:"enabled"`
}

type Initializers struct {
	Enabled bool `yaml:"enabled"`
}

type OwnerReferencesPermissionEnforcement struct {
	Enabled bool `yaml:"enabled"`
}

type PersistentVolumeClaimResize struct {
	Enabled bool `yaml:"enabled"`
}

type AuditLog struct {
	Enabled   bool   `yaml:"enabled"`
	LogPath   string `yaml:"logPath"`
	MaxAge    int    `yaml:"maxAge"`
	MaxBackup int    `yaml:"maxBackup"`
	MaxSize   int    `yaml:"maxSize"`
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

type EncryptionAtRest struct {
	Enabled bool `yaml:"enabled"`
}

type EphemeralImageStorage struct {
	Enabled    bool   `yaml:"enabled"`
	Disk       string `yaml:"disk"`
	Filesystem string `yaml:"filesystem"`
}

type KIAMSupport struct {
	Enabled         bool                `yaml:"enabled"`
	Image           Image               `yaml:"image,omitempty"`
	SessionDuration string              `yaml:"sessionDuration,omitempty"`
	ServerAddresses KIAMServerAddresses `yaml:"serverAddresses,omitempty"`
	ServerResources ComputeResources    `yaml:"serverResources,omitempty"`
	AgentResources  ComputeResources    `yaml:"agentResources,omitempty"`
}

type KIAMServerAddresses struct {
	ServerAddress string `yaml:"serverAddress,omitempty"`
	AgentAddress  string `yaml:"agentAddress,omitempty"`
}

type Kube2IamSupport struct {
	Enabled bool `yaml:"enabled"`
}

type GpuSupport struct {
	Enabled      bool   `yaml:"enabled"`
	Version      string `yaml:"version"`
	InstallImage string `yaml:"installImage"`
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
	Interval int    `yaml:"interval"`
}

type HostOS struct {
	BashPrompt BashPrompt `yaml:"bashPrompt,omitempty"`
	MOTDBanner MOTDBanner `yaml:"motdBanner,omitempty"`
}

func (c *LocalStreaming) IntervalSec() int64 {
	// Convert from seconds to milliseconds (and return as int64 type)
	return int64(c.Interval * 1000)
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
	IPVSMode         IPVSMode               `yaml:"ipvsMode"`
	ComputeResources ComputeResources       `yaml:"resources,omitempty"`
	Config           map[string]interface{} `yaml:"config,omitempty"`
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
	Provider                 string            `yaml:"provider"`
	NodeLocalResolver        bool              `yaml:"nodeLocalResolver"`
	NodeLocalResolverOptions []string          `yaml:"nodeLocalResolverOptions"`
	DeployToControllers      bool              `yaml:"deployToControllers"`
	Autoscaler               KubeDnsAutoscaler `yaml:"autoscaler"`
}

func (c *KubeDns) MergeIfEmpty(other KubeDns) {
	if c.NodeLocalResolver == false && c.DeployToControllers == false {
		c.NodeLocalResolver = other.NodeLocalResolver
		c.DeployToControllers = other.DeployToControllers
	}
}
