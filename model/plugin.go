package model

type PluginConfigs map[string]PluginConfig

type PluginConfig struct {
	Enabled bool `yaml:"enabled,omitempty"`
	Values  `yaml:",inline"`
}

type Values map[string]interface{}
