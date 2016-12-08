package config

// Configuration specific to auto scaling groups
type AutoScalingGroup struct {
	MinSize                            int `yaml:"minSize,omitempty"`
	MaxSize                            int `yaml:"maxSize,omitempty"`
	RollingUpdateMinInstancesInService int `yaml:"rollingUpdateMinInstancesInService,omitempty"`
}
