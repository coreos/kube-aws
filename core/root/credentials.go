package root

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/cert"
	"github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/defaults"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func RenderCredentials(configPath string, renderCredentialsOpts config.CredentialsOptions) error {

	cluster, err := config.ClusterFromFile(configPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(defaults.AssetsDir, 0700); err != nil {
		return err
	}

	_, err = cluster.NewAssetsOnDisk(defaults.AssetsDir, renderCredentialsOpts)
	return err
}

func LoadCertificates() (map[string][]cert.Certificate, error) {

	if _, err := os.Stat(defaults.AssetsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist, run 'render credentials' first", defaults.AssetsDir)
	}

	files, err := ioutil.ReadDir(defaults.AssetsDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read files from %s: %v", defaults.AssetsDir, err)
	}

	certs := make(map[string][]cert.Certificate)
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".pem") {
			continue
		}
		b, err := ioutil.ReadFile(path.Join(defaults.AssetsDir, f.Name()))
		if err != nil {
			return nil, fmt.Errorf("cannot read %s file: %v", f.Name(), err)
		}
		if !cert.IsCertificate(b) {
			continue
		}
		c, err := cert.ParseCertificates(b)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %s file: %v", f.Name(), err)
		}
		certs[f.Name()] = c
	}
	return certs, nil
}
