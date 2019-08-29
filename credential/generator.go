package credential

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/netutil"
	"github.com/kubernetes-incubator/kube-aws/pki"
)

type Generator struct {
	TLSCADurationDays                int
	TLSCertDurationDays              int
	TLSBootstrapEnabled              bool
	ManageCertificates               bool
	Region                           string
	APIServerExternalDNSNames        []string
	APIServerAdditionalDNSSans       []string
	APIServerAdditionalIPAddressSans []string
	EtcdNodeDNSNames                 []string
	ServiceCIDR                      string
}

type GeneratorOptions struct {
	AwsDebug   bool
	GenerateCA bool
	CaCertPath string
	CommonName string
	// Paths for private certificate keys.
	AdminKeyPath                 string
	ApiServerAggregatorKeyPath   string
	ApiServerKeyPath             string
	CaKeyPath                    string
	EtcdClientKeyPath            string
	EtcdKeyPath                  string
	KubeControllerManagerKeyPath string
	KubeSchedulerKeyPath         string
	ServiceAccountKeyPath        string
	WorkerKeyPath                string
}

func (c Generator) GenerateAssetsOnDisk(dir string, o GeneratorOptions) (*RawAssetsOnDisk, error) {
	logger.Info("Generating credentials...")
	var caKey *rsa.PrivateKey
	var caCert *x509.Certificate
	if o.GenerateCA {
		var err error
		caKey, caCert, err = pki.NewCA(c.TLSCADurationDays, o.CommonName)
		if err != nil {
			return nil, fmt.Errorf("failed generating cluster CA: %v", err)
		}
		logger.Info("-> Generating new TLS CA\n")
	} else {
		logger.Info("-> Parsing existing TLS CA\n")
		if caKeyBytes, err := ioutil.ReadFile(o.CaKeyPath); err != nil {
			return nil, fmt.Errorf("failed reading ca key file %s : %v", o.CaKeyPath, err)
		} else {
			if caKey, err = pki.DecodePrivateKeyPEM(caKeyBytes); err != nil {
				return nil, fmt.Errorf("failed parsing ca key: %v", err)
			}
		}
		if caCertBytes, err := ioutil.ReadFile(o.CaCertPath); err != nil {
			return nil, fmt.Errorf("failed reading ca cert file %s : %v", o.CaCertPath, err)
		} else {
			if caCert, err = pki.DecodeCertificatePEM(caCertBytes); err != nil {
				return nil, fmt.Errorf("failed parsing ca cert: %v", err)
			}
		}
	}

	logger.Info("-> Generating new assets")
	assets, err := c.GenerateAssetsOnMemory(caKey, caCert, o)
	if err != nil {
		return nil, fmt.Errorf("Error generating default assets: %v", err)
	}

	logger.Infof("--> Summarizing the configuration\n    TLS certificates managed by kube-aws=%v, CA key required on controller nodes=%v\n", c.ManageCertificates, true)

	logger.Info("--> Writing to the storage")
	alsoWriteCAKey := o.GenerateCA || c.ManageCertificates
	if err := assets.WriteToDir(dir, alsoWriteCAKey); err != nil {
		return nil, fmt.Errorf("Error creating assets: %v", err)
	}

	{
		logger.Info("--> Verifying the result")
		verified, err := ReadRawAssets(dir, c.ManageCertificates, c.ManageCertificates)

		if err != nil {
			return nil, fmt.Errorf("failed verifying the result: %v", err)
		}

		return verified, nil
	}
}

func getOrCreatePrivateKey(keyPath string) (*rsa.PrivateKey, error) {
	if keyPath != "" {
		keyBytes, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed reading key file %s : %v", keyPath, err)
		}

		key, err := pki.DecodePrivateKeyPEM(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed parsing key: %v", err)
		}
		return key, nil
	}
	return pki.NewPrivateKey()
}

func (c Generator) GenerateAssetsOnMemory(caKey *rsa.PrivateKey, caCert *x509.Certificate, generatorOptions GeneratorOptions) (*RawAssetsOnMemory, error) {
	// Convert from days to time.Duration
	certDuration := time.Duration(c.TLSCertDurationDays) * 24 * time.Hour

	// Generate keys for the various components.
	privateKeys := map[string]*rsa.PrivateKey{
		generatorOptions.ApiServerKeyPath:             nil,
		generatorOptions.KubeControllerManagerKeyPath: nil,
		generatorOptions.KubeSchedulerKeyPath:         nil,
		generatorOptions.WorkerKeyPath:                nil,
		generatorOptions.AdminKeyPath:                 nil,
		generatorOptions.EtcdKeyPath:                  nil,
		generatorOptions.EtcdClientKeyPath:            nil,
		generatorOptions.ServiceAccountKeyPath:        nil,
		generatorOptions.ApiServerAggregatorKeyPath:   nil,
	}

	for key := range privateKeys {
		var err error
		if privateKeys[key], err = getOrCreatePrivateKey(key); err != nil {
			return nil, err
		}
	}

	// Compute kubernetesServiceIP from serviceCIDR
	_, serviceNet, err := net.ParseCIDR(c.ServiceCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid serviceCIDR: %v", err)
	}
	kubernetesServiceIPAddr := netutil.IncrementIP(serviceNet.IP)

	dnsNames := append(
		[]string{
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			"kubernetes.default.svc.cluster.local",
		}, c.APIServerExternalDNSNames...)

	// 127.0.0.1 also allows control plane components to reach the apiserver via HTTPS at localhost
	ipAddresses := []string{kubernetesServiceIPAddr.String(), "127.0.0.1"}

	apiServerConfig := pki.ServerCertConfig{
		CommonName:  "kube-apiserver",
		DNSNames:    append(dnsNames, c.APIServerExternalDNSNames...),
		IPAddresses: append(ipAddresses, c.APIServerAdditionalIPAddressSans...),
		Duration:    certDuration,
	}
	apiServerCert, err := pki.NewSignedServerCertificate(apiServerConfig, privateKeys[generatorOptions.ApiServerKeyPath], caCert, caKey)
	if err != nil {
		return nil, err
	}

	etcdConfig := pki.ServerCertConfig{
		CommonName: "kube-etcd",
		DNSNames:   c.EtcdNodeDNSNames,
		// etcd https client/peer interfaces are not exposed externally
		// but anyway we'll make it valid for the same duration as other certs just because it is easy to implement.
		Duration: certDuration,
	}

	etcdCert, err := pki.NewSignedServerCertificate(etcdConfig, privateKeys[generatorOptions.EtcdKeyPath], caCert, caKey)
	if err != nil {
		return nil, err
	}

	workerConfig := pki.ClientCertConfig{
		CommonName: "kube-worker",
		DNSNames: []string{
			fmt.Sprintf("*.%s.compute.internal", c.Region),
			"*.ec2.internal",
		},
		Duration: certDuration,
	}
	workerCert, err := pki.NewSignedClientCertificate(workerConfig, privateKeys[generatorOptions.WorkerKeyPath], caCert, caKey)
	if err != nil {
		return nil, err
	}

	etcdClientConfig := pki.ClientCertConfig{
		CommonName: "kube-etcd-client",
		Duration:   certDuration,
	}

	etcdClientCert, err := pki.NewSignedClientCertificate(etcdClientConfig, privateKeys[generatorOptions.EtcdClientKeyPath], caCert, caKey)
	if err != nil {
		return nil, err
	}

	adminConfig := pki.ClientCertConfig{
		CommonName:   "kube-admin",
		Organization: []string{"system:masters"},
		Duration:     certDuration,
	}
	adminCert, err := pki.NewSignedClientCertificate(adminConfig, privateKeys[generatorOptions.AdminKeyPath], caCert, caKey)
	if err != nil {
		return nil, err
	}

	kubeControllerManagerConfig := pki.ClientCertConfig{
		CommonName: "system:kube-controller-manager",
		Duration:   certDuration,
	}
	kubeControllerManagerCert, err := pki.NewSignedClientCertificate(kubeControllerManagerConfig, privateKeys[generatorOptions.KubeControllerManagerKeyPath], caCert, caKey)
	if err != nil {
		return nil, err
	}

	kubeSchedulerConfig := pki.ClientCertConfig{
		CommonName: "system:kube-scheduler",
		Duration:   certDuration,
	}
	kubeSchedulerCert, err := pki.NewSignedClientCertificate(kubeSchedulerConfig, privateKeys[generatorOptions.KubeSchedulerKeyPath], caCert, caKey)
	if err != nil {
		return nil, err
	}

	apiServerAggregatorConfig := pki.ClientCertConfig{
		CommonName: "aggregator",
		Duration:   certDuration,
	}
	apiServerAggregatorCert, err := pki.NewSignedClientCertificate(apiServerAggregatorConfig, privateKeys[generatorOptions.ApiServerAggregatorKeyPath], caCert, caKey)
	if err != nil {
		return nil, err
	}

	authTokens := ""
	tlsBootstrapToken, err := RandomTokenString()
	if err != nil {
		return nil, err
	}

	encryptionConfig, err := EncryptionConfig()
	if err != nil {
		return nil, err
	}

	r := &RawAssetsOnMemory{
		CACert:                    pki.EncodeCertificatePEM(caCert),
		APIServerCert:             pki.EncodeCertificatePEM(apiServerCert),
		KubeControllerManagerCert: pki.EncodeCertificatePEM(kubeControllerManagerCert),
		KubeSchedulerCert:         pki.EncodeCertificatePEM(kubeSchedulerCert),
		WorkerCert:                pki.EncodeCertificatePEM(workerCert),
		AdminCert:                 pki.EncodeCertificatePEM(adminCert),
		EtcdCert:                  pki.EncodeCertificatePEM(etcdCert),
		EtcdClientCert:            pki.EncodeCertificatePEM(etcdClientCert),
		APIServerAggregatorCert:   pki.EncodeCertificatePEM(apiServerAggregatorCert),
		CAKey:                     pki.EncodePrivateKeyPEM(caKey),
		APIServerKey:              pki.EncodePrivateKeyPEM(privateKeys[generatorOptions.ApiServerKeyPath]),
		KubeControllerManagerKey:  pki.EncodePrivateKeyPEM(privateKeys[generatorOptions.KubeControllerManagerKeyPath]),
		KubeSchedulerKey:          pki.EncodePrivateKeyPEM(privateKeys[generatorOptions.KubeSchedulerKeyPath]),
		WorkerKey:                 pki.EncodePrivateKeyPEM(privateKeys[generatorOptions.WorkerKeyPath]),
		AdminKey:                  pki.EncodePrivateKeyPEM(privateKeys[generatorOptions.AdminKeyPath]),
		EtcdKey:                   pki.EncodePrivateKeyPEM(privateKeys[generatorOptions.EtcdKeyPath]),
		EtcdClientKey:             pki.EncodePrivateKeyPEM(privateKeys[generatorOptions.EtcdClientKeyPath]),
		ServiceAccountKey:         pki.EncodePrivateKeyPEM(privateKeys[generatorOptions.ServiceAccountKeyPath]),
		APIServerAggregatorKey:    pki.EncodePrivateKeyPEM(privateKeys[generatorOptions.ApiServerAggregatorKeyPath]),

		AuthTokens:        []byte(authTokens),
		TLSBootstrapToken: []byte(tlsBootstrapToken),
		EncryptionConfig:  []byte(encryptionConfig),
	}

	return r, nil
}
