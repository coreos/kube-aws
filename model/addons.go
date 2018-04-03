package model

type Addons struct {
	Rescheduler       Rescheduler              `yaml:"rescheduler"`
	ClusterAutoscaler ClusterAutoscalerSupport `yaml:"clusterAutoscaler,omitempty"`
	MetricsServer     MetricsServer            `yaml:"metricsServer,omitempty"`
	Prometheus        Prometheus               `yaml:"prometheus"`
}

type ClusterAutoscalerSupport struct {
	Enabled bool `yaml:"enabled"`
}

type Rescheduler struct {
	Enabled bool `yaml:"enabled"`
}

type MetricsServer struct {
	Enabled bool `yaml:"enabled"`
}

type Prometheus struct {
	SecurityGroupsEnabled bool `yaml:"securityGroupsEnabled"`
}
