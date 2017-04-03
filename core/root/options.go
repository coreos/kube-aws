package root

import "github.com/kubernetes-incubator/kube-aws/core/root/defaults"

type options struct {
	AssetsDir                         string
	ControllerTmplFile                string
	WorkerTmplFile                    string
	EtcdTmplFile                      string
	RootStackTemplateTmplFile         string
	ControlPlaneStackTemplateTmplFile string
	NodePoolStackTemplateTmplFile     string
	S3URI                             string
	SkipWait                          bool
	PrettyPrint                       bool
}

func NewOptions(s3URI string, prettyPrint bool, skipWait bool) options {
	return options{
		AssetsDir:                         defaults.AssetsDir,
		ControllerTmplFile:                defaults.ControllerTmplFile,
		WorkerTmplFile:                    defaults.WorkerTmplFile,
		EtcdTmplFile:                      defaults.EtcdTmplFile,
		ControlPlaneStackTemplateTmplFile: defaults.ControlPlaneStackTemplateTmplFile,
		NodePoolStackTemplateTmplFile:     defaults.NodePoolStackTemplateTmplFile,
		RootStackTemplateTmplFile:         defaults.RootStackTemplateTmplFile,
		S3URI:       s3URI,
		SkipWait:    skipWait,
		PrettyPrint: prettyPrint,
	}
}
