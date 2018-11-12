package api

type StackNameOverrides struct {
	ControlPlane string `yaml:"controlPlane,omitempty"`
	Network      string `yaml:"network,omitempty"`
	Etcd         string `yaml:"etcd,omitempty"`
}
