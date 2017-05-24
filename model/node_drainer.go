package model

import (
	"fmt"
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

func (nd *NodeDrainer) Valid() error {
	if nd.DrainTimeout < 0 || nd.DrainTimeout > 60 {
		return fmt.Errorf("Drain timeout must be an integer between 0 and 60, but was %d", nd.DrainTimeout)
	}
	if nd.DrainInterval < 0 || nd.DrainInterval > 60 {
		return fmt.Errorf("Drain interval must be an integer between 0 and 60, but was %d", nd.DrainInterval)
	}
	return nil
}
