package model

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/kubernetes-incubator/kube-aws/cfnstack"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/naming"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
)

// ValidateStack validates the CloudFormation stack for this control plane already uploaded to S3
func (s *Context) ValidateStack(c *Stack) (string, error) {
	if err := c.validateCertsAgainstSettings(); err != nil {
		return "", err
	}

	templateURL, err := c.TemplateURL()
	if err != nil {
		return "", fmt.Errorf("failed to get template url : %v", err)
	}
	return s.stackProvisioner(c).ValidateStackAtURL(templateURL)
}

func (s *Context) stackProvisioner(c *Stack) *cfnstack.Provisioner {
	stackPolicyBody := `{
  "Statement" : [
    {
       "Effect" : "Allow",
       "Principal" : "*",
       "Action" : "Update:*",
       "Resource" : "*"
     }
  ]
}
`
	return cfnstack.NewProvisioner(
		c.StackName,
		c.Config.StackTags,
		c.ClusterExportedStacksS3URI(),
		c.Region,
		stackPolicyBody,
		s.Session,
		c.Config.CloudFormation.RoleARN,
	)
}

func (s *Context) InspectEtcdExistingState(c *Config) (api.EtcdExistingState, error) {
	var err error
	if s.ProvidedCFInterrogator == nil {
		s.ProvidedCFInterrogator = cloudformation.New(s.Session)
	}
	if s.ProvidedEC2Interrogator == nil {
		s.ProvidedEC2Interrogator = ec2.New(s.Session)
	}

	state := api.EtcdExistingState{}
	state.StackExists, err = cfnstack.NestedStackExists(s.ProvidedCFInterrogator, c.ClusterName, naming.FromStackToCfnResource(c.Etcd.LogicalName()))
	if err != nil {
		return state, fmt.Errorf("failed to check for existence of etcd cloud-formation stack: %v", err)
	}
	// when the Etcd stack exists we need to check for the MajorMinor version of Etcd running and trigger a migration if different to ours.
	if state.StackExists {
		if state.EtcdMigrationEnabled, err = s.isAMajorEtcdUpgrade(c); err != nil {
			return state, fmt.Errorf("failed to check existing etcd major minor version: %v", err)
		}
		if state.EtcdMigrationEnabled {
			if state.EtcdMigrationExistingEndpoints, err = s.lookupExistingEtcdEndpoints(c); err != nil {
				return state, fmt.Errorf("failed to lookup existing etcd endpoints: %v", err)
			}
			logger.Warn("Performing a Major Etcd Version Upgrade: -")
			logger.Warn("To do this we will spin up new etcd servers and then export the existing kubernetes state to them.")
			logger.Warn("There will be cluster apiserver disruption until all of your existing controllers have rolled.")
			logger.Warn("If the cloudformation update fails (at any point) then we will roll back to the original etcd servers.")
			logger.Warn("You MAY lose/rollback changes that are made to the cluster AFTER the etcd export has been performed!")
			logger.Warn("This operation is best scheduled for a quiet time or in an outage window.")
		}
	}
	return state, nil
}

// isAMajorEtcdUpgrade looks for etcd instances using tag kube-aws:etcd_upgrade_group and the config clusters major-minor version.
// If we find no instances then it is an upgrade, otherwise
func (s *Context) isAMajorEtcdUpgrade(c *Config) (bool, error) {
	clusterTag := fmt.Sprintf("tag:kubernetes.io/cluster/%s", c.ClusterName)
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:kube-aws:role"),
				Values: []*string{aws.String("etcd")},
			},
			{
				Name:   aws.String(clusterTag),
				Values: []*string{aws.String("owned")},
			},
			{
				Name:   aws.String("tag:kube-aws:etcd_upgrade_group"),
				Values: []*string{aws.String(c.Etcd.Cluster.MajorMinorVersion())},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []*string{aws.String("running"), aws.String("pending")},
			},
		},
	}

	logger.Debugf("Calling AWS EC2 DescribeInstances ->")
	resp, err := s.ProvidedEC2Interrogator.DescribeInstances(params)
	if err != nil {
		return false, fmt.Errorf("can't lookup ec2 instances: %v", err)
	}
	if resp == nil {
		return false, nil
	}

	logger.Debugf("<- received %d instances from AWS", len(resp.Reservations))
	if len(resp.Reservations) == 0 {
		logger.Debugf("There are 0 instances matching major-minor version %s - this is an upgrade...", c.Etcd.Cluster.MajorMinorVersion())
		return true, nil
	}
	logger.Debugf("Found existing instances matching major-minor version %s - not an upgrade...", c.Etcd.Cluster.MajorMinorVersion())
	return false, nil
}

// lookupExistingEtcdEndpoints supports the migration from embedded etcd servers to their own stack
// by looking up the existing etcd servers for a specific cluster and constructing and etcd endpoints
// list as used by tools such as etcdctl and the etcdadm script.
func (s *Context) lookupExistingEtcdEndpoints(c *Config) (string, error) {
	clusterTag := fmt.Sprintf("tag:kubernetes.io/cluster/%s", c.ClusterName)
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:kube-aws:role"),
				Values: []*string{aws.String("etcd")},
			},
			{
				Name:   aws.String(clusterTag),
				Values: []*string{aws.String("owned")},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []*string{aws.String("running"), aws.String("pending")},
			},
		},
	}
	logger.Debugf("Calling AWS EC2 DescribeInstances ->")
	resp, err := s.ProvidedEC2Interrogator.DescribeInstances(params)
	if err != nil {
		return "", fmt.Errorf("can't lookup ec2 instances: %v", err)
	}
	if resp == nil {
		return "", nil
	}

	logger.Debugf("<- received %d instances from AWS", len(resp.Reservations))
	if len(resp.Reservations) == 0 {
		return "", nil
	}
	// construct comma separated endpoints string
	endpoints := []string{}
	for _, res := range resp.Reservations {
		for _, inst := range res.Instances {
			logger.Debugf("Inspecting AWS instance: %+v", inst)
			// when the instance has an attached network interface we must attach to that one, not the private instance dns
			// the attached interface should be at DeviceIndex 1
			if len(inst.NetworkInterfaces) == 2 {
				for _, i := range inst.NetworkInterfaces {
					if *i.Attachment.DeviceIndex == 1 {
						endpoints = append(endpoints, fmt.Sprintf("https://%s:2379", *i.PrivateDnsName))
					}
				}
			} else {
				endpoints = append(endpoints, fmt.Sprintf("https://%s:2379", *inst.PrivateDnsName))
			}
		}
	}
	result := strings.Join(endpoints, ",")
	logger.Debugf("Existing etcd endpoints found: %s", result)
	return result, nil
}

// ValidateStack validates the CloudFormation stack for this worker node pool already uploaded to S3
func (s *Context) ValidateNodePoolStack(c *NodePoolConfig, stack *Stack) (string, error) {
	ec2Svc := ec2.New(s.Session)

	ref := newNodePoolStackRef(
		c,
		s.Session,
	)
	if err := ref.validateWorkerRootVolume(ec2Svc); err != nil {
		return "", err
	}
	if c.KeyName != "" {
		if err := ref.validateKeyPair(ec2Svc); err != nil {
			return "", err
		}
	}

	stackTemplateURL, err := stack.TemplateURL()
	if err != nil {
		return "", fmt.Errorf("failed to get template url : %v", err)
	}
	return s.stackProvisioner(stack).ValidateStackAtURL(stackTemplateURL)
}
