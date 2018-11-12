package model

import (
	"fmt"
	"github.com/go-yaml/yaml"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
	"io/ioutil"
)

func ClusterFromFile(filename string) (*api.Cluster, error) {
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
func ClusterFromBytes(data []byte) (*api.Cluster, error) {
	c := api.NewDefaultCluster()

	if err := yaml.Unmarshal(data, c); err != nil {
		return c, fmt.Errorf("failed to parse cluster: %v", err)
	}

	c.HyperkubeImage.Tag = c.K8sVer

	if err := c.Load(); err != nil {
		return c, err
	}

	return c, nil
}
