package root

import (
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/awsconn"
	"github.com/kubernetes-incubator/kube-aws/cfnstack"
	"github.com/kubernetes-incubator/kube-aws/core/root/config"
)

type DestroyOptions struct {
	AwsDebug bool
	Force    bool
}

type ClusterDestroyer interface {
	Destroy() error
}

type clusterDestroyerImpl struct {
	underlying *cfnstack.Destroyer
}

func ClusterDestroyerFromFile(configPath string, opts DestroyOptions) (ClusterDestroyer, error) {
	cfg, err := config.ConfigFromFile(configPath)
	if err != nil {
		return nil, err
	}

	session, err := awsconn.NewSessionFromRegion(cfg.Region, opts.AwsDebug)
	if err != nil {
		return nil, fmt.Errorf("failed to establish aws session: %v", err)
	}

	cfnDestroyer := cfnstack.NewDestroyer(cfg.RootStackName(), session, cfg.CloudFormation.RoleARN)
	return clusterDestroyerImpl{
		underlying: cfnDestroyer,
	}, nil
}

func (d clusterDestroyerImpl) Destroy() error {
	return d.underlying.Destroy()
}
