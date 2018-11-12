package api

import (
	"fmt"
	"time"
)

type NodeDrainer struct {
	Enabled      bool    `yaml:"enabled"`
	DrainTimeout int     `yaml:"drainTimeout"`
	IAMRole      IAMRole `yaml:"iamRole,omitempty"`
}

func (nd *NodeDrainer) DrainTimeoutInSeconds() int {
	return int((time.Duration(nd.DrainTimeout) * time.Minute) / time.Second)
}

func (nd *NodeDrainer) Validate() error {
	if !nd.Enabled {
		return nil
	}

	if nd.DrainTimeout < 1 || nd.DrainTimeout > 60 {
		return fmt.Errorf("Drain timeout must be an integer between 1 and 60, but was %d", nd.DrainTimeout)
	}

	return nil
}
