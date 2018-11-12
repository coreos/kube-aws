package api

import (
	"strings"
)

type CustomSystemdUnitDropIn struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

func (c CustomSystemdUnitDropIn) ContentArray() []string {
	trimmedContent := strings.TrimRight(c.Content, "\n")
	return strings.Split(trimmedContent, "\n")
}
