package provisioner

import (
	"encoding/base64"
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
)

func (c Content) String() string {
	if len(c.str) == 0 && len(c.bytes) > 0 {
		return string(c.bytes)
	}
	return c.str
}

func (c *Content) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tmp string
	if err := unmarshal(&tmp); err != nil {
		return err
	}
	*c = NewStringContent(tmp)
	return nil
}

func (c Content) MarshalYAML() (interface{}, error) {
	return c.String(), nil
}

func NewStringContent(str string) Content {
	return Content{
		bytes: []byte(str),
		str:   str,
	}
}

func NewBinaryContent(bytes []byte) Content {
	return Content{
		bytes: bytes,
	}
}

func (c Content) ToBase64() Content {
	bytes := []byte(base64.StdEncoding.EncodeToString(c.bytes))
	return Content{
		bytes: bytes,
	}
}

func (c Content) ToGzip() Content {
	bytes, err := gzipcompressor.BytesToGzippedBytes(c.bytes)
	if err != nil {
		panic(fmt.Errorf("Unexpected error in ToGzip: %v", err))
	}
	return Content{
		bytes: bytes,
	}
}

func (c Content) GzippedBase64Content() string {
	out, err := gzipcompressor.BytesToGzippedBase64String(c.bytes)
	if err != nil {
		return ""
	}
	return out
}
