package contents

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/kubernetes-incubator/kube-aws/plugin/api"
)

type Loader struct {
	p *api.Plugin
}

func LoaderFor(p *api.Plugin) *Loader {
	return &Loader{
		p: p,
	}
}

func (l *Loader) StringFrom(contents api.Contents) (string, error) {
	if contents.Inline != "" {
		return contents.Inline, nil
	}

	if contents.Path != "" {
		path := filepath.Join("plugins", l.p.Name, contents.Path)
		raw, err := ioutil.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to load %s: %v", path, err)
		}
		return string(raw), nil
	}

	return "", fmt.Errorf("failed to load string from %v: either `inline` or `path` must be specified but both of these were missing", contents)
}
