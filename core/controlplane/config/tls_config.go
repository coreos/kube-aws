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
	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
	"github.com/kubernetes-incubator/kube-aws/model"
	"github.com/kubernetes-incubator/kube-aws/netutil"
	"github.com/kubernetes-incubator/kube-aws/tlsutil"
)

// PEM encoded TLS assets.
type RawTLSAssetsOnMemory struct {
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

// PEM encoded TLS assets.
type RawTLSAssetsOnDisk struct {
	CACert         RawCredentialOnDisk
	CAKey          RawCredentialOnDisk
	APIServerCert  RawCredentialOnDisk
	APIServerKey   RawCredentialOnDisk
	WorkerCert     RawCredentialOnDisk
	WorkerKey      RawCredentialOnDisk
	AdminCert      RawCredentialOnDisk
	AdminKey       RawCredentialOnDisk
	EtcdCert       RawCredentialOnDisk
	EtcdClientCert RawCredentialOnDisk
	EtcdKey        RawCredentialOnDisk
	EtcdClientKey  RawCredentialOnDisk
}

// Encrypted PEM encoded TLS assets
type EncryptedTLSAssetsOnDisk struct {
	CACert         EncryptedCredentialOnDisk
	CAKey          EncryptedCredentialOnDisk
	APIServerCert  EncryptedCredentialOnDisk
	APIServerKey   EncryptedCredentialOnDisk
	WorkerCert     EncryptedCredentialOnDisk
	WorkerKey      EncryptedCredentialOnDisk
	AdminCert      EncryptedCredentialOnDisk
	AdminKey       EncryptedCredentialOnDisk
	EtcdCert       EncryptedCredentialOnDisk
	EtcdClientCert EncryptedCredentialOnDisk
	EtcdKey        EncryptedCredentialOnDisk
	EtcdClientKey  EncryptedCredentialOnDisk
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

type CredentialsOptions struct {
	GenerateCA bool
	CaKeyPath  string
	CaCertPath string
}

func (c *Cluster) NewTLSAssetsOnDisk(dir string, renderCredentialsOpts CredentialsOptions, caKey *rsa.PrivateKey, caCert *x509.Certificate) (*RawTLSAssetsOnDisk, error) {
	assets, err := c.NewTLSAssetsOnMemory(caKey, caCert)
	if err != nil {
		return nil, fmt.Errorf("Error generating default assets: %v", err)
	}
	if err := assets.WriteToDir(dir, renderCredentialsOpts.GenerateCA); err != nil {
		return nil, fmt.Errorf("Error create assets: %v", err)
	}
	return ReadRawTLSAssets(dir)
}

func (c *Cluster) NewTLSAssetsOnMemory(caKey *rsa.PrivateKey, caCert *x509.Certificate) (*RawTLSAssetsOnMemory, error) {
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
		DNSNames:   c.EtcdCluster().DNSNames(),
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
		CommonName:   "kube-admin",
		Organization: []string{"system:masters"},
		Duration:     certDuration,
	}
	adminCert, err := tlsutil.NewSignedClientCertificate(adminConfig, adminKey, caCert, caKey)
	if err != nil {
		return nil, err
	}

	return &RawTLSAssetsOnMemory{
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

func ReadRawTLSAssets(dirname string) (*RawTLSAssetsOnDisk, error) {
	r := new(RawTLSAssetsOnDisk)
	files := []struct {
		name      string
		cert, key *RawCredentialOnDisk
	}{
		{"ca", &r.CACert, &r.CAKey},
		{"apiserver", &r.APIServerCert, &r.APIServerKey},
		{"worker", &r.WorkerCert, &r.WorkerKey},
		{"admin", &r.AdminCert, &r.AdminKey},
		{"etcd", &r.EtcdCert, &r.EtcdKey},
		{"etcd-client", &r.EtcdClientCert, &r.EtcdClientKey},
	}
	for _, file := range files {
		certPath := filepath.Join(dirname, file.name+".pem")
		keyPath := filepath.Join(dirname, file.name+"-key.pem")

		certData, err := RawCredentialFileFromPath(certPath)
		if err != nil {
			return nil, err
		}
		*file.cert = *certData

		if file.key != nil {
			keyData, err := RawCredentialFileFromPath(keyPath)
			if err != nil {
				return nil, err
			}
			*file.key = *keyData
		}
	}
	return r, nil
}

func ReadOrEncryptTLSAssets(dirname string, encryptor CachedEncryptor) (*EncryptedTLSAssetsOnDisk, error) {
	r := new(EncryptedTLSAssetsOnDisk)
	files := []struct {
		name      string
		cert, key *EncryptedCredentialOnDisk
	}{
		{"ca", &r.CACert, &r.CAKey},
		{"apiserver", &r.APIServerCert, &r.APIServerKey},
		{"worker", &r.WorkerCert, &r.WorkerKey},
		{"admin", &r.AdminCert, &r.AdminKey},
		{"etcd", &r.EtcdCert, &r.EtcdKey},
		{"etcd-client", &r.EtcdClientCert, &r.EtcdClientKey},
	}
	for _, file := range files {
		certPath := filepath.Join(dirname, file.name+".pem")
		keyPath := filepath.Join(dirname, file.name+"-key.pem")

		certData, err := encryptor.EncryptedCredentialFromPath(certPath)
		if err != nil {
			return nil, err
		}
		*file.cert = *certData

		if err := certData.Persist(); err != nil {
			return nil, err
		}

		if file.key != nil {
			keyData, err := encryptor.EncryptedCredentialFromPath(keyPath)
			if err != nil {
				return nil, err
			}
			*file.key = *keyData

			if err := keyData.Persist(); err != nil {
				return nil, err
			}
		}
	}

	return r, nil
}

func (r *RawTLSAssetsOnMemory) WriteToDir(dirname string, includeCAKey bool) error {
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

func (r *EncryptedTLSAssetsOnDisk) WriteToDir(dirname string) error {
	assets := []struct {
		name      string
		cert, key EncryptedCredentialOnDisk
	}{
		{"ca", r.CACert, r.CAKey},
		{"apiserver", r.APIServerCert, r.APIServerKey},
		{"worker", r.WorkerCert, r.WorkerKey},
		{"admin", r.AdminCert, r.AdminKey},
		{"etcd", r.EtcdCert, r.EtcdKey},
		{"etcd-client", r.EtcdClientCert, r.EtcdClientKey},
	}
	for _, asset := range assets {
		if err := asset.cert.Persist(); err != nil {
			return err
		}

		if asset.name != "ca" {
			if err := asset.key.Persist(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RawTLSAssetsOnDisk) Compact() (*CompactTLSAssets, error) {
	var err error
	compact := func(c RawCredentialOnDisk) string {
		if err != nil {
			return ""
		}

		var out string
		if out, err = gzipcompressor.CompressData(c.content); err != nil {
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

func (r *EncryptedTLSAssetsOnDisk) Compact() (*CompactTLSAssets, error) {
	var err error
	compact := func(c EncryptedCredentialOnDisk) string {
		if err != nil {
			return ""
		}

		var out string
		if out, err = gzipcompressor.CompressData(c.content); err != nil {
			return ""
		}
		return out
	}
	compactAssets := CompactTLSAssets{
		CACert:         compact(r.CACert),
		CAKey:          compact(r.CAKey),
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
	Region         model.Region
	EncryptService EncryptService
	KMSKeyARN      string
}

func ReadOrCreateEncryptedTLSAssets(tlsAssetsDir string, kmsConfig KMSConfig) (*EncryptedTLSAssetsOnDisk, error) {
	var kmsSvc EncryptService

	// TODO Cleaner way to inject this dependency
	if kmsConfig.EncryptService == nil {
		awsConfig := aws.NewConfig().
			WithRegion(kmsConfig.Region.String()).
			WithCredentialsChainVerboseErrors(true)
		kmsSvc = kms.New(session.New(awsConfig))
	} else {
		kmsSvc = kmsConfig.EncryptService
	}

	encryptionSvc := bytesEncryptionService{
		kmsKeyARN: kmsConfig.KMSKeyARN,
		kmsSvc:    kmsSvc,
	}

	encryptor := CachedEncryptor{
		bytesEncryptionService: encryptionSvc,
	}

	return ReadOrEncryptTLSAssets(tlsAssetsDir, encryptor)
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

func ReadOrCreateUnencryptedCompactTLSAssets(tlsAssetsDir string) (*CompactTLSAssets, error) {
	unencryptedAssets, err := ReadRawTLSAssets(tlsAssetsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read/create TLS assets: %v", err)
	}

	compactAssets, err := unencryptedAssets.Compact()
	if err != nil {
		return nil, fmt.Errorf("failed to compress TLS assets: %v", err)
	}

	return compactAssets, nil
}
