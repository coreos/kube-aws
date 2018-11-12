package api

type DataVolume struct {
	Size        int    `yaml:"size,omitempty"`
	Type        string `yaml:"type,omitempty"`
	IOPS        int    `yaml:"iops,omitempty"`
	Ephemeral   bool   `yaml:"ephemeral,omitempty"`
	Encrypted   bool   `yaml:"encrypted,omitempty"`
	UnknownKeys `yaml:",inline"`
}
