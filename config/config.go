package config

//go:generate go run templates_gen.go
//go:generate gofmt -w templates.go

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"text/template"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"

	"github.com/coreos/coreos-cloudinit/config/validate"
	"github.com/coreos/go-semver/semver"
	"github.com/coreos/kube-aws/coreosutil"
	yaml "gopkg.in/yaml.v2"
)

const (
	credentialsDir = "credentials"
	userDataDir    = "userdata"
)

func newDefaultCluster() *Cluster {
	experimental := Experimental{
		NodeDrainer{
			Enabled: false,
		},
	}

	return &Cluster{
		ClusterName:              "kubernetes",
		ReleaseChannel:           "stable",
		VPCCIDR:                  "10.0.0.0/16",
		PodCIDR:                  "10.2.0.0/16",
		ServiceCIDR:              "10.3.0.0/24",
		DNSServiceIP:             "10.3.0.10",
		K8sVer:                   "v1.4.5_coreos.0",
		HyperkubeImageRepo:       "quay.io/coreos/hyperkube",
		TLSCADurationDays:        365 * 10,
		TLSCertDurationDays:      365,
		ContainerRuntime:         "docker",
		ControllerCount:          1,
		ControllerInstanceType:   "m3.medium",
		ControllerRootVolumeType: "gp2",
		ControllerRootVolumeIOPS: 0,
		ControllerRootVolumeSize: 30,
		WorkerCount:              1,
		WorkerInstanceType:       "m3.medium",
		WorkerRootVolumeType:     "gp2",
		WorkerRootVolumeIOPS:     0,
		WorkerRootVolumeSize:     30,
		EtcdCount:                1,
		EtcdInstanceType:         "m3.medium",
		EtcdRootVolumeSize:       30,
		EtcdDataVolumeSize:       30,
		CreateRecordSet:          false,
		RecordSetTTL:             300,
		Subnets:                  []*Subnet{},
		MapPublicIPs:             true,
		Experimental:             experimental,
		IsChinaRegion:            false,
	}
}

func newDefaultClusterWithDeps(encSvc encryptService) *Cluster {
	cluster := newDefaultCluster()
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
	c := newDefaultCluster()
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

type Cluster struct {
	ClusterName              string            `yaml:"clusterName,omitempty"`
	ExternalDNSName          string            `yaml:"externalDNSName,omitempty"`
	KeyName                  string            `yaml:"keyName,omitempty"`
	Region                   string            `yaml:"region,omitempty"`
	AvailabilityZone         string            `yaml:"availabilityZone,omitempty"`
	ReleaseChannel           string            `yaml:"releaseChannel,omitempty"`
	AmiId                    string            `yaml:"amiId,omitempty"`
	ControllerCount          int               `yaml:"controllerCount,omitempty"`
	ControllerInstanceType   string            `yaml:"controllerInstanceType,omitempty"`
	ControllerRootVolumeType string            `yaml:"controllerRootVolumeType,omitempty"`
	ControllerRootVolumeIOPS int               `yaml:"controllerRootVolumeIOPS,omitempty"`
	ControllerRootVolumeSize int               `yaml:"controllerRootVolumeSize,omitempty"`
	WorkerCount              int               `yaml:"workerCount,omitempty"`
	WorkerInstanceType       string            `yaml:"workerInstanceType,omitempty"`
	WorkerRootVolumeType     string            `yaml:"workerRootVolumeType,omitempty"`
	WorkerRootVolumeIOPS     int               `yaml:"workerRootVolumeIOPS,omitempty"`
	WorkerRootVolumeSize     int               `yaml:"workerRootVolumeSize,omitempty"`
	WorkerSpotPrice          string            `yaml:"workerSpotPrice,omitempty"`
	EtcdLoadBalancer         string            `yaml:"etcdLoadBalancer,omitempty"`
	EtcdSecurityGroups       []string          `yaml:"etcdSecurityGroups,omitempty"`
	EtcdCount                int               `yaml:"etcdCount"`
	EtcdInstanceType         string            `yaml:"etcdInstanceType,omitempty"`
	EtcdRootVolumeSize       int               `yaml:"etcdRootVolumeSize,omitempty"`
	EtcdDataVolumeSize       int               `yaml:"etcdDataVolumeSize,omitempty"`
	EtcdDataVolumeEphemeral  bool              `yaml:"etcdDataVolumEphemeral,omitempty"`
	VPCID                    string            `yaml:"vpcId,omitempty"`
	RouteTableID             string            `yaml:"routeTableId,omitempty"`
	VPCCIDR                  string            `yaml:"vpcCIDR,omitempty"`
	InstanceCIDR             string            `yaml:"instanceCIDR,omitempty"`
	PodCIDR                  string            `yaml:"podCIDR,omitempty"`
	ServiceCIDR              string            `yaml:"serviceCIDR,omitempty"`
	DNSServiceIP             string            `yaml:"dnsServiceIP,omitempty"`
	K8sVer                   string            `yaml:"kubernetesVersion,omitempty"`
	HyperkubeImageRepo       string            `yaml:"hyperkubeImageRepo,omitempty"`
	ContainerRuntime         string            `yaml:"containerRuntime,omitempty"`
	KMSKeyARN                string            `yaml:"kmsKeyArn,omitempty"`
	CreateRecordSet          bool              `yaml:"createRecordSet,omitempty"`
	RecordSetTTL             int               `yaml:"recordSetTTL,omitempty"`
	TLSCADurationDays        int               `yaml:"tlsCADurationDays,omitempty"`
	TLSCertDurationDays      int               `yaml:"tlsCertDurationDays,omitempty"`
	HostedZone               string            `yaml:"hostedZone,omitempty"`
	HostedZoneID             string            `yaml:"hostedZoneId,omitempty"`
	StackTags                map[string]string `yaml:"stackTags,omitempty"`
	UseCalico                bool              `yaml:"useCalico,omitempty"`
	Subnets                  []*Subnet         `yaml:"subnets,omitempty"`
	MapPublicIPs             bool              `yaml:"mapPublicIPs,omitempty"`
	ElasticFileSystemID      string            `yaml:"elasticFileSystemId,omitempty"`
	Experimental             Experimental      `yaml:"experimental"`
	providedEncryptService   encryptService
	IsChinaRegion            bool
}

type Subnet struct {
	AvailabilityZone  string `yaml:"availabilityZone,omitempty"`
	InstanceCIDR      string `yaml:"instanceCIDR,omitempty"`
	lastAllocatedAddr *net.IP
}

type Experimental struct {
	NodeDrainer NodeDrainer `yaml:"nodeDrainer"`
}

type NodeDrainer struct {
	Enabled bool `yaml:"enabled"`
}

const (
	vpcLogicalName = "VPC"
)

var supportedReleaseChannels = map[string]bool{
	"alpha":  true,
	"beta":   true,
	"stable": true,
}

func (c Cluster) Config() (*Config, error) {
	config := Config{Cluster: c}

	config.MinWorkerCount = config.WorkerCount - 1
	config.MaxWorkerCount = config.WorkerCount + 1

	config.MinControllerCount = config.ControllerCount - 1
	config.MaxControllerCount = config.ControllerCount + 1

	config.APIServerEndpoint = fmt.Sprintf("https://%s", c.ExternalDNSName)
	config.K8sNetworkPlugin = "cni"

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
		if config.AMI, err = getAMI(config.Region, config.ReleaseChannel); err != nil {
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

	// Use custom etcd cluster if etcdLoadBalancer is set. TODO some minimal URL validation.
	if config.EtcdLoadBalancer != "" {
		config.EtcdEndpoints = config.EtcdLoadBalancer
		// Keep EtcdInitialCluster empty as sign for the template engine to skip etcd node creation
		config.EtcdInitialCluster = ""
	} else {
		config.EtcdInstances = make([]etcdInstance, config.EtcdCount)
		var etcdEndpoints, etcdInitialCluster bytes.Buffer
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
					ip = incrementIP(ip)
				}
				subnet.lastAllocatedAddr = &ip
			}

			nextAddr := incrementIP(*subnet.lastAllocatedAddr)
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
	}

	config.IsChinaRegion = strings.HasPrefix(config.Region, "cn")

	return &config, nil
}

// releaseVersionIsGreaterThan will return true if the supplied version is greater then
// or equal to the current CoreOS release indicated by the given release
// channel.
func releaseVersionIsGreaterThan(minVersion semver.Version, release string) (bool, error) {
	metaData, err := coreosutil.GetAMIData(release)
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

func execute(filename string, data interface{}, compress bool) (string, error) {
	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New(filename).Parse(string(raw))
	if err != nil {
		return "", err
	}
	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, data); err != nil {
		return "", err
	}
	if compress {
		return compressData(buff.Bytes())
	}
	return buff.String(), nil
}

func (c Cluster) stackConfig(opts StackTemplateOptions, compressUserData bool) (*stackConfig, error) {
	assets, err := ReadTLSAssets(opts.TLSAssetsDir)
	if err != nil {
		return nil, err
	}
	stackConfig := stackConfig{}

	if stackConfig.Config, err = c.Config(); err != nil {
		return nil, err
	}

	awsConfig := aws.NewConfig().
		WithRegion(stackConfig.Config.Region).
		WithCredentialsChainVerboseErrors(true)

	// TODO Cleaner way to inject this dependency
	var kmsSvc encryptService
	if c.providedEncryptService != nil {
		kmsSvc = c.providedEncryptService
	} else {
		kmsSvc = kms.New(session.New(awsConfig))
	}

	compactAssets, err := assets.compact(stackConfig.Config, kmsSvc)
	if err != nil {
		return nil, fmt.Errorf("failed to compress TLS assets: %v", err)
	}

	stackConfig.Config.TLSConfig = compactAssets

	if stackConfig.UserDataWorker, err = execute(opts.WorkerTmplFile, stackConfig.Config, compressUserData); err != nil {
		return nil, fmt.Errorf("failed to render worker cloud config: %v", err)
	}
	if stackConfig.UserDataController, err = execute(opts.ControllerTmplFile, stackConfig.Config, compressUserData); err != nil {
		return nil, fmt.Errorf("failed to render controller cloud config: %v", err)
	}
	if stackConfig.UserDataEtcd, err = execute(opts.EtcdTmplFile, stackConfig.Config, compressUserData); err != nil {
		return nil, fmt.Errorf("failed to render etcd cloud config: %v", err)
	}

	return &stackConfig, nil
}

func (c Cluster) ValidateUserData(opts StackTemplateOptions) error {
	stackConfig, err := c.stackConfig(opts, false)
	if err != nil {
		return err
	}

	errors := []string{}

	for _, userData := range []struct {
		Name    string
		Content string
	}{
		{
			Content: stackConfig.UserDataWorker,
			Name:    "UserDataWorker",
		},
		{
			Content: stackConfig.UserDataController,
			Name:    "UserDataController",
		},
		{
			Content: stackConfig.UserDataEtcd,
			Name:    "UserDataEtcd",
		},
	} {
		report, err := validate.Validate([]byte(userData.Content))

		if err != nil {
			errors = append(
				errors,
				fmt.Sprintf("cloud-config %s could not be parsed: %v",
					userData.Name,
					err,
				),
			)
			continue
		}

		for _, entry := range report.Entries() {
			errors = append(errors, fmt.Sprintf("%s: %+v", userData.Name, entry))
		}
	}

	if len(errors) > 0 {
		reportString := strings.Join(errors, "\n")
		return fmt.Errorf("cloud-config validation errors:\n%s\n", reportString)
	}

	return nil
}

func (c Cluster) RenderStackTemplate(opts StackTemplateOptions) ([]byte, error) {
	stackConfig, err := c.stackConfig(opts, true)
	if err != nil {
		return nil, err
	}

	rendered, err := execute(opts.StackTemplateTmplFile, stackConfig, false)
	if err != nil {
		return nil, err
	}

	//Use unmarshal function to do syntax validation
	renderedBytes := []byte(rendered)
	var jsonHolder map[string]interface{}
	if err := json.Unmarshal(renderedBytes, &jsonHolder); err != nil {
		syntaxError, ok := err.(*json.SyntaxError)
		if ok {
			contextString := getContextString(renderedBytes, int(syntaxError.Offset), 3)
			return nil, fmt.Errorf("%v:\njson syntax error (offset=%d), in this region:\n-------\n%s\n-------\n", err, syntaxError.Offset, contextString)
		}
		return nil, err
	}

	// minify JSON
	var buff bytes.Buffer
	if err := json.Compact(&buff, renderedBytes); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

func getContextString(buf []byte, offset, lineCount int) string {

	linesSeen := 0
	var leftLimit int
	for leftLimit = offset; leftLimit > 0 && linesSeen <= lineCount; leftLimit-- {
		if buf[leftLimit] == '\n' {
			linesSeen++
		}
	}

	linesSeen = 0
	var rightLimit int
	for rightLimit = offset + 1; rightLimit < len(buf) && linesSeen <= lineCount; rightLimit++ {
		if buf[rightLimit] == '\n' {
			linesSeen++
		}
	}

	return string(buf[leftLimit:rightLimit])
}

type etcdInstance struct {
	IPAddress   net.IP
	SubnetIndex int
}

type Config struct {
	Cluster

	MinWorkerCount int
	MaxWorkerCount int

	MinControllerCount int
	MaxControllerCount int

	EtcdEndpoints      string
	EtcdInitialCluster string
	EtcdInstances      []etcdInstance

	APIServerEndpoint string
	AMI               string

	// Encoded TLS assets
	TLSConfig *CompactTLSAssets

	//Logical names of dynamic resources
	VPCLogicalName string

	//Reference strings for dynamic resources
	VPCRef string

	K8sNetworkPlugin string
}

func (c Cluster) valid() error {
	if c.ExternalDNSName == "" {
		return errors.New("externalDNSName must be set")
	}

	releaseChannelSupported := supportedReleaseChannels[c.ReleaseChannel]
	if !releaseChannelSupported {
		return fmt.Errorf("releaseChannel %s is not supported", c.ReleaseChannel)
	}

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
		if c.RecordSetTTL != newDefaultCluster().RecordSetTTL {
			return errors.New(
				"recordSetTTL should not be modified when createRecordSet is false",
			)
		}
	}
	if c.KeyName == "" {
		return errors.New("keyName must be set")
	}
	if c.Region == "" {
		return errors.New("region must be set")
	}
	if c.ClusterName == "" {
		return errors.New("clusterName must be set")
	}
	if c.KMSKeyARN == "" {
		return errors.New("kmsKeyArn must be set")
	}

	if c.VPCID == "" && c.RouteTableID != "" {
		return errors.New("vpcId must be specified if routeTableId is specified")
	}

	_, vpcNet, err := net.ParseCIDR(c.VPCCIDR)
	if err != nil {
		return fmt.Errorf("invalid vpcCIDR: %v", err)
	}

	if len(c.Subnets) == 0 {
		if c.AvailabilityZone == "" {
			return fmt.Errorf("availabilityZone must be set")
		}
		_, instanceCIDR, err := net.ParseCIDR(c.InstanceCIDR)
		if err != nil {
			return fmt.Errorf("invalid instanceCIDR: %v", err)
		}
		if !vpcNet.Contains(instanceCIDR.IP) {
			return fmt.Errorf("vpcCIDR (%s) does not contain instanceCIDR (%s)",
				c.VPCCIDR,
				c.InstanceCIDR,
			)
		}
	} else {
		if c.InstanceCIDR != "" {
			return fmt.Errorf("The top-level instanceCIDR(%s) must be empty when subnets are specified", c.InstanceCIDR)
		}
		if c.AvailabilityZone != "" {
			return fmt.Errorf("The top-level availabilityZone(%s) must be empty when subnets are specified", c.AvailabilityZone)
		}

		var instanceCIDRs = make([]*net.IPNet, 0)
		for i, subnet := range c.Subnets {
			if subnet.AvailabilityZone == "" {
				return fmt.Errorf("availabilityZone must be set for subnet #%d", i)
			}
			_, instanceCIDR, err := net.ParseCIDR(subnet.InstanceCIDR)
			if err != nil {
				return fmt.Errorf("invalid instanceCIDR for subnet #%d: %v", i, err)
			}
			instanceCIDRs = append(instanceCIDRs, instanceCIDR)
			if !vpcNet.Contains(instanceCIDR.IP) {
				return fmt.Errorf("vpcCIDR (%s) does not contain instanceCIDR (%s) for subnet #%d",
					c.VPCCIDR,
					c.InstanceCIDR,
					i,
				)
			}
		}

		for i, a := range instanceCIDRs {
			for j, b := range instanceCIDRs[i+1:] {
				if cidrOverlap(a, b) {
					return fmt.Errorf("CIDR of subnet %d (%s) overlaps with CIDR of subnet %d (%s)", i, a, j, b)
				}
			}
		}
	}

	_, podNet, err := net.ParseCIDR(c.PodCIDR)
	if err != nil {
		return fmt.Errorf("invalid podCIDR: %v", err)
	}

	_, serviceNet, err := net.ParseCIDR(c.ServiceCIDR)
	if err != nil {
		return fmt.Errorf("invalid serviceCIDR: %v", err)
	}
	if cidrOverlap(serviceNet, vpcNet) {
		return fmt.Errorf("vpcCIDR (%s) overlaps with serviceCIDR (%s)", c.VPCCIDR, c.ServiceCIDR)
	}
	if cidrOverlap(podNet, vpcNet) {
		return fmt.Errorf("vpcCIDR (%s) overlaps with podCIDR (%s)", c.VPCCIDR, c.PodCIDR)
	}
	if cidrOverlap(serviceNet, podNet) {
		return fmt.Errorf("serviceCIDR (%s) overlaps with podCIDR (%s)", c.ServiceCIDR, c.PodCIDR)
	}

	kubernetesServiceIPAddr := incrementIP(serviceNet.IP)
	if !serviceNet.Contains(kubernetesServiceIPAddr) {
		return fmt.Errorf("serviceCIDR (%s) does not contain kubernetesServiceIP (%s)", c.ServiceCIDR, kubernetesServiceIPAddr)
	}

	dnsServiceIPAddr := net.ParseIP(c.DNSServiceIP)
	if dnsServiceIPAddr == nil {
		return fmt.Errorf("Invalid dnsServiceIP: %s", c.DNSServiceIP)
	}
	if !serviceNet.Contains(dnsServiceIPAddr) {
		return fmt.Errorf("serviceCIDR (%s) does not contain dnsServiceIP (%s)", c.ServiceCIDR, c.DNSServiceIP)
	}

	if dnsServiceIPAddr.Equal(kubernetesServiceIPAddr) {
		return fmt.Errorf("dnsServiceIp conflicts with kubernetesServiceIp (%s)", dnsServiceIPAddr)
	}

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
			if cidrOverlap(instanceNet, existingSubnet) {
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

//Return next IP address in network range
func incrementIP(netIP net.IP) net.IP {
	ip := make(net.IP, len(netIP))
	copy(ip, netIP)

	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}

	return ip
}

//Does the address space of these networks "a" and "b" overlap?
func cidrOverlap(a, b *net.IPNet) bool {
	return a.Contains(b.IP) || b.Contains(a.IP)
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
