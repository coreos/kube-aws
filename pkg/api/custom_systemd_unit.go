package api

import (
	"strconv"
	"strings"
)

type CustomSystemdUnit struct {
	Name        string                    `yaml:"name"`
	Command     string                    `yaml:"command,omitempty"`
	Content     string                    `yaml:"content,omitempty"`
	Enable      bool                      `yaml:"enable,omitempty"`
	Runtime     bool                      `yaml:"runtime,omitempty"`
	DropIns     []CustomSystemdUnitDropIn `yaml:"drop-ins,omitempty"`
	UnknownKeys `yaml:",inline"`
}

func (c CustomSystemdUnit) ContentPresent() bool {
	if len(c.Content) > 0 {
		return true
	}
	return false
}

func (c CustomSystemdUnit) DropInsPresent() bool {
	if len(c.DropIns) > 0 {
		return true
	}
	return false
}

func (c CustomSystemdUnit) ContentArray() []string {
	trimmedContent := strings.TrimRight(c.Content, "\n")
	return strings.Split(trimmedContent, "\n")
}

func (c CustomSystemdUnit) EnableString() string {
	return strconv.FormatBool(c.Enable)
}

func (c CustomSystemdUnit) RuntimeString() string {
	return strconv.FormatBool(c.Runtime)
}
