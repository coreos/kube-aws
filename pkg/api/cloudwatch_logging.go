package api

type SystemdMessageResponse struct {
	InstanceId  string `json:"instanceId,omitempty"`
	Hostname    string `json:"hostname,omitempty"`
	CmdName     string `json:"cmdName,omitempty"`
	Exe         string `json:"exe,omitempty"`
	CmdLine     string `json:"cmdLine,omitempty"`
	SystemdUnit string `json:"systemdUnit,omitempty"`
	Priority    string `json:"priority,omitempty"`
	Message     string `json:"message,omitempty"`
}
