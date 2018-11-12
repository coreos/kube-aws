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
	CaKeyPath  string
	CaCertPath string
	// KIAM is set to true when you want kube-aws to render TLS assets for uswitch/kiam
	KIAM bool
}

func (c Generator) GenerateAssetsOnDisk(dir string, o GeneratorOptions) (*RawAssetsOnDisk, error) {
	logger.Info("Generating credentials...")
	var caKey *rsa.PrivateKey
	var caCert *x509.Certificate
	if o.GenerateCA {
		var err error
		caKey, caCert, err = pki.NewCA(c.TLSCADurationDays)
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
	assets, err := c.GenerateAssetsOnMemory(caKey, caCert, o.KIAM)
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

func (c Generator) GenerateAssetsOnMemory(caKey *rsa.PrivateKey, caCert *x509.Certificate, kiamEnabled bool) (*RawAssetsOnMemory, error) {
	// Convert from days to time.Duration
	certDuration := time.Duration(c.TLSCertDurationDays) * 24 * time.Hour

	// Generate keys for the various components.
	keys := make([]*rsa.PrivateKey, 11)
	var err error
	for i := range keys {
		if keys[i], err = pki.NewPrivateKey(); err != nil {
			return nil, err
		}
	}
	apiServerKey, kubeControllerManagerKey, kubeSchedulerKey, workerKey, adminKey, etcdKey, etcdClientKey, kiamAgentKey, kiamServerKey, serviceAccountKey, apiServerAggregatorKey := keys[0], keys[1], keys[2], keys[3], keys[4], keys[5], keys[6], keys[7], keys[8], keys[9], keys[10]

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
	apiServerCert, err := pki.NewSignedServerCertificate(apiServerConfig, apiServerKey, caCert, caKey)
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

	etcdCert, err := pki.NewSignedServerCertificate(etcdConfig, etcdKey, caCert, caKey)
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
	workerCert, err := pki.NewSignedClientCertificate(workerConfig, workerKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	etcdClientConfig := pki.ClientCertConfig{
		CommonName: "kube-etcd-client",
		Duration:   certDuration,
	}

	etcdClientCert, err := pki.NewSignedClientCertificate(etcdClientConfig, etcdClientKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	adminConfig := pki.ClientCertConfig{
		CommonName:   "kube-admin",
		Organization: []string{"system:masters"},
		Duration:     certDuration,
	}
	adminCert, err := pki.NewSignedClientCertificate(adminConfig, adminKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	kubeControllerManagerConfig := pki.ClientCertConfig{
		CommonName: "system:kube-controller-manager",
		Duration:   certDuration,
	}
	kubeControllerManagerCert, err := pki.NewSignedClientCertificate(kubeControllerManagerConfig, kubeControllerManagerKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	kubeSchedulerConfig := pki.ClientCertConfig{
		CommonName: "system:kube-scheduler",
		Duration:   certDuration,
	}
	kubeSchedulerCert, err := pki.NewSignedClientCertificate(kubeSchedulerConfig, kubeSchedulerKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	apiServerAggregatorConfig := pki.ClientCertConfig{
		CommonName: "aggregator",
		Duration:   certDuration,
	}
	apiServerAggregatorCert, err := pki.NewSignedClientCertificate(apiServerAggregatorConfig, apiServerAggregatorKey, caCert, caKey)
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
		APIServerKey:              pki.EncodePrivateKeyPEM(apiServerKey),
		KubeControllerManagerKey:  pki.EncodePrivateKeyPEM(kubeControllerManagerKey),
		KubeSchedulerKey:          pki.EncodePrivateKeyPEM(kubeSchedulerKey),
		WorkerKey:                 pki.EncodePrivateKeyPEM(workerKey),
		AdminKey:                  pki.EncodePrivateKeyPEM(adminKey),
		EtcdKey:                   pki.EncodePrivateKeyPEM(etcdKey),
		EtcdClientKey:             pki.EncodePrivateKeyPEM(etcdClientKey),
		ServiceAccountKey:         pki.EncodePrivateKeyPEM(serviceAccountKey),
		APIServerAggregatorKey:    pki.EncodePrivateKeyPEM(apiServerAggregatorKey),

		AuthTokens:        []byte(authTokens),
		TLSBootstrapToken: []byte(tlsBootstrapToken),
		EncryptionConfig:  []byte(encryptionConfig),
	}

	if kiamEnabled {
		// See https://github.com/uswitch/kiam/blob/master/docs/agent.json
		agentConfig := pki.ClientCertConfig{
			CommonName: "Kiam Agent",
			Duration:   certDuration,
		}
		kiamAgentCert, err := pki.NewSignedClientCertificate(agentConfig, kiamAgentKey, caCert, caKey)
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
		kiamServerCert, err := pki.NewSignedKIAMCertificate(serverConfig, kiamServerKey, caCert, caKey)
		if err != nil {
			return nil, err
		}

		r.KIAMCACert = pki.EncodeCertificatePEM(caCert)
		r.KIAMAgentCert = pki.EncodeCertificatePEM(kiamAgentCert)
		r.KIAMAgentKey = pki.EncodePrivateKeyPEM(kiamAgentKey)
		r.KIAMServerCert = pki.EncodeCertificatePEM(kiamServerCert)
		r.KIAMServerKey = pki.EncodePrivateKeyPEM(kiamServerKey)
	}

	return r, nil
}
