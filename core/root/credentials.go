package root

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/core/controlplane/config"
	"github.com/kubernetes-incubator/kube-aws/core/root/defaults"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/tlscerts"
	"github.com/kubernetes-incubator/kube-aws/tlsutil"
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

func LoadCertificates() (map[string]tlscerts.Certificates, error) {

	if _, err := os.Stat(defaults.AssetsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist, run 'render credentials' first", defaults.AssetsDir)
	}

	files, err := ioutil.ReadDir(defaults.AssetsDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read files from %s: %v", defaults.AssetsDir, err)
	}

	certs := make(map[string]tlscerts.Certificates)
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".pem") {
			continue
		}
		b, err := ioutil.ReadFile(path.Join(defaults.AssetsDir, f.Name()))
		if err != nil {
			logger.Warnf("cannot read %q file: %v", f.Name(), err)
			continue
		}
		if !tlsutil.IsCertificatePEM(b) {
			continue
		}
		c, err := tlscerts.FromBytes(b)
		if err != nil {
			logger.Warnf("cannot parse %q file: %v", f.Name(), err)
			continue
		}
		certs[f.Name()] = c
	}
	return certs, nil
}
