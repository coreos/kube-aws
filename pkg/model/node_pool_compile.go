package model

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/coreos/amiregistry"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"github.com/pkg/errors"
)

func nodePoolPreprocess(c api.WorkerNodePool, main *Config) (*api.WorkerNodePool, error) {
	if c.SpotFleet.Enabled() {
		enabled := false
		c.WaitSignal.EnabledOverride = &enabled
	}

	c = c.WithDefaultsFrom(main.DefaultWorkerSettings)

	c.DeploymentSettings = c.DeploymentSettings.WithDefaultsFrom(main.DeploymentSettings)

	// Inherit parameters from the control plane stack
	c.KubeClusterSettings = main.KubeClusterSettings
	c.HostOS = main.HostOS
	c.Experimental.TLSBootstrap = main.DeploymentSettings.Experimental.TLSBootstrap
	c.Experimental.NodeDrainer = main.DeploymentSettings.Experimental.NodeDrainer
	c.Experimental.GpuSupport = main.DeploymentSettings.Experimental.GpuSupport
	c.Kubelet.RotateCerts = main.DeploymentSettings.Kubelet.RotateCerts
	c.Kubelet.SystemReservedResources = main.DeploymentSettings.Kubelet.SystemReservedResources
	c.Kubelet.KubeReservedResources = main.DeploymentSettings.Kubelet.KubeReservedResources

	if c.Experimental.ClusterAutoscalerSupport.Enabled {
		if !main.Addons.ClusterAutoscaler.Enabled {
			return nil, errors.New("clusterAutoscalerSupport can't be enabled on node pools when cluster-autoscaler is not going to be deployed to the cluster")
		}
	}

	// Default to public subnets defined in the main cluster
	// CAUTION: cluster-autoscaler Won't work if there're 2 or more subnets spanning over different AZs
	if len(c.Subnets) == 0 {
		var defaults []api.Subnet
		if c.Private {
			defaults = main.PrivateSubnets()
		} else {
			defaults = main.PublicSubnets()
		}
		if len(defaults) == 0 {
			return nil, errors.New(`public subnets required by default for node pool missing in cluster.yaml.
define one or more public subnets in cluster.yaml or explicitly reference private subnets from node pool by specifying nodePools[].subnets[].name`)
		}
		c.Subnets = defaults
	} else {
		// Fetch subnets defined in the main cluster by name
		for i, s := range c.Subnets {
			linkedSubnet := main.FindSubnetMatching(s)
			c.Subnets[i] = linkedSubnet
		}
	}

	// Import all the managed subnets from the network stack i.e. don't create subnets inside the node pool cfn stack
	var err error
	c.Subnets, err = c.Subnets.ImportFromNetworkStack()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to import subnets from network stack: %v", err)
	}

	anySubnetIsManaged := false
	for _, s := range c.Subnets {
		anySubnetIsManaged = anySubnetIsManaged || s.ManageSubnet()
	}

	if anySubnetIsManaged && main.ElasticFileSystemID == "" && c.ElasticFileSystemID != "" {
		return nil, fmt.Errorf("elasticFileSystemId cannot be specified for a node pool in managed subnet(s), but was: %s", c.ElasticFileSystemID)
	}

	return &c, nil
}

func NodePoolCompile(spec api.WorkerNodePool, main *Config) (*NodePoolConfig, error) {
	cfg, err := nodePoolPreprocess(spec, main)
	if err != nil {
		return nil, err
	}

	c := &NodePoolConfig{
		WorkerNodePool: *cfg,
	}

	var ami string
	if spec.AmiId == "" {
		var err error
		if ami, err = amiregistry.GetAMI(main.Region.String(), cfg.ReleaseChannel); err != nil {
			return nil, errors.Wrapf(err, "unable to fetch AMI for worker node pool \"%s\"", spec.NodePoolName)
		}
	} else {
		ami = spec.AmiId
	}
	c.AMI = ami

	c.EtcdNodes = main.EtcdNodes
	c.KubeResourcesAutosave = main.KubeResourcesAutosave

	var apiEndpoint APIEndpoint
	if c.APIEndpointName != "" {
		found, err := main.APIEndpoints.FindByName(c.APIEndpointName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find an API endpoint named \"%s\": %v", c.APIEndpointName, err)
		}
		apiEndpoint = *found
	} else {
		if len(main.APIEndpoints) > 1 {
			return nil, errors.New("worker.nodePools[].apiEndpointName must not be empty when there's 2 or more api endpoints under the key `apiEndpoints")
		}
		apiEndpoint = main.APIEndpoints.GetDefault()
	}

	if !apiEndpoint.LoadBalancer.ManageELBRecordSet() {
		fmt.Printf(`WARN: the worker node pool "%s" is associated to a k8s API endpoint behind the DNS name "%s" managed by YOU!
Please never point the DNS record for it to a different k8s cluster, especially when the name is a "stable" one which is shared among multiple k8s clusters for achieving blue-green deployments of k8s clusters!
kube-aws can't save users from mistakes like that
`, c.NodePoolName, apiEndpoint.DNSName)
	}
	c.APIEndpoint = apiEndpoint

	if spec.Autoscaling.ClusterAutoscaler.Enabled && !main.Addons.ClusterAutoscaler.Enabled {
		return nil, errors.New("Autoscaling with cluster-autoscaler can't be enabled for node pools because " +
			"you didn't enabled the cluster-autoscaler addon. Enable it by turning on `addons.clusterAutoscaler.enabled`")
	}

	if err := c.Validate(); err != nil {
		return nil, errors.Wrapf(err, "invalid node pool spec")
	}

	return c, nil
}
