package model

type Addons struct {
	Rescheduler         Rescheduler              `yaml:"rescheduler"`
	ClusterAutoscaler   ClusterAutoscalerSupport `yaml:"clusterAutoscaler,omitempty"`
	MetricsServer       MetricsServer            `yaml:"metricsServer,omitempty"`
	Prometheus          Prometheus               `yaml:"prometheus"`
	APIServerAggregator APIServerAggregator      `yaml:"apiserverAggregator"`
	UnknownKeys         `yaml:",inline"`
}

type ClusterAutoscalerSupport struct {
	Enabled          bool              `yaml:"enabled"`
	ComputeResources ComputeResources  `yaml:"resources,omitempty"`
	Options          map[string]string `yaml:"options"`
	UnknownKeys      `yaml:",inline"`
}

type Rescheduler struct {
	Enabled     bool `yaml:"enabled"`
	UnknownKeys `yaml:",inline"`
}

type MetricsServer struct {
	Enabled     bool `yaml:"enabled"`
	UnknownKeys `yaml:",inline"`
}

type Prometheus struct {
	SecurityGroupsEnabled bool `yaml:"securityGroupsEnabled"`
	UnknownKeys           `yaml:",inline"`
}

type APIServerAggregator struct {
	Enabled bool `yaml:"enabled"`
}

type ComputeResources struct {
	Limits   ResourceQuota `yaml:"limits,omitempty"`
	Requests ResourceQuota `yaml:"requests,omitempty"`
}

type ResourceQuota struct {
	Cpu    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}
