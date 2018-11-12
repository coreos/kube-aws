package api

type MOTDBanner struct {
	Enabled          bool        `yaml:"enabled,omitempty"`
	EtcdColour       ShellColour `yaml:"etcd-colour,omitempty"`
	KubernetesColour ShellColour `yaml:"kubernetes-colour,omitempty"`
	KubeAWSColour    ShellColour `yaml:"kube-aws-colour,omitempty"`
}

func NewDefaultMOTDBanner() MOTDBanner {
	return MOTDBanner{
		Enabled:          true,
		EtcdColour:       LightGreen,
		KubernetesColour: LightBlue,
		KubeAWSColour:    LightBlue,
	}
}
