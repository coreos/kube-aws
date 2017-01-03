package config

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	"io/ioutil"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/coreos/kube-aws/gzipcompressor"
	"github.com/coreos/kube-aws/netutil"
	"github.com/coreos/kube-aws/tlsutil"
)

// PEM encoded TLS assets.
type RawTLSAssets struct {
	CACert         []byte
	CAKey          []byte
	APIServerCert  []byte
	APIServerKey   []byte
	WorkerCert     []byte
	WorkerKey      []byte
	AdminCert      []byte
	AdminKey       []byte
	EtcdCert       []byte
	EtcdClientCert []byte
	EtcdKey        []byte
	EtcdClientKey  []byte
}

// Encrypted PEM encoded TLS assets
type EncryptedTLSAssets struct {
	CACert         []byte
	CAKey          []byte
	APIServerCert  []byte
	APIServerKey   []byte
	WorkerCert     []byte
	WorkerKey      []byte
	AdminCert      []byte
	AdminKey       []byte
	EtcdCert       []byte
	EtcdClientCert []byte
	EtcdKey        []byte
	EtcdClientKey  []byte
}

// PEM -> encrypted -> gzip -> base64 encoded TLS assets.
type CompactTLSAssets struct {
	CACert         string
	CAKey          string
	APIServerCert  string
	APIServerKey   string
	WorkerCert     string
	WorkerKey      string
	AdminCert      string
	AdminKey       string
	EtcdCert       string
	EtcdClientCert string
	EtcdClientKey  string
	EtcdKey        string
}

func (c *Cluster) NewTLSCA() (*rsa.PrivateKey, *x509.Certificate, error) {
	caKey, err := tlsutil.NewPrivateKey()
	if err != nil {
		return nil, nil, err
	}

	// Convert from days to time.Duration
	caDuration := time.Duration(c.TLSCADurationDays) * 24 * time.Hour

	caConfig := tlsutil.CACertConfig{
		CommonName:   "kube-ca",
		Organization: "kube-aws",
		Duration:     caDuration,
	}
	caCert, err := tlsutil.NewSelfSignedCACertificate(caConfig, caKey)
	if err != nil {
		return nil, nil, err
	}

	return caKey, caCert, nil
}

func (c *Cluster) NewTLSAssets(caKey *rsa.PrivateKey, caCert *x509.Certificate) (*RawTLSAssets, error) {
	// Convert from days to time.Duration
	certDuration := time.Duration(c.TLSCertDurationDays) * 24 * time.Hour

	// Generate keys for the various components.
	keys := make([]*rsa.PrivateKey, 5)
	var err error
	for i := range keys {
		if keys[i], err = tlsutil.NewPrivateKey(); err != nil {
			return nil, err
		}
	}
	apiServerKey, workerKey, adminKey, etcdKey, etcdClientKey := keys[0], keys[1], keys[2], keys[3], keys[4]

	//Compute kubernetesServiceIP from serviceCIDR
	_, serviceNet, err := net.ParseCIDR(c.ServiceCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid serviceCIDR: %v", err)
	}
	kubernetesServiceIPAddr := netutil.IncrementIP(serviceNet.IP)

	apiServerConfig := tlsutil.ServerCertConfig{
		CommonName: "kube-apiserver",
		DNSNames: []string{
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			"kubernetes.default.svc.cluster.local",
			c.ExternalDNSName,
		},
		IPAddresses: []string{
			kubernetesServiceIPAddr.String(),
		},
		Duration: certDuration,
	}
	apiServerCert, err := tlsutil.NewSignedServerCertificate(apiServerConfig, apiServerKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	etcdConfig := tlsutil.ServerCertConfig{
		CommonName: "kube-etcd",
		DNSNames: []string{
			fmt.Sprintf("*.%s.compute.internal", c.Region),
			"*.ec2.internal",
		},
		//etcd https client/peer interfaces are not exposed externally
		//will live the full year with the CA
		Duration: tlsutil.Duration365d,
	}

	etcdCert, err := tlsutil.NewSignedServerCertificate(etcdConfig, etcdKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	workerConfig := tlsutil.ClientCertConfig{
		CommonName: "kube-worker",
		DNSNames: []string{
			fmt.Sprintf("*.%s.compute.internal", c.Region),
			"*.ec2.internal",
		},
		Duration: certDuration,
	}
	workerCert, err := tlsutil.NewSignedClientCertificate(workerConfig, workerKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	etcdClientConfig := tlsutil.ClientCertConfig{
		CommonName: "kube-etcd-client",
		Duration:   certDuration,
	}

	etcdClientCert, err := tlsutil.NewSignedClientCertificate(etcdClientConfig, etcdClientKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	adminConfig := tlsutil.ClientCertConfig{
		CommonName: "kube-admin",
		Duration:   certDuration,
	}
	adminCert, err := tlsutil.NewSignedClientCertificate(adminConfig, adminKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	return &RawTLSAssets{
		CACert:         tlsutil.EncodeCertificatePEM(caCert),
		APIServerCert:  tlsutil.EncodeCertificatePEM(apiServerCert),
		WorkerCert:     tlsutil.EncodeCertificatePEM(workerCert),
		AdminCert:      tlsutil.EncodeCertificatePEM(adminCert),
		EtcdCert:       tlsutil.EncodeCertificatePEM(etcdCert),
		EtcdClientCert: tlsutil.EncodeCertificatePEM(etcdClientCert),
		CAKey:          tlsutil.EncodePrivateKeyPEM(caKey),
		APIServerKey:   tlsutil.EncodePrivateKeyPEM(apiServerKey),
		WorkerKey:      tlsutil.EncodePrivateKeyPEM(workerKey),
		AdminKey:       tlsutil.EncodePrivateKeyPEM(adminKey),
		EtcdKey:        tlsutil.EncodePrivateKeyPEM(etcdKey),
		EtcdClientKey:  tlsutil.EncodePrivateKeyPEM(etcdClientKey),
	}, nil
}

func ReadRawTLSAssets(dirname string) (*RawTLSAssets, error) {
	r := new(RawTLSAssets)
	files := []struct {
		name      string
		cert, key *[]byte
	}{
		{"ca", &r.CACert, nil},
		{"apiserver", &r.APIServerCert, &r.APIServerKey},
		{"worker", &r.WorkerCert, &r.WorkerKey},
		{"admin", &r.AdminCert, &r.AdminKey},
		{"etcd", &r.EtcdCert, &r.EtcdKey},
		{"etcd-client", &r.EtcdClientCert, &r.EtcdClientKey},
	}
	for _, file := range files {
		certPath := filepath.Join(dirname, file.name+".pem")
		keyPath := filepath.Join(dirname, file.name+"-key.pem")

		certData, err := ioutil.ReadFile(certPath)
		if err != nil {
			return nil, err
		}
		*file.cert = certData

		if file.key != nil {
			keyData, err := ioutil.ReadFile(keyPath)
			if err != nil {
				return nil, err
			}
			*file.key = keyData
		}
	}
	return r, nil
}

func ReadEncryptedTLSAssets(dirname string) (*EncryptedTLSAssets, error) {
	r := new(EncryptedTLSAssets)
	files := []struct {
		name      string
		cert, key *[]byte
	}{
		{"ca", &r.CACert, nil},
		{"apiserver", &r.APIServerCert, &r.APIServerKey},
		{"worker", &r.WorkerCert, &r.WorkerKey},
		{"admin", &r.AdminCert, &r.AdminKey},
		{"etcd", &r.EtcdCert, &r.EtcdKey},
		{"etcd-client", &r.EtcdClientCert, &r.EtcdClientKey},
	}
	for _, file := range files {
		certPath := filepath.Join(dirname, file.name+".pem.enc")
		keyPath := filepath.Join(dirname, file.name+"-key.pem.enc")

		certData, err := ioutil.ReadFile(certPath)
		if err != nil {
			return nil, err
		}
		*file.cert = certData

		if file.key != nil {
			keyData, err := ioutil.ReadFile(keyPath)
			if err != nil {
				return nil, err
			}
			*file.key = keyData
		}
	}
	return r, nil
}

func (r *RawTLSAssets) WriteToDir(dirname string, includeCAKey bool) error {
	assets := []struct {
		name      string
		cert, key []byte
	}{
		{"ca", r.CACert, r.CAKey},
		{"apiserver", r.APIServerCert, r.APIServerKey},
		{"worker", r.WorkerCert, r.WorkerKey},
		{"admin", r.AdminCert, r.AdminKey},
		{"etcd", r.EtcdCert, r.EtcdKey},
		{"etcd-client", r.EtcdClientCert, r.EtcdClientKey},
	}
	for _, asset := range assets {
		certPath := filepath.Join(dirname, asset.name+".pem")
		keyPath := filepath.Join(dirname, asset.name+"-key.pem")

		if err := ioutil.WriteFile(certPath, asset.cert, 0600); err != nil {
			return err
		}

		if asset.name != "ca" || includeCAKey {
			if err := ioutil.WriteFile(keyPath, asset.key, 0600); err != nil {
				return err
			}
		}
	}
	return nil
}

type EncryptService interface {
	Encrypt(*kms.EncryptInput) (*kms.EncryptOutput, error)
}

func (r *RawTLSAssets) Encrypt(kMSKeyARN string, kmsSvc EncryptService) (*EncryptedTLSAssets, error) {
	var err error
	encrypt := func(data []byte) []byte {
		if err != nil {
			return []byte{}
		}

		encryptInput := kms.EncryptInput{
			KeyId:     aws.String(kMSKeyARN),
			Plaintext: data,
		}

		var encryptOutput *kms.EncryptOutput
		if encryptOutput, err = kmsSvc.Encrypt(&encryptInput); err != nil {
			return []byte{}
		}
		return encryptOutput.CiphertextBlob
	}
	encryptedAssets := EncryptedTLSAssets{
		CACert:         encrypt(r.CACert),
		APIServerCert:  encrypt(r.APIServerCert),
		APIServerKey:   encrypt(r.APIServerKey),
		WorkerCert:     encrypt(r.WorkerCert),
		WorkerKey:      encrypt(r.WorkerKey),
		AdminCert:      encrypt(r.AdminCert),
		AdminKey:       encrypt(r.AdminKey),
		EtcdCert:       encrypt(r.EtcdCert),
		EtcdClientCert: encrypt(r.EtcdClientCert),
		EtcdClientKey:  encrypt(r.EtcdClientKey),
		EtcdKey:        encrypt(r.EtcdKey),
	}
	if err != nil {
		return nil, err
	}
	return &encryptedAssets, nil
}

func (r *EncryptedTLSAssets) WriteToDir(dirname string, includeCAKey bool) error {
	assets := []struct {
		name      string
		cert, key []byte
	}{
		{"ca", r.CACert, r.CAKey},
		{"apiserver", r.APIServerCert, r.APIServerKey},
		{"worker", r.WorkerCert, r.WorkerKey},
		{"admin", r.AdminCert, r.AdminKey},
		{"etcd", r.EtcdCert, r.EtcdKey},
		{"etcd-client", r.EtcdClientCert, r.EtcdClientKey},
	}
	for _, asset := range assets {
		certPath := filepath.Join(dirname, asset.name+".pem.enc")
		keyPath := filepath.Join(dirname, asset.name+"-key.pem.enc")

		if err := ioutil.WriteFile(certPath, asset.cert, 0600); err != nil {
			return err
		}

		if asset.name != "ca" || includeCAKey {
			if err := ioutil.WriteFile(keyPath, asset.key, 0600); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *EncryptedTLSAssets) Compact() (*CompactTLSAssets, error) {
	var err error
	compact := func(data []byte) string {
		if err != nil {
			return ""
		}

		var out string
		if out, err = gzipcompressor.CompressData(data); err != nil {
			return ""
		}
		return out
	}
	compactAssets := CompactTLSAssets{
		CACert:         compact(r.CACert),
		APIServerCert:  compact(r.APIServerCert),
		APIServerKey:   compact(r.APIServerKey),
		WorkerCert:     compact(r.WorkerCert),
		WorkerKey:      compact(r.WorkerKey),
		AdminCert:      compact(r.AdminCert),
		AdminKey:       compact(r.AdminKey),
		EtcdCert:       compact(r.EtcdCert),
		EtcdClientCert: compact(r.EtcdClientCert),
		EtcdClientKey:  compact(r.EtcdClientKey),
		EtcdKey:        compact(r.EtcdKey),
	}
	if err != nil {
		return nil, err
	}
	return &compactAssets, nil
}

type KMSConfig struct {
	Region         string
	EncryptService EncryptService
	KMSKeyARN      string
}

func ReadOrCreateEncryptedTLSAssets(tlsAssetsDir string, kmsConfig KMSConfig) (*EncryptedTLSAssets, error) {
	var kmsSvc EncryptService

	encryptedAssets, err := ReadEncryptedTLSAssets(tlsAssetsDir)
	if err != nil {
		rawAssets, err := ReadRawTLSAssets(tlsAssetsDir)
		if err != nil {
			return nil, err
		}

		awsConfig := aws.NewConfig().
			WithRegion(kmsConfig.Region).
			WithCredentialsChainVerboseErrors(true)

		// TODO Cleaner way to inject this dependency
		if kmsConfig.EncryptService == nil {
			kmsSvc = kms.New(session.New(awsConfig))
		} else {
			kmsSvc = kmsConfig.EncryptService
		}

		encryptedAssets, err = rawAssets.Encrypt(kmsConfig.KMSKeyARN, kmsSvc)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt TLS assets: %v", err)
		}

		// The fact KMS encryption produces different ciphertexts for the same plaintext had been
		// causing unnecessary node replacements(https://github.com/coreos/kube-aws/issues/107)
		// Write encrypted tls assets for caching purpose so that we can avoid that.
		encryptedAssets.WriteToDir(tlsAssetsDir, true)
	}

	return encryptedAssets, nil
}

func ReadOrCreateCompactTLSAssets(tlsAssetsDir string, kmsConfig KMSConfig) (*CompactTLSAssets, error) {
	encryptedAssets, err := ReadOrCreateEncryptedTLSAssets(tlsAssetsDir, kmsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to read/create TLS assets: %v", err)
	}

	compactAssets, err := encryptedAssets.Compact()
	if err != nil {
		return nil, fmt.Errorf("failed to compress TLS assets: %v", err)
	}

	return compactAssets, nil
}
