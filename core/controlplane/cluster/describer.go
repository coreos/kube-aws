package cluster

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elbv2"
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
	// For ELBV1 API objects
	elbNameRefs := []*string{}
	elbNames := []string{}

	// For ELBV2 API objects
	elbv2NameRefs := []*string{}
	elbv2Names := []string{}

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

			// ELBV2 uses ARN identifiers
			if strings.HasPrefix(*resp.StackResourceDetail.PhysicalResourceId, "arn:") {
				elbv2NameRefs = append(elbv2NameRefs, resp.StackResourceDetail.PhysicalResourceId)
				elbv2Names = append(elbv2Names, *resp.StackResourceDetail.PhysicalResourceId)
			} else {
				elbNameRefs = append(elbNameRefs, resp.StackResourceDetail.PhysicalResourceId)
				elbNames = append(elbNames, *resp.StackResourceDetail.PhysicalResourceId)
			}
		}
	}

	elbSvc := elb.New(c.session)
	elbv2Svc := elbv2.New(c.session)

	var info Info
	{
		dnsNames := []string{}

		// Use the proper API to describe classic ELB objects
		if len(elbNameRefs) > 0 {
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
			for _, d := range resp.LoadBalancerDescriptions {
				dnsNames = append(dnsNames, *d.DNSName)
			}
		}

		// Use the proper API to describe ELBV2 API objects
		if len(elbv2NameRefs) > 0 {
			respv2, err := elbv2Svc.DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
				LoadBalancerArns: elbv2NameRefs,
			})
			if err != nil {
				return nil, fmt.Errorf("error describing load balancers %v: %v", elbv2Names, err)
			}
			if len(respv2.LoadBalancers) == 0 {
				return nil, fmt.Errorf("could not find load balancers with names %v", elbv2Names)
			}
			for _, d := range respv2.LoadBalancers {
				dnsNames = append(dnsNames, *d.DNSName)
			}
		}

		info.Name = c.clusterName
		info.ControllerHosts = dnsNames
	}
	return &info, nil
}
