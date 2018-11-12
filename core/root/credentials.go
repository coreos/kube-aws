package root

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/core/root/defaults"
	"github.com/kubernetes-incubator/kube-aws/credential"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/pki"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

func RenderCredentials(configPath string, renderCredentialsOpts credential.GeneratorOptions) error {
	opts := NewOptions(false, false)
	cluster, err := CompileClusterFromFile(configPath, opts, renderCredentialsOpts.AwsDebug)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(defaults.AssetsDir, 0700); err != nil {
		return err
	}

	if _, err = cluster.GenerateAssetsOnDisk(defaults.AssetsDir, renderCredentialsOpts); err != nil {
		return err
	}

	return nil
}

func LoadCertificates() (map[string]pki.Certificates, error) {

	if _, err := os.Stat(defaults.AssetsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist, run 'render credentials' first", defaults.AssetsDir)
	}

	files, err := ioutil.ReadDir(defaults.AssetsDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read files from %s: %v", defaults.AssetsDir, err)
	}

	certs := make(map[string]pki.Certificates)
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".pem") {
			continue
		}
		b, err := ioutil.ReadFile(path.Join(defaults.AssetsDir, f.Name()))
		if err != nil {
			logger.Warnf("cannot read %q file: %v", f.Name(), err)
			continue
		}
		if !pki.IsCertificatePEM(b) {
			continue
		}
		c, err := pki.CertificatesFromBytes(b)
		if err != nil {
			logger.Warnf("cannot parse %q file: %v", f.Name(), err)
			continue
		}
		certs[f.Name()] = c
	}
	return certs, nil
}
