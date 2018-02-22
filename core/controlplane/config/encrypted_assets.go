package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"os"
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
	WorkerCACert   []byte
	WorkerCAKey    []byte
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
	EtcdTrustedCA  []byte
	KIAMServerCert []byte
	KIAMServerKey  []byte
	KIAMAgentCert  []byte
	KIAMAgentKey   []byte
	KIAMCACert     []byte

	// Other assets.
	AuthTokens        []byte
	TLSBootstrapToken []byte
}

type RawAssetsOnDisk struct {
	// PEM encoded TLS assets.
	CACert         RawCredentialOnDisk
	CAKey          RawCredentialOnDisk
	WorkerCACert   RawCredentialOnDisk
	WorkerCAKey    RawCredentialOnDisk
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
	EtcdTrustedCA  RawCredentialOnDisk
	KIAMServerCert RawCredentialOnDisk
	KIAMServerKey  RawCredentialOnDisk
	KIAMAgentCert  RawCredentialOnDisk
	KIAMAgentKey   RawCredentialOnDisk
	KIAMCACert     RawCredentialOnDisk

	// Other assets.
	AuthTokens        RawCredentialOnDisk
	TLSBootstrapToken RawCredentialOnDisk
}

type EncryptedAssetsOnDisk struct {
	// Encrypted PEM encoded TLS assets.
	CACert         EncryptedCredentialOnDisk
	CAKey          EncryptedCredentialOnDisk
	WorkerCACert   EncryptedCredentialOnDisk
	WorkerCAKey    EncryptedCredentialOnDisk
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
	EtcdTrustedCA  EncryptedCredentialOnDisk
	KIAMServerCert EncryptedCredentialOnDisk
	KIAMServerKey  EncryptedCredentialOnDisk
	KIAMAgentCert  EncryptedCredentialOnDisk
	KIAMAgentKey   EncryptedCredentialOnDisk
	KIAMCACert     EncryptedCredentialOnDisk

	// Other encrypted assets.
	AuthTokens        EncryptedCredentialOnDisk
	TLSBootstrapToken EncryptedCredentialOnDisk
}

type CompactAssets struct {
	// PEM -> encrypted -> gzip -> base64 encoded TLS assets.
	CACert         string
	CAKey          string
	WorkerCACert   string
	WorkerCAKey    string
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
	EtcdTrustedCA  string
	KIAMServerCert string
	KIAMServerKey  string
	KIAMAgentCert  string
	KIAMAgentKey   string
	KIAMCACert     string

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
	// KIAM is set to true when you want kube-aws to render TLS assets for uswitch/kiam
	KIAM bool
}

func (c *Cluster) NewAssetsOnDisk(dir string, o CredentialsOptions) (*RawAssetsOnDisk, error) {
	fmt.Println("Generating credentials...")
	var caKey *rsa.PrivateKey
	var caCert *x509.Certificate
	if o.GenerateCA {
		var err error
		caKey, caCert, err = c.NewTLSCA()
		if err != nil {
			return nil, fmt.Errorf("failed generating cluster CA: %v", err)
		}
		fmt.Printf("-> Generating new TLS CA\n")
	} else {
		fmt.Printf("-> Parsing existing TLS CA\n")
		if caKeyBytes, err := ioutil.ReadFile(o.CaKeyPath); err != nil {
			return nil, fmt.Errorf("failed reading ca key file %s : %v", o.CaKeyPath, err)
		} else {
			if caKey, err = tlsutil.DecodePrivateKeyPEM(caKeyBytes); err != nil {
				return nil, fmt.Errorf("failed parsing ca key: %v", err)
			}
		}
		if caCertBytes, err := ioutil.ReadFile(o.CaCertPath); err != nil {
			return nil, fmt.Errorf("failed reading ca cert file %s : %v", o.CaCertPath, err)
		} else {
			if caCert, err = tlsutil.DecodeCertificatePEM(caCertBytes); err != nil {
				return nil, fmt.Errorf("failed parsing ca cert: %v", err)
			}
		}
	}

	fmt.Println("-> Generating new assets")
	assets, err := c.NewAssetsOnMemory(caKey, caCert, o.KIAM)
	if err != nil {
		return nil, fmt.Errorf("Error generating default assets: %v", err)
	}

	tlsBootstrappingEnabled := c.Experimental.TLSBootstrap.Enabled
	certsManagedByKubeAws := c.ManageCertificates
	caKeyRequiredOnController := certsManagedByKubeAws && tlsBootstrappingEnabled

	fmt.Printf("--> Summarizing the configuration\n    Kubelet TLS bootstrapping enabled=%v, TLS certificates managed by kube-aws=%v, CA key required on controller nodes=%v\n", tlsBootstrappingEnabled, certsManagedByKubeAws, caKeyRequiredOnController)

	fmt.Println("--> Writing to the storage")
	alsoWriteCAKey := o.GenerateCA || caKeyRequiredOnController
	if err := assets.WriteToDir(dir, alsoWriteCAKey, o.KIAM); err != nil {
		return nil, fmt.Errorf("Error creating assets: %v", err)
	}

	{
		fmt.Println("--> Verifying the result")
		verified, err := ReadRawAssets(dir, certsManagedByKubeAws, tlsBootstrappingEnabled, o.KIAM)

		if err != nil {
			return nil, fmt.Errorf("failed verifying the result: %v", err)
		}

		return verified, nil
	}
}

func (c *Cluster) NewAssetsOnMemory(caKey *rsa.PrivateKey, caCert *x509.Certificate, kiamEnabled bool) (*RawAssetsOnMemory, error) {
	// Convert from days to time.Duration
	certDuration := time.Duration(c.TLSCertDurationDays) * 24 * time.Hour

	// Generate keys for the various components.
	keys := make([]*rsa.PrivateKey, 7)
	var err error
	for i := range keys {
		if keys[i], err = tlsutil.NewPrivateKey(); err != nil {
			return nil, err
		}
	}
	apiServerKey, workerKey, adminKey, etcdKey, etcdClientKey, kiamAgentKey, kiamServerKey := keys[0], keys[1], keys[2], keys[3], keys[4], keys[5], keys[6]

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
		// etcd https client/peer interfaces are not exposed externally
		// but anyway we'll make it valid for the same duration as other certs just because it is easy to implement.
		Duration: certDuration,
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

	r := &RawAssetsOnMemory{
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
	}

	if kiamEnabled {
		// See https://github.com/uswitch/kiam/blob/master/docs/agent.json
		agentConfig := tlsutil.ClientCertConfig{
			CommonName: "Kiam Agent",
			Duration:   certDuration,
		}
		kiamAgentCert, err := tlsutil.NewSignedClientCertificate(agentConfig, kiamAgentKey, caCert, caKey)
		if err != nil {
			return nil, err
		}
		// See https://github.com/uswitch/kiam/blob/master/docs/server.json
		serverConfig := tlsutil.ClientCertConfig{
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
		kiamServerCert, err := tlsutil.NewSignedKIAMCertificate(serverConfig, kiamServerKey, caCert, caKey)
		if err != nil {
			return nil, err
		}

		r.KIAMCACert = tlsutil.EncodeCertificatePEM(caCert)
		r.KIAMAgentCert = tlsutil.EncodeCertificatePEM(kiamAgentCert)
		r.KIAMAgentKey = tlsutil.EncodePrivateKeyPEM(kiamAgentKey)
		r.KIAMServerCert = tlsutil.EncodeCertificatePEM(kiamServerCert)
		r.KIAMServerKey = tlsutil.EncodePrivateKeyPEM(kiamServerKey)
	}

	return r, nil
}

func ReadRawAssets(dirname string, manageCertificates bool, caKeyRequiredOnController bool, kiamEnabled bool) (*RawAssetsOnDisk, error) {
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
			{"worker-ca.pem", &r.WorkerCACert, nil},
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
			{"etcd-trusted-ca.pem", &r.EtcdTrustedCA, nil},
		}...)

		if caKeyRequiredOnController {
			files = append(files, entry{"worker-ca-key.pem", &r.WorkerCAKey, nil})
		}

		if kiamEnabled {
			files = append(files, entry{"kiam-server-key.pem", &r.KIAMServerKey, nil})
			files = append(files, entry{"kiam-server.pem", &r.KIAMServerCert, nil})
			files = append(files, entry{"kiam-agent-key.pem", &r.KIAMAgentKey, nil})
			files = append(files, entry{"kiam-agent.pem", &r.KIAMAgentCert, nil})
			files = append(files, entry{"kiam-ca.pem", &r.KIAMCACert, nil})
		}
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

func ReadOrEncryptAssets(dirname string, manageCertificates bool, caKeyRequiredOnController bool, kiamEnabled bool, encryptor CachedEncryptor) (*EncryptedAssetsOnDisk, error) {
	defaultTokensFile := ""
	defaultTLSBootstrapToken, err := RandomTLSBootstrapTokenString()
	if err != nil {
		return nil, err
	}

	r := new(EncryptedAssetsOnDisk)

	type entry struct {
		name          string
		data          *EncryptedCredentialOnDisk
		defaultValue  *string
		readEncrypted bool
	}

	files := []entry{
		{"tokens.csv", &r.AuthTokens, &defaultTokensFile, true},
		{"kubelet-tls-bootstrap-token", &r.TLSBootstrapToken, &defaultTLSBootstrapToken, true},
	}

	if manageCertificates {
		files = append(files, []entry{
			{"ca.pem", &r.CACert, nil, false},
			{"worker-ca.pem", &r.WorkerCACert, nil, false},
			{"apiserver.pem", &r.APIServerCert, nil, false},
			{"apiserver-key.pem", &r.APIServerKey, nil, true},
			{"worker.pem", &r.WorkerCert, nil, false},
			{"worker-key.pem", &r.WorkerKey, nil, true},
			{"admin.pem", &r.AdminCert, nil, false},
			{"admin-key.pem", &r.AdminKey, nil, true},
			{"etcd.pem", &r.EtcdCert, nil, false},
			{"etcd-key.pem", &r.EtcdKey, nil, true},
			{"etcd-client.pem", &r.EtcdClientCert, nil, false},
			{"etcd-client-key.pem", &r.EtcdClientKey, nil, true},
			{"etcd-trusted-ca.pem", &r.EtcdTrustedCA, nil, false},
		}...)

		if caKeyRequiredOnController {
			files = append(files, entry{"worker-ca-key.pem", &r.WorkerCAKey, nil, true})
		}

		if kiamEnabled {
			files = append(files, entry{"kiam-server-key.pem", &r.KIAMServerKey, nil, true})
			files = append(files, entry{"kiam-server.pem", &r.KIAMServerCert, nil, false})
			files = append(files, entry{"kiam-agent-key.pem", &r.KIAMAgentKey, nil, true})
			files = append(files, entry{"kiam-agent.pem", &r.KIAMAgentCert, nil, false})
			files = append(files, entry{"kiam-ca.pem", &r.KIAMCACert, nil, false})
		}
	}

	for _, file := range files {
		path := filepath.Join(dirname, file.name)
		if file.readEncrypted {
			data, err := encryptor.EncryptedCredentialFromPath(path, file.defaultValue)
			if err != nil {
				return nil, fmt.Errorf("Error encrypting %s: %v", path, err)
			}

			*file.data = *data
			if err := data.Persist(); err != nil {
				return nil, fmt.Errorf("Error persisting %s: %v", path, err)
			}
		} else {
			raw, err := RawCredentialFileFromPath(path, file.defaultValue)
			if err != nil {
				return nil, fmt.Errorf("Error reading credential file %s: %v", path, err)
			}
			(*file.data).content = raw.content
		}
	}

	return r, nil
}

func (r *RawAssetsOnMemory) WriteToDir(dirname string, includeCAKey bool, kiamEnabled bool) error {
	type asset struct {
		name             string
		data             []byte
		overwrite        bool
		ifEmptySymlinkTo string
	}
	assets := []asset{
		{"ca.pem", r.CACert, true, ""},
		{"worker-ca.pem", r.WorkerCACert, true, "ca.pem"},
		{"apiserver.pem", r.APIServerCert, true, ""},
		{"apiserver-key.pem", r.APIServerKey, true, ""},
		{"worker.pem", r.WorkerCert, true, ""},
		{"worker-key.pem", r.WorkerKey, true, ""},
		{"admin.pem", r.AdminCert, true, ""},
		{"admin-key.pem", r.AdminKey, true, ""},
		{"etcd.pem", r.EtcdCert, true, ""},
		{"etcd-key.pem", r.EtcdKey, true, ""},
		{"etcd-client.pem", r.EtcdClientCert, true, ""},
		{"etcd-client-key.pem", r.EtcdClientKey, true, ""},
		{"etcd-trusted-ca.pem", r.EtcdTrustedCA, true, "ca.pem"},
		{"kubelet-tls-bootstrap-token", r.TLSBootstrapToken, true, ""},

		// Content entirely provided by user, so do not overwrite it if
		// the file already exists
		{"tokens.csv", r.AuthTokens, false, ""},
	}

	if includeCAKey {
		// This is required to be linked from worker-ca-key.pem
		assets = append(assets,
			asset{"ca-key.pem", r.CAKey, true, ""},
			asset{"worker-ca-key.pem", r.WorkerCAKey, true, "ca-key.pem"},
		)
	}

	if kiamEnabled {
		assets = append(assets,
			asset{"kiam-server-key.pem", r.KIAMServerKey, true, ""},
			asset{"kiam-server.pem", r.KIAMServerCert, true, ""},
			asset{"kiam-agent-key.pem", r.KIAMAgentKey, true, ""},
			asset{"kiam-agent.pem", r.KIAMAgentCert, true, ""},
			asset{"kiam-ca.pem", r.KIAMCACert, true, "ca.pem"},
		)
	}

	for _, asset := range assets {
		path := filepath.Join(dirname, asset.name)

		if !asset.overwrite {
			info, err := os.Stat(path)
			if info != nil {
				continue
			}

			// Unexpected error
			if err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		if len(asset.data) == 0 {
			if asset.ifEmptySymlinkTo != "" {
				// etcd trusted ca and worker-ca are separate files, but pointing to ca.pem by default.
				// In advanced configurations, when certs are managed outside of kube-aws,
				// these can be separate CAs to ensure that worker nodes have no certs which would let them
				// access etcd directly. If worker-ca.pem != ca.pem, then ca.pem should include worker-ca.pem
				// to let TLS bootstrapped workers acces APIServer.
				wd, err := os.Getwd()
				if err != nil {
					return err
				}

				if err := os.Chdir(dirname); err != nil {
					return err
				}

				// The path of the symlink
				from := asset.name
				// The path to the actual file
				to := asset.ifEmptySymlinkTo

				lstatFileInfo, lstatErr := os.Lstat(from)
				symlinkExists := lstatErr == nil && (lstatFileInfo.Mode()&os.ModeSymlink == os.ModeSymlink)
				fileExists := lstatErr == nil && !symlinkExists

				if fileExists {
					fmt.Printf("INFO: Removing a file at %s\n", from)
					if err := os.Remove(from); err != nil {
						return err
					}
				}

				if symlinkExists {
					fmt.Printf("INFO: Removing a symlink at %s\n", from)
					if err := os.Remove(from); err != nil {
						return err
					}
				}

				fmt.Printf("INFO: Creating a symlink from %s to %s\n", from, to)
				if err := os.Symlink(to, from); err != nil {
					return err
				}

				if err := os.Chdir(wd); err != nil {
					return err
				}
				continue
			} else if asset.name != "tokens.csv" {
				return fmt.Errorf("Not sure what to do for %s", path)
			}
		}
		fmt.Printf("INFO: Writing %d bytes to %s\n", len(asset.data), path)
		if err := ioutil.WriteFile(path, asset.data, 0600); err != nil {
			return err
		}
	}

	return nil
}

func (r *EncryptedAssetsOnDisk) WriteToDir(dirname string, kiamEnabled bool) error {
	type asset struct {
		name string
		data EncryptedCredentialOnDisk
	}
	assets := []asset{
		{"ca.pem", r.CACert},
		{"ca-key.pem", r.CAKey},
		{"worker-ca.pem", r.WorkerCACert},
		{"worker-ca-key.pem", r.WorkerCAKey},
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
		{"etcd-trusted-ca.pem", r.EtcdTrustedCA},

		{"tokens.csv", r.AuthTokens},
		{"kubelet-tls-bootstrap-token", r.TLSBootstrapToken},
	}
	if kiamEnabled {
		assets = append(assets,
			asset{"kiam-server-key.pem", r.KIAMServerKey},
			asset{"kiam-server.pem", r.KIAMServerCert},
			asset{"kiam-agent-key.pem", r.KIAMAgentKey},
			asset{"kiam-agent.pem", r.KIAMAgentCert},
			asset{"kiam-ca.pem", r.KIAMCACert},
		)
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
		CACert:       compact(r.CACert), // why no CAKey here?
		WorkerCACert: compact(r.WorkerCACert),
		//WorkerCAKey:    compact(r.WorkerCAKey),
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
		EtcdTrustedCA:  compact(r.EtcdTrustedCA),
		KIAMAgentCert:  compact(r.KIAMAgentCert),
		KIAMAgentKey:   compact(r.KIAMAgentKey),
		KIAMServerCert: compact(r.KIAMServerCert),
		KIAMServerKey:  compact(r.KIAMServerKey),
		KIAMCACert:     compact(r.KIAMCACert),

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
		WorkerCACert:   compact(r.WorkerCACert),
		WorkerCAKey:    compact(r.WorkerCAKey),
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
		EtcdTrustedCA:  compact(r.EtcdTrustedCA),
		KIAMAgentKey:   compact(r.KIAMAgentKey),
		KIAMAgentCert:  compact(r.KIAMAgentCert),
		KIAMServerKey:  compact(r.KIAMServerKey),
		KIAMServerCert: compact(r.KIAMServerCert),
		KIAMCACert:     compact(r.KIAMCACert),

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

func ReadOrCreateEncryptedAssets(tlsAssetsDir string, manageCertificates bool, caKeyRequiredOnController bool, kiamEnabled bool, kmsConfig KMSConfig) (*EncryptedAssetsOnDisk, error) {
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

	return ReadOrEncryptAssets(tlsAssetsDir, manageCertificates, caKeyRequiredOnController, kiamEnabled, encryptor)
}

func ReadOrCreateCompactAssets(assetsDir string, manageCertificates bool, caKeyRequiredOnController bool, kiamEnabled bool, kmsConfig KMSConfig) (*CompactAssets, error) {
	encryptedAssets, err := ReadOrCreateEncryptedAssets(assetsDir, manageCertificates, caKeyRequiredOnController, kiamEnabled, kmsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to read/create encrypted assets: %v", err)
	}

	compactAssets, err := encryptedAssets.Compact()
	if err != nil {
		return nil, fmt.Errorf("failed to compress encrypted assets: %v", err)
	}

	return compactAssets, nil
}

func ReadOrCreateUnencryptedCompactAssets(assetsDir string, manageCertificates bool, caKeyRequiredOnController bool, kiamEnabled bool) (*CompactAssets, error) {
	unencryptedAssets, err := ReadRawAssets(assetsDir, manageCertificates, caKeyRequiredOnController, kiamEnabled)
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
