package api

type KubernetesAuthentication struct {
	AWSIAM AWSIAM `yaml:"awsIAM"`
}

type AWSIAM struct {
	Enabled           bool   `yaml:"enabled"`
	BinaryDownloadURL string `yaml:"binaryDownloadURL"`
	ClusterID         string `yaml:"clusterID"`
}

func (a AWSIAM) BinaryStorePathes() []string {
	return []string{
		"files/worker/opt/bin/heptio-authenticator-aws",
		"files/controller/opt/bin/heptio-authenticator-aws",
	}
}
