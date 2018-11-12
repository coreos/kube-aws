package api

type NodeSettings struct {
	FeatureGates FeatureGates `yaml:"featureGates"`
	NodeLabels   NodeLabels   `yaml:"nodeLabels"`
	Taints       Taints       `yaml:"taints"`
}

func newNodeSettings() NodeSettings {
	return NodeSettings{
		FeatureGates: FeatureGates{},
		NodeLabels:   NodeLabels{},
		Taints:       Taints{},
	}
}

func (s NodeSettings) Validate() error {
	if err := s.Taints.Validate(); err != nil {
		return err
	}
	return nil
}
