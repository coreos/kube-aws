package api

import "github.com/kubernetes-incubator/kube-aws/provisioner"

type HelmReleaseFileset struct {
	ValuesFile  *provisioner.RemoteFile
	ReleaseFile *provisioner.RemoteFile
}
