package model

import (
	"time"
)

type NodeDrainer struct {
	Enabled       bool `yaml:"enabled"`
	DrainTimeout  int  `yaml:"drainTimeout"`
	DrainInterval int  `yaml:"drainInterval"`
}

func (nd *NodeDrainer) DrainTimeoutInSeconds() int {
	return int((time.Duration(nd.DrainTimeout) * time.Minute) / time.Second)
}

func (nd *NodeDrainer) DrainIntervalInSeconds() int {
	return int((time.Duration(nd.DrainInterval) * time.Minute) / time.Second)
}
