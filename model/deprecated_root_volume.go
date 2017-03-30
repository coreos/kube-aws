package model

type DeprecatedRootVolume struct {
	DeprecatedRootVolumeType *string `yaml:"rootVolumeType,omitempty"`
	DeprecatedRootVolumeIOPS *int    `yaml:"rootVolumeIOPS,omitempty"`
	DeprecatedRootVolumeSize *int    `yaml:"rootVolumeSize,omitempty"`
}
