package root

import "github.com/kubernetes-incubator/kube-aws/core/root/defaults"

type options struct {
	AssetsDir                         string
	ControllerTmplFile                string
	WorkerTmplFile                    string
	EtcdTmplFile                      string
	RootStackTemplateTmplFile         string
	ControlPlaneStackTemplateTmplFile string
	NetworkStackTemplateTmplFile      string
	EtcdStackTemplateTmplFile         string
	NodePoolStackTemplateTmplFile     string
	AWSProfile                        string
	SkipWait                          bool
	PrettyPrint                       bool
}

func NewOptions(prettyPrint bool, skipWait bool, awsProfile ...string) options {
	var profile string
	if len(awsProfile) > 0 {
		profile = awsProfile[0]
	}

	return options{
		AssetsDir:                         defaults.AssetsDir,
		ControllerTmplFile:                defaults.ControllerTmplFile,
		WorkerTmplFile:                    defaults.WorkerTmplFile,
		EtcdTmplFile:                      defaults.EtcdTmplFile,
		ControlPlaneStackTemplateTmplFile: defaults.ControlPlaneStackTemplateTmplFile,
		NetworkStackTemplateTmplFile:      defaults.NetworkStackTemplateTmplFile,
		EtcdStackTemplateTmplFile:         defaults.EtcdStackTemplateTmplFile,
		NodePoolStackTemplateTmplFile:     defaults.NodePoolStackTemplateTmplFile,
		RootStackTemplateTmplFile:         defaults.RootStackTemplateTmplFile,
		AWSProfile:                        profile,
		SkipWait:                          skipWait,
		PrettyPrint:                       prettyPrint,
	}
}
