package credential

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/netutil"
	"github.com/kubernetes-incubator/kube-aws/pki"
	"io/ioutil"
	"net"
	"time"
)

type Generator struct {
	TLSCADurationDays         int
	TLSCertDurationDays       int
	TLSBootstrapEnabled       bool
	ManageCertificates        bool
	Region                    string
	APIServerExternalDNSNames []string
	EtcdNodeDNSNames          []string
	ServiceCIDR               string
}

type GeneratorOptions struct {
	AwsDebug   bool
	GenerateCA bool
	CaCertPath string
	CommonName string
	// KIAM is set to true when you want kube-aws to render TLS assets for uswitch/kiam
	KIAM bool
	// Paths for private certificate keys.
	AdminKeyPath                 string
	ApiServerAggregatorKeyPath   string
	ApiServerKeyPath             string
	CaKeyPath                    string
	EtcdClientKeyPath            string
	EtcdKeyPath                  string
	KiamAgentKeyPath             string
	KiamServerKeyPath            string
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

	tlsBootstrappingEnabled := c.TLSBootstrapEnabled
	certsManagedByKubeAws := c.ManageCertificates
	caKeyRequiredOnController := certsManagedByKubeAws && tlsBootstrappingEnabled

	logger.Infof("--> Summarizing the configuration\n    Kubelet TLS bootstrapping enabled=%v, TLS certificates managed by kube-aws=%v, CA key required on controller nodes=%v\n", tlsBootstrappingEnabled, certsManagedByKubeAws, caKeyRequiredOnController)

	logger.Info("--> Writing to the storage")
	alsoWriteCAKey := o.GenerateCA || caKeyRequiredOnController
	if err := assets.WriteToDir(dir, alsoWriteCAKey, o.KIAM); err != nil {
		return nil, fmt.Errorf("Error creating assets: %v", err)
	}

	{
		logger.Info("--> Verifying the result")
		verified, err := ReadRawAssets(dir, certsManagedByKubeAws, tlsBootstrappingEnabled, o.KIAM)

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
		generatorOptions.KiamAgentKeyPath:             nil,
		generatorOptions.KiamServerKeyPath:            nil,
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

	apiServerConfig := pki.ServerCertConfig{
		CommonName: "kube-apiserver",
		DNSNames: append(
			[]string{
				"kubernetes",
				"kubernetes.default",
				"kubernetes.default.svc",
				"kubernetes.default.svc.cluster.local",
			},
			c.APIServerExternalDNSNames...,
		),
		IPAddresses: []string{
			kubernetesServiceIPAddr.String(),

			// Also allows control plane components to reach the apiserver via HTTPS at localhost
			"127.0.0.1",
		},
		Duration: certDuration,
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

	if generatorOptions.KIAM {
		// See https://github.com/uswitch/kiam/blob/master/docs/agent.json
		agentConfig := pki.ClientCertConfig{
			CommonName: "Kiam Agent",
			Duration:   certDuration,
		}
		kiamAgentCert, err := pki.NewSignedClientCertificate(agentConfig, privateKeys[generatorOptions.KiamAgentKeyPath], caCert, caKey)
		if err != nil {
			return nil, err
		}
		// See https://github.com/uswitch/kiam/blob/master/docs/server.json
		serverConfig := pki.ClientCertConfig{
			CommonName: "Kiam Server",
			DNSNames: append(
				[]string{
					"kiam-server:443",
					"localhost:443",
					"localhost:9610",
				},
			),
			Duration: certDuration,
		}
		kiamServerCert, err := pki.NewSignedKIAMCertificate(serverConfig, privateKeys[generatorOptions.KiamServerKeyPath], caCert, caKey)
		if err != nil {
			return nil, err
		}

		r.KIAMCACert = pki.EncodeCertificatePEM(caCert)
		r.KIAMAgentCert = pki.EncodeCertificatePEM(kiamAgentCert)
		r.KIAMAgentKey = pki.EncodePrivateKeyPEM(privateKeys[generatorOptions.KiamAgentKeyPath])
		r.KIAMServerCert = pki.EncodeCertificatePEM(kiamServerCert)
		r.KIAMServerKey = pki.EncodePrivateKeyPEM(privateKeys[generatorOptions.KiamServerKeyPath])
	}

	return r, nil
}
