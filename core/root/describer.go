package root

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/kubernetes-incubator/kube-aws/core/controlplane/cluster"
	cp "github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/config"
)

type Info struct {
	ControlPlane *cluster.Info
}

func (i *Info) String() string {
	return i.ControlPlane.String()
}

type ClusterDescriber interface {
	Info() (*Info, error)
}

type clusterDescriberImpl struct {
	cpConfig    *cp.Config
	session     *session.Session
	clusterName string
	stackName   string
}

func ClusterDescriberFromFile(configPath string) (ClusterDescriber, error) {
	config, err := config.ConfigFromFile(configPath)
	if err != nil {
		return nil, err
	}
	awsConfig := aws.NewConfig().
		WithRegion(config.Region.String()).
		WithCredentialsChainVerboseErrors(true)

	session, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to establish aws session: %v", err)
	}

	cpConfig, err := config.Config()
	if err != nil {
		return nil, err
	}

	return NewClusterDescriber(config.ClusterName, config.ClusterName, cpConfig, session), nil
}

func NewClusterDescriber(clusterName string, stackName string, cpConfig *cp.Config, session *session.Session) ClusterDescriber {
	return clusterDescriberImpl{
		clusterName: clusterName,
		stackName:   stackName,
		cpConfig:    cpConfig,
		session:     session,
	}
}

func (c clusterDescriberImpl) Info() (*Info, error) {
	cfSvc := cloudformation.New(c.session)

	var cpStackName string
	{
		resp, err := cfSvc.DescribeStackResource(
			&cloudformation.DescribeStackResourceInput{
				LogicalResourceId: aws.String("Controlplane"),
				StackName:         aws.String(c.stackName),
			},
		)
		if err != nil {
			errmsg := "unable to get nested stack for control-plane:\n" + err.Error()
			return nil, fmt.Errorf(errmsg)
		}
		cpStackName = *resp.StackResourceDetail.PhysicalResourceId
	}

	var info Info
	{
		resp, err := cfSvc.DescribeStacks(&cloudformation.DescribeStacksInput{
			StackName: aws.String(cpStackName),
		})
		if err != nil {
			return nil, fmt.Errorf("error describing stack %s: %v", cpStackName, err)
		}
		if len(resp.Stacks) == 0 {
			return nil, fmt.Errorf("could not find a stack with name %s", cpStackName)
		}
		if len(resp.Stacks) > 1 {
			return nil, fmt.Errorf("found multiple load balancers with name %s: %v", cpStackName, resp)
		}

		cpDescriber := cluster.NewClusterDescriber(c.clusterName, cpStackName, c.cpConfig.ManagedELBLogicalNames(), c.cpConfig.ManagedALBLogicalNames(), c.session)

		cpInfo, err := cpDescriber.Info()

		if err != nil {
			return nil, fmt.Errorf("error describing stack %s: %v", cpStackName, err)
		}

		info.ControlPlane = cpInfo
	}

	return &info, nil
}
