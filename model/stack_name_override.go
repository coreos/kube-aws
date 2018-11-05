package model

type StackNameOverride struct {
	ControlPlane string `yaml:"controlPlane,omitempty"`
	Network      string `yaml:"network,omitempty"`
	Etcd         string `yaml:"etcd,omitempty"`
}
