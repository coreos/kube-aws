package api

type BashPrompt struct {
	Enabled           bool        `yaml:"enabled,omitempty"`
	IncludePWD        bool        `yaml:"include-pwd,omitempty"`
	IncludeHostname   bool        `yaml:"include-hostname,omitempty"`
	IncludeUser       bool        `yaml:"include-user,omitempty"`
	ClusterColour     ShellColour `yaml:"cluster-colour,omitempty"`
	Divider           string      `yaml:"divider,omitempty"`
	DividerColour     ShellColour `yaml:"divider-colour,omitempty"`
	EtcdLabel         string      `yaml:"etcd-label,omitempty"`
	EtcdColour        ShellColour `yaml:"etcd-colour,omitempty"`
	ControllerLabel   string      `yaml:"controller-label,omitempty"`
	ControllerColour  ShellColour `yaml:"controller-colour,omitempty"`
	WorkerLabel       string      `yaml:"worker-label,omitempty"`
	WorkerColour      ShellColour `yaml:"worker-colour,omitempty"`
	RootUserColour    ShellColour `yaml:"root-user-colour,omitempty"`
	NonRootUserColour ShellColour `yaml:"non-root-user-colour,omitempty"`
	DirectoryColour   ShellColour `yaml:"directory-colour,omitempty"`
}

func NewDefaultBashPrompt() BashPrompt {
	return BashPrompt{
		Enabled:           true,
		IncludePWD:        true,
		IncludeHostname:   true,
		IncludeUser:       true,
		ClusterColour:     LightCyan,
		Divider:           "|",
		DividerColour:     DefaultColour,
		EtcdLabel:         "etcd",
		EtcdColour:        LightGreen,
		ControllerLabel:   "master",
		ControllerColour:  LightRed,
		WorkerLabel:       "node",
		WorkerColour:      LightBlue,
		RootUserColour:    LightRed,
		NonRootUserColour: LightGreen,
		DirectoryColour:   LightBlue,
	}
}
