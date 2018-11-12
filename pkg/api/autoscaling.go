package api

type Autoscaling struct {
	ClusterAutoscaler ClusterAutoscaler `yaml:"clusterAutoscaler,omitempty"`
}
