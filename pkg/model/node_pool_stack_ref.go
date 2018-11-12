package model

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/kubernetes-incubator/kube-aws/cfnstack"
)

type ec2DescribeKeyPairsService interface {
	DescribeKeyPairs(*ec2.DescribeKeyPairsInput) (*ec2.DescribeKeyPairsOutput, error)
}

type ec2CreateVolumeService interface {
	CreateVolume(*ec2.CreateVolumeInput) (*ec2.Volume, error)
}

type NodePoolStackRef struct {
	*NodePoolConfig
	session *session.Session
}

func newNodePoolStackRef(cfg *NodePoolConfig, session *session.Session) *NodePoolStackRef {
	return &NodePoolStackRef{
		NodePoolConfig: cfg,
		session:        session,
	}
}

func (c *NodePoolStackRef) validateKeyPair(ec2Svc ec2DescribeKeyPairsService) error {
	_, err := ec2Svc.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{
		KeyNames: []*string{aws.String(c.KeyName)},
	})

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "InvalidKeyPair.NotFound" {
				return fmt.Errorf("Key %s does not exist.", c.KeyName)
			}
		}
		return err
	}
	return nil
}

func (c *NodePoolStackRef) validateWorkerRootVolume(ec2Svc ec2CreateVolumeService) error {

	//Send a dry-run request to validate the worker root volume parameters
	workerRootVolume := c.getWorkerRootVolumeConfig()

	if _, err := ec2Svc.CreateVolume(workerRootVolume); err != nil {
		operr, ok := err.(awserr.Error)

		if !ok || (ok && operr.Code() != "DryRunOperation") {
			return fmt.Errorf("create volume dry-run request failed: %v", err)
		}
	}

	return nil
}

func (c *NodePoolStackRef) getWorkerRootVolumeConfig() *ec2.CreateVolumeInput {
	var workerRootVolume = &ec2.CreateVolumeInput{}

	switch c.RootVolume.Type {
	case "standard", "gp2":
		workerRootVolume = &ec2.CreateVolumeInput{
			DryRun:           aws.Bool(true),
			AvailabilityZone: aws.String(c.Subnets[0].AvailabilityZone),
			Size:             aws.Int64(int64(c.RootVolume.Size)),
			VolumeType:       aws.String(c.RootVolume.Type),
		}
	case "io1":
		workerRootVolume = &ec2.CreateVolumeInput{
			DryRun:           aws.Bool(true),
			AvailabilityZone: aws.String(c.Subnets[0].AvailabilityZone),
			Iops:             aws.Int64(int64(c.RootVolume.IOPS)),
			Size:             aws.Int64(int64(c.RootVolume.Size)),
			VolumeType:       aws.String(c.RootVolume.Type),
		}
	default:
		workerRootVolume = &ec2.CreateVolumeInput{
			DryRun:           aws.Bool(true),
			AvailabilityZone: aws.String(c.Subnets[0].AvailabilityZone),
			Size:             aws.Int64(int64(c.RootVolume.Size)),
			VolumeType:       aws.String(c.RootVolume.Type),
		}
	}

	return workerRootVolume
}

func (c *NodePoolStackRef) Info() (*NodePoolStackInfo, error) {
	var info NodePoolStackInfo
	{
		info.Name = c.NodePoolName
	}
	return &info, nil
}

func (c *NodePoolStackRef) Destroy() error {
	return cfnstack.NewDestroyer(c.StackName(), c.session, c.CloudFormation.RoleARN).Destroy()
}
