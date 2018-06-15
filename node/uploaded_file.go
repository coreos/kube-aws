package node

import (
	"encoding/base64"
	"fmt"

	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
)

type UploadedFile struct {
	Path    string
	Content uploadedFileContent
}

type uploadedFileContent struct {
	bytes []byte
}

func NewUploadedFileContent(bytes []byte) uploadedFileContent {
	return uploadedFileContent{
		bytes: bytes,
	}
}

func (c uploadedFileContent) ToBase64() uploadedFileContent {
	bytes := []byte(base64.StdEncoding.EncodeToString(c.bytes))
	return uploadedFileContent{
		bytes: bytes,
	}
}

func (c uploadedFileContent) ToGzip() uploadedFileContent {
	bytes, err := gzipcompressor.BytesToBytes(c.bytes)
	if err != nil {
		panic(fmt.Errorf("Unexpected error in ToGzip: %v", err))
	}
	return uploadedFileContent{
		bytes: bytes,
	}
}

func (c uploadedFileContent) String() string {
	return string(c.bytes)
}
