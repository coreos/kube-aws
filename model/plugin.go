package model

type Plugins map[string]Plugin

type Plugin struct {
	Enabled  bool `yaml:"enabled,omitempty"`
	Settings `yaml:",inline"`
}

type Settings map[string]interface{}
