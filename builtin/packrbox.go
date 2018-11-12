package builtin

import "github.com/gobuffalo/packr"

var _box *packr.Box

const (
	AssetsDir                         = "credentials"
	ControllerTmplFile                = "userdata/cloud-config-controller"
	WorkerTmplFile                    = "userdata/cloud-config-worker"
	EtcdTmplFile                      = "userdata/cloud-config-etcd"
	ControlPlaneStackTemplateTmplFile = "stack-templates/control-plane.json.tmpl"
	NetworkStackTemplateTmplFile      = "stack-templates/network.json.tmpl"
	EtcdStackTemplateTmplFile         = "stack-templates/etcd.json.tmpl"
	NodePoolStackTemplateTmplFile     = "stack-templates/node-pool.json.tmpl"
	RootStackTemplateTmplFile         = "stack-templates/root.json.tmpl"
)

func Box() *packr.Box {
	if _box == nil {
		b := packr.NewBox("./files")
		_box = &b
	}
	return _box
}

func Bytes(path string) []byte {
	bytes, err := Box().MustBytes(path)
	if err != nil {
		panic(err)
	}
	return bytes
}

func MustBytes(path string) ([]byte, error) {
	return Box().MustBytes(path)
}

func String(path string) string {
	str, err := Box().MustString(path)
	if err != nil {
		panic(err)
	}
	return str
}
