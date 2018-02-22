package model

type EC2Instance struct {
	Count         int    `yaml:"count,omitempty"`
	CreateTimeout string `yaml:"createTimeout,omitempty"`
	InstanceType  string `yaml:"instanceType,omitempty"`
	RootVolume    `yaml:"rootVolume,omitempty"`
	Tenancy       string            `yaml:"tenancy,omitempty"`
	InstanceTags  map[string]string `yaml:"instanceTags,omitempty"`
}
