package cluster

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/elb"
)

type ClusterDescriber interface {
	Info() (*Info, error)
}

type clusterDescriberImpl struct {
	session     *session.Session
	clusterName string
	stackName   string
}

func NewClusterDescriber(clusterName string, stackName string, session *session.Session) ClusterDescriber {
	return clusterDescriberImpl{
		clusterName: clusterName,
		stackName:   stackName,
		session:     session,
	}
}

func (c clusterDescriberImpl) Info() (*Info, error) {
	var elbName string
	{
		cfSvc := cloudformation.New(c.session)
		resp, err := cfSvc.DescribeStackResource(
			&cloudformation.DescribeStackResourceInput{
				LogicalResourceId: aws.String("ElbAPIServer"),
				StackName:         aws.String(c.stackName),
			},
		)
		if err != nil {
			errmsg := "unable to get public IP of controller instance:\n" + err.Error()
			return nil, fmt.Errorf(errmsg)
		}
		elbName = *resp.StackResourceDetail.PhysicalResourceId
	}

	elbSvc := elb.New(c.session)

	var info Info
	{
		resp, err := elbSvc.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{
			LoadBalancerNames: []*string{
				aws.String(elbName),
			},
			PageSize: aws.Int64(2),
		})
		if err != nil {
			return nil, fmt.Errorf("error describing load balancer %s: %v", elbName, err)
		}
		if len(resp.LoadBalancerDescriptions) == 0 {
			return nil, fmt.Errorf("could not find a load balancer with name %s", elbName)
		}
		if len(resp.LoadBalancerDescriptions) > 1 {
			return nil, fmt.Errorf("found multiple load balancers with name %s: %v", elbName, resp)
		}

		info.Name = c.clusterName
		info.ControllerHost = *resp.LoadBalancerDescriptions[0].DNSName
	}
	return &info, nil
}
