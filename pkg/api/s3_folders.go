package api

import (
	"fmt"
	"strings"
)

type S3Folders struct {
	clusterName string
	s3URI       string
}

func NewS3Folders(s3URI string, clusterName string) S3Folders {
	return S3Folders{
		s3URI:       s3URI,
		clusterName: clusterName,
	}
}

func (n S3Folders) root() S3Folder {
	return newS3Folder(n.s3URI)
}

func (n S3Folders) Cluster() S3Folder {
	return n.root().subFolder(fmt.Sprintf("kube-aws/clusters/%s", n.clusterName))
}

func (n S3Folders) ClusterBackups() S3Folder {
	return n.Cluster().subFolder("backup")
}

func (n S3Folders) ClusterExportedStacks() S3Folder {
	return n.Cluster().subFolder("exported/stacks")
}

type S3Folder struct {
	s3URI string
}

func newS3Folder(uri string) S3Folder {
	return S3Folder{
		s3URI: strings.TrimSuffix(uri, "/"),
	}
}

func (f S3Folder) Path() string {
	uri := strings.TrimSuffix(f.s3URI, "/")
	return strings.TrimPrefix(uri, "s3://")
}

func (f S3Folder) URI() string {
	return f.s3URI
}

func (f S3Folder) subFolder(name string) S3Folder {
	return newS3Folder(fmt.Sprintf("%s/%s", f.s3URI, name))
}
