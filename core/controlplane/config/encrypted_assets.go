package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
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

type RawAssetsOnMemory struct {
	// PEM encoded TLS assets.
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

	// Other assets.
	AuthTokens        []byte
	TLSBootstrapToken []byte
}

type RawAssetsOnDisk struct {
	// PEM encoded TLS assets.
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

	// Other assets.
	AuthTokens        RawCredentialOnDisk
	TLSBootstrapToken RawCredentialOnDisk
}

type EncryptedAssetsOnDisk struct {
	// Encrypted PEM encoded TLS assets.
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

	// Other encrypted assets.
	AuthTokens        EncryptedCredentialOnDisk
	TLSBootstrapToken EncryptedCredentialOnDisk
}

type CompactAssets struct {
	// PEM -> encrypted -> gzip -> base64 encoded TLS assets.
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

	// Encrypted -> gzip -> base64 encoded assets.
	AuthTokens        string
	TLSBootstrapToken string
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

func (c *Cluster) NewAssetsOnDisk(dir string, renderCredentialsOpts CredentialsOptions, caKey *rsa.PrivateKey, caCert *x509.Certificate) (*RawAssetsOnDisk, error) {
	assets, err := c.NewAssetsOnMemory(caKey, caCert)
	if err != nil {
		return nil, fmt.Errorf("Error generating default assets: %v", err)
	}
	if err := assets.WriteToDir(dir, renderCredentialsOpts.GenerateCA); err != nil {
		return nil, fmt.Errorf("Error create assets: %v", err)
	}
	return ReadRawAssets(dir, true)
}

func (c *Cluster) NewAssetsOnMemory(caKey *rsa.PrivateKey, caCert *x509.Certificate) (*RawAssetsOnMemory, error) {
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
		DNSNames: append(
			[]string{
				"kubernetes",
				"kubernetes.default",
				"kubernetes.default.svc",
				"kubernetes.default.svc.cluster.local",
			},
			c.ExternalDNSNames()...,
		),
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

	authTokens := ""

	tlsBootstrapToken, err := RandomTLSBootstrapTokenString()
	if err != nil {
		return nil, err
	}

	return &RawAssetsOnMemory{
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

		AuthTokens:        []byte(authTokens),
		TLSBootstrapToken: []byte(tlsBootstrapToken),
	}, nil
}

func ReadRawAssets(dirname string, manageCertificates bool) (*RawAssetsOnDisk, error) {
	defaultTokensFile := ""
	defaultTLSBootstrapToken, err := RandomTLSBootstrapTokenString()
	if err != nil {
		return nil, err
	}

	r := new(RawAssetsOnDisk)

	type entry struct {
		name         string
		data         *RawCredentialOnDisk
		defaultValue *string
	}

	// Uses a random token as default value
	files := []entry{
		{"tokens.csv", &r.AuthTokens, &defaultTokensFile},
		{"kubelet-tls-bootstrap-token", &r.TLSBootstrapToken, &defaultTLSBootstrapToken},
	}

	if manageCertificates {
		// Assumes no default values for any cert
		files = append(files, []entry{
			{"ca.pem", &r.CACert, nil},
			{"ca-key.pem", &r.CAKey, nil},
			{"apiserver.pem", &r.APIServerCert, nil},
			{"apiserver-key.pem", &r.APIServerKey, nil},
			{"worker.pem", &r.WorkerCert, nil},
			{"worker-key.pem", &r.WorkerKey, nil},
			{"admin.pem", &r.AdminCert, nil},
			{"admin-key.pem", &r.AdminKey, nil},
			{"etcd.pem", &r.EtcdCert, nil},
			{"etcd-key.pem", &r.EtcdKey, nil},
			{"etcd-client.pem", &r.EtcdClientCert, nil},
			{"etcd-client-key.pem", &r.EtcdClientKey, nil},
		}...)
	}

	for _, file := range files {
		path := filepath.Join(dirname, file.name)
		data, err := RawCredentialFileFromPath(path, file.defaultValue)
		if err != nil {
			return nil, fmt.Errorf("Error reading credential file %s: %v", path, err)
		}

		*file.data = *data
	}

	return r, nil
}

func ReadOrEncryptAssets(dirname string, manageCertificates bool, encryptor CachedEncryptor) (*EncryptedAssetsOnDisk, error) {
	defaultTokensFile := ""
	defaultTLSBootstrapToken, err := RandomTLSBootstrapTokenString()
	if err != nil {
		return nil, err
	}

	r := new(EncryptedAssetsOnDisk)

	type entry struct {
		name         string
		data         *EncryptedCredentialOnDisk
		defaultValue *string
	}

	files := []entry{
		{"tokens.csv", &r.AuthTokens, &defaultTokensFile},
		{"kubelet-tls-bootstrap-token", &r.TLSBootstrapToken, &defaultTLSBootstrapToken},
	}

	if manageCertificates {
		files = append(files, []entry{
			{"ca.pem", &r.CACert, nil},
			{"ca-key.pem", &r.CAKey, nil},
			{"apiserver.pem", &r.APIServerCert, nil},
			{"apiserver-key.pem", &r.APIServerKey, nil},
			{"worker.pem", &r.WorkerCert, nil},
			{"worker-key.pem", &r.WorkerKey, nil},
			{"admin.pem", &r.AdminCert, nil},
			{"admin-key.pem", &r.AdminKey, nil},
			{"etcd.pem", &r.EtcdCert, nil},
			{"etcd-key.pem", &r.EtcdKey, nil},
			{"etcd-client.pem", &r.EtcdClientCert, nil},
			{"etcd-client-key.pem", &r.EtcdClientKey, nil},
		}...)
	}

	for _, file := range files {
		path := filepath.Join(dirname, file.name)
		data, err := encryptor.EncryptedCredentialFromPath(path, file.defaultValue)
		if err != nil {
			return nil, fmt.Errorf("Error encrypting %s: %v", path, err)
		}

		*file.data = *data
		if err := data.Persist(); err != nil {
			return nil, fmt.Errorf("Error persisting %s: %v", path, err)
		}
	}

	return r, nil
}

func (r *RawAssetsOnMemory) WriteToDir(dirname string, includeCAKey bool) error {
	assets := []struct {
		name string
		data []byte
	}{
		{"ca.pem", r.CACert},
		{"ca-key.pem", r.CAKey},
		{"apiserver.pem", r.APIServerCert},
		{"apiserver-key.pem", r.APIServerKey},
		{"worker.pem", r.WorkerCert},
		{"worker-key.pem", r.WorkerKey},
		{"admin.pem", r.AdminCert},
		{"admin-key.pem", r.AdminKey},
		{"etcd.pem", r.EtcdCert},
		{"etcd-key.pem", r.EtcdKey},
		{"etcd-client.pem", r.EtcdClientCert},
		{"etcd-client-key.pem", r.EtcdClientKey},

		{"tokens.csv", r.AuthTokens},
		{"kubelet-tls-bootstrap-token", r.TLSBootstrapToken},
	}
	for _, asset := range assets {
		path := filepath.Join(dirname, asset.name)

		if asset.name != "ca-key.pem" || includeCAKey {
			if err := ioutil.WriteFile(path, asset.data, 0600); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *EncryptedAssetsOnDisk) WriteToDir(dirname string) error {
	assets := []struct {
		name string
		data EncryptedCredentialOnDisk
	}{
		{"ca.pem", r.CACert},
		{"ca-key.pem", r.CAKey},
		{"apiserver.pem", r.APIServerCert},
		{"apiserver-key.pem", r.APIServerKey},
		{"worker.pem", r.WorkerCert},
		{"worker-key.pem", r.WorkerKey},
		{"admin.pem", r.AdminCert},
		{"admin-key.pem", r.AdminKey},
		{"etcd.pem", r.EtcdCert},
		{"etcd-key.pem", r.EtcdKey},
		{"etcd-client.pem", r.EtcdClientCert},
		{"etcd-client-key.pem", r.EtcdClientKey},

		{"tokens.csv", r.AuthTokens},
		{"kubelet-tls-bootstrap-token", r.TLSBootstrapToken},
	}
	for _, asset := range assets {
		if asset.name != "ca-key.pem" {
			if err := asset.data.Persist(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *RawAssetsOnDisk) Compact() (*CompactAssets, error) {
	var err error
	compact := func(c RawCredentialOnDisk) string {
		// Nothing to compact
		if len(c.content) == 0 {
			return ""
		}

		if err != nil {
			return ""
		}

		var out string
		if out, err = gzipcompressor.CompressData(c.content); err != nil {
			return ""
		}
		return out
	}
	compactAssets := CompactAssets{
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

		AuthTokens:        compact(r.AuthTokens),
		TLSBootstrapToken: compact(r.TLSBootstrapToken),
	}
	if err != nil {
		return nil, err
	}
	return &compactAssets, nil
}

func (r *EncryptedAssetsOnDisk) Compact() (*CompactAssets, error) {
	var err error
	compact := func(c EncryptedCredentialOnDisk) string {
		// Nothing to compact
		if len(c.content) == 0 {
			return ""
		}

		if err != nil {
			return ""
		}

		var out string
		if out, err = gzipcompressor.CompressData(c.content); err != nil {
			return ""
		}
		return out
	}
	compactAssets := CompactAssets{
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

		AuthTokens:        compact(r.AuthTokens),
		TLSBootstrapToken: compact(r.TLSBootstrapToken),
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

func ReadOrCreateEncryptedAssets(tlsAssetsDir string, manageCertificates bool, kmsConfig KMSConfig) (*EncryptedAssetsOnDisk, error) {
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

	return ReadOrEncryptAssets(tlsAssetsDir, manageCertificates, encryptor)
}

func ReadOrCreateCompactAssets(assetsDir string, manageCertificates bool, kmsConfig KMSConfig) (*CompactAssets, error) {
	encryptedAssets, err := ReadOrCreateEncryptedAssets(assetsDir, manageCertificates, kmsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to read/create encrypted assets: %v", err)
	}

	compactAssets, err := encryptedAssets.Compact()
	if err != nil {
		return nil, fmt.Errorf("failed to compress encrypted assets: %v", err)
	}

	return compactAssets, nil
}

func ReadOrCreateUnencryptedCompactAssets(assetsDir string, manageCertificates bool) (*CompactAssets, error) {
	unencryptedAssets, err := ReadRawAssets(assetsDir, manageCertificates)
	if err != nil {
		return nil, fmt.Errorf("failed to read/create encrypted assets: %v", err)
	}

	compactAssets, err := unencryptedAssets.Compact()
	if err != nil {
		return nil, fmt.Errorf("failed to compress encrypted assets: %v", err)
	}

	return compactAssets, nil
}

func RandomTLSBootstrapTokenString() (string, error) {
	b := make([]byte, 256)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (a *CompactAssets) HasAuthTokens() bool {
	return len(a.AuthTokens) > 0
}

func (a *CompactAssets) HasTLSBootstrapToken() bool {
	return len(a.TLSBootstrapToken) > 0
}
