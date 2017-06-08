package cluster

import (
	"fmt"
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
	clusterName                    string
	classicElbResourceLogicalNames []string
	elbResourceLogicalNames        []string
	session                        *session.Session
	stackName                      string
}

func NewClusterDescriber(clusterName string, stackName string, classicElbResourceLogicalNames []string, elbResourceLogicalNames []string, session *session.Session) ClusterDescriber {
	return clusterDescriberImpl{
		clusterName:                    clusterName,
		classicElbResourceLogicalNames: classicElbResourceLogicalNames,
		elbResourceLogicalNames:        elbResourceLogicalNames,
		stackName:                      stackName,
		session:                        session,
	}
}

func (c clusterDescriberImpl) Info() (*Info, error) {
	classicElbNameRefs := []*string{}
	classicElbNames := []string{}
	{
		cfSvc := cloudformation.New(c.session)
		for _, lb := range c.classicElbResourceLogicalNames {
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
			elbName := *resp.StackResourceDetail.PhysicalResourceId
			classicElbNameRefs = append(classicElbNameRefs, &elbName)
			classicElbNames = append(classicElbNames, elbName)
		}
	}

	albArnRefs := []*string{}
	albArns := []string{}
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
			albArn := *resp.StackResourceDetail.PhysicalResourceId
			albArnRefs = append(albArnRefs, &albArn)
			albArns = append(albArns, albArn)
		}
	}

	var info Info
	{
		dnsNames := []string{}

		if len(classicElbNames) > 0 {
			elbSvc := elb.New(c.session)
			resp, err := elbSvc.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{
				LoadBalancerNames: classicElbNameRefs,
				PageSize:          aws.Int64(2),
			})
			if err != nil {
				return nil, fmt.Errorf("error describing load balancers %v: %v", classicElbNames, err)
			}
			if len(resp.LoadBalancerDescriptions) == 0 {
				return nil, fmt.Errorf("could not find load balancers with names %v", classicElbNames)
			}

			for _, d := range resp.LoadBalancerDescriptions {
				dnsNames = append(dnsNames, *d.DNSName)
			}
		}

		if len(albArns) > 0 {
			elbv2Svc := elbv2.New(c.session)
			resp, err := elbv2Svc.DescribeLoadBalancers(&elbv2.DescribeLoadBalancersInput{
				LoadBalancerArns: albArnRefs,
			})
			if err != nil {
				return nil, fmt.Errorf("error describing application load balancers %v: %v", albArns, err)
			}
			if len(resp.LoadBalancers) == 0 {
				return nil, fmt.Errorf("could not find appilcation load balancers with names %v", albArns)
			}

			for _, d := range resp.LoadBalancers {
				dnsNames = append(dnsNames, *d.DNSName)
			}
		}

		info.Name = c.clusterName
		info.ControllerHosts = dnsNames
	}
	return &info, nil
}
