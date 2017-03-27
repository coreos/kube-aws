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
	clusterName             string
	elbResourceLogicalNames []string
	session                 *session.Session
	stackName               string
}

func NewClusterDescriber(clusterName string, stackName string, elbResourceLogicalNames []string, session *session.Session) ClusterDescriber {
	return clusterDescriberImpl{
		clusterName:             clusterName,
		elbResourceLogicalNames: elbResourceLogicalNames,
		stackName:               stackName,
		session:                 session,
	}
}

func (c clusterDescriberImpl) Info() (*Info, error) {
	elbNameRefs := []*string{}
	elbNames := []string{}
	{
		cfSvc := cloudformation.New(c.session)
		for _, lb := range c.elbResourceLogicalNames {
			resp, err := cfSvc.DescribeStackResource(
				&cloudformation.DescribeStackResourceInput{
					LogicalResourceId: aws.String(lb),
					StackName:         aws.String(c.stackName),
				},
			)
			if err != nil {
				errmsg := "unable to get public IP of controller instance:\n" + err.Error()
				return nil, fmt.Errorf(errmsg)
			}
			elbNameRefs = append(elbNameRefs, resp.StackResourceDetail.PhysicalResourceId)
			elbNames = append(elbNames, *resp.StackResourceDetail.PhysicalResourceId)
		}
	}

	elbSvc := elb.New(c.session)

	var info Info
	{
		resp, err := elbSvc.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{
			LoadBalancerNames: elbNameRefs,
			PageSize:          aws.Int64(2),
		})
		if err != nil {
			return nil, fmt.Errorf("error describing load balancers %v: %v", elbNames, err)
		}
		if len(resp.LoadBalancerDescriptions) == 0 {
			return nil, fmt.Errorf("could not find load balancers with names %v", elbNames)
		}

		dnsNames := []string{}
		for _, d := range resp.LoadBalancerDescriptions {
			dnsNames = append(dnsNames, *d.DNSName)
		}

		info.Name = c.clusterName
		info.ControllerHosts = dnsNames
	}
	return &info, nil
}
