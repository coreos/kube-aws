package defaults

const (
	AssetsDir                         = "credentials"
	ControllerTmplFile                = "userdata/cloud-config-controller"
	WorkerTmplFile                    = "userdata/cloud-config-worker"
	EtcdTmplFile                      = "userdata/cloud-config-etcd"
	ControlPlaneStackTemplateTmplFile = "stack-templates/control-plane.json.tmpl"
	NodePoolStackTemplateTmplFile     = "stack-templates/node-pool.json.tmpl"
	RootStackTemplateTmplFile         = "stack-templates/root.json.tmpl"
)
