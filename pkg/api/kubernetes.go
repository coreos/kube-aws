package api

type Kubernetes struct {
	Authentication    KubernetesAuthentication `yaml:"authentication"`
	EncryptionAtRest  EncryptionAtRest         `yaml:"encryptionAtRest"`
	Networking        Networking               `yaml:"networking,omitempty"`
	ControllerManager ControllerManager        `yaml:"controllerManager,omitempty"`
	KubeScheduler     KubeScheduler            `yaml:"kubeScheduler,omitempty"`
	KubeProxy         KubeProxy                `yaml:"kubeProxy,omitempty"`
	KubeApiServer     KubeApiServer            `yaml:"apiServer,omitempty"`
	Kubelet           Kubelet                  `yaml:"kubelet,omitempty"`
	APIServer         KubernetesAPIServer      `yaml:"apiserver,omitempty"`

	// Manifests is a list of manifests to be installed to the cluster.
	// Note that the list is sorted by their names by kube-aws so that it won't result in unnecessarily node replacements.
	Manifests KubernetesManifests `yaml:"manifests,omitempty"`
}

type KubeApiServer struct {
	ComputeResources ComputeResources `yaml:"resources,omitempty"`
	TargetRamMb      int              `yaml:"targetRamMb,omitempty"`
}

type ControllerManager struct {
	ComputeResources ComputeResources `yaml:"resources,omitempty"`
	Flags            CommandLineFlags `yaml:"flags,omitempty"`
}

type KubeScheduler struct {
	ComputeResources ComputeResources `yaml:"resources,omitempty"`
	Flags            CommandLineFlags `yaml:"flags,omitempty"`
}

type ComputeResources struct {
	Requests ResourceQuota `yaml:"requests,omitempty"`
	Limits   ResourceQuota `yaml:"limits,omitempty"`
}

type ResourceQuota struct {
	Cpu    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}
