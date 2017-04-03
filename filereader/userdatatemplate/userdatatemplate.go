package userdatatemplate

import (
	"github.com/kubernetes-incubator/kube-aws/filereader/texttemplate"
)

func GetString(filename string, data interface{}) (string, error) {
	buf, err := texttemplate.GetBytesBuffer(filename, data)

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
