package model

type Elastigroup struct {
	AccessToken         string `yaml:"accessToken,omitempty"`
	AccountId           string `yaml:"accountId,omitempty"`
	MinSize             int    `yaml:"minSize,omitempty"`
	MaxSize             int    `yaml:"maxSize,omitempty"`
	TargetSize          int    `yaml:"targetSize,omitempty"`
	Risk                int    `yaml:"risk,omitempty"`
	OnDemandCount       int    `yaml:"onDemandCount,omitempty"`
	AvalilabilityVsCost string `yaml:"availabilityVsCost,omitempty"`
	DrainingTimeout     int    `yaml:"drainingTimeout,omitempty"`
	FallbackToOd        bool   `yaml:"fallbackToOd,omitempty"`
	LifetimePeriod      string `yaml:"lifetimePeriod,omitempty"`
	RevertToSpot        string `yaml:"revertToSpot,omitempty"`
	SignalsTimeout      int    `yaml:"signalsTimeout,omitempty"`
	ElbType             string `yaml:"elbType,omitempty"`
}

func (f Elastigroup) Enabled() bool {
	return f.TargetSize > 0
}
