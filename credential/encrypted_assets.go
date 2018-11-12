package credential

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"
	"github.com/kubernetes-incubator/kube-aws/logger"
	"github.com/kubernetes-incubator/kube-aws/pki"
)

type RawAssetsOnMemory struct {
	// PEM encoded TLS assets.
	CACert                    []byte
	CAKey                     []byte
	WorkerCACert              []byte
	WorkerCAKey               []byte
	APIServerCert             []byte
	APIServerKey              []byte
	APIServerAggregatorCert   []byte
	APIServerAggregatorKey    []byte
	KubeControllerManagerCert []byte
	KubeControllerManagerKey  []byte
	KubeSchedulerCert         []byte
	KubeSchedulerKey          []byte
	WorkerCert                []byte
	WorkerKey                 []byte
	AdminCert                 []byte
	AdminKey                  []byte
	EtcdCert                  []byte
	EtcdClientCert            []byte
	EtcdKey                   []byte
	EtcdClientKey             []byte
	EtcdTrustedCA             []byte
	KIAMServerCert            []byte
	KIAMServerKey             []byte
	KIAMAgentCert             []byte
	KIAMAgentKey              []byte
	KIAMCACert                []byte
	ServiceAccountKey         []byte

	// Other assets.
	AuthTokens        []byte
	TLSBootstrapToken []byte
	EncryptionConfig  []byte
}

type RawAssetsOnDisk struct {
	// PEM encoded TLS assets.
	CACert                    PlaintextFile
	CAKey                     PlaintextFile
	WorkerCACert              PlaintextFile
	WorkerCAKey               PlaintextFile
	APIServerCert             PlaintextFile
	APIServerKey              PlaintextFile
	APIServerAggregatorCert   PlaintextFile
	APIServerAggregatorKey    PlaintextFile
	KubeControllerManagerCert PlaintextFile
	KubeControllerManagerKey  PlaintextFile
	KubeSchedulerCert         PlaintextFile
	KubeSchedulerKey          PlaintextFile
	WorkerCert                PlaintextFile
	WorkerKey                 PlaintextFile
	AdminCert                 PlaintextFile
	AdminKey                  PlaintextFile
	EtcdCert                  PlaintextFile
	EtcdClientCert            PlaintextFile
	EtcdKey                   PlaintextFile
	EtcdClientKey             PlaintextFile
	EtcdTrustedCA             PlaintextFile
	KIAMServerCert            PlaintextFile
	KIAMServerKey             PlaintextFile
	KIAMAgentCert             PlaintextFile
	KIAMAgentKey              PlaintextFile
	KIAMCACert                PlaintextFile
	ServiceAccountKey         PlaintextFile

	// Other assets.
	AuthTokens        PlaintextFile
	TLSBootstrapToken PlaintextFile
	EncryptionConfig  PlaintextFile
}

type EncryptedAssetsOnDisk struct {
	// Encrypted PEM encoded TLS assets.
	CACert                    EncryptedFile
	CAKey                     EncryptedFile
	WorkerCACert              EncryptedFile
	WorkerCAKey               EncryptedFile
	APIServerCert             EncryptedFile
	APIServerKey              EncryptedFile
	APIServerAggregatorCert   EncryptedFile
	APIServerAggregatorKey    EncryptedFile
	KubeControllerManagerCert EncryptedFile
	KubeControllerManagerKey  EncryptedFile
	KubeSchedulerCert         EncryptedFile
	KubeSchedulerKey          EncryptedFile
	WorkerCert                EncryptedFile
	WorkerKey                 EncryptedFile
	AdminCert                 EncryptedFile
	AdminKey                  EncryptedFile
	EtcdCert                  EncryptedFile
	EtcdClientCert            EncryptedFile
	EtcdKey                   EncryptedFile
	EtcdClientKey             EncryptedFile
	EtcdTrustedCA             EncryptedFile
	KIAMServerCert            EncryptedFile
	KIAMServerKey             EncryptedFile
	KIAMAgentCert             EncryptedFile
	KIAMAgentKey              EncryptedFile
	KIAMCACert                EncryptedFile
	ServiceAccountKey         EncryptedFile

	// Other encrypted assets.
	AuthTokens        EncryptedFile
	TLSBootstrapToken EncryptedFile
	EncryptionConfig  EncryptedFile
}

type CompactAssets struct {
	// PEM -> encrypted -> gzip -> base64 encoded TLS assets.
	CACert                    string
	CAKey                     string
	WorkerCACert              string
	WorkerCAKey               string
	APIServerCert             string
	APIServerKey              string
	APIServerAggregatorCert   string
	APIServerAggregatorKey    string
	KubeControllerManagerCert string
	KubeControllerManagerKey  string
	KubeSchedulerCert         string
	KubeSchedulerKey          string
	WorkerCert                string
	WorkerKey                 string
	AdminCert                 string
	AdminKey                  string
	EtcdCert                  string
	EtcdClientCert            string
	EtcdClientKey             string
	EtcdKey                   string
	EtcdTrustedCA             string
	KIAMServerCert            string
	KIAMServerKey             string
	KIAMAgentCert             string
	KIAMAgentKey              string
	KIAMCACert                string
	ServiceAccountKey         string

	// Encrypted -> gzip -> base64 encoded assets.
	AuthTokens        string
	TLSBootstrapToken string

	// Encrypted -> base64 encoded EncryptionConfig.
	EncryptionConfig string
}

func ReadRawAssets(dirname string, manageCertificates bool, caKeyRequiredOnController bool, kiamEnabled bool) (*RawAssetsOnDisk, error) {
	defaultTokensFile := ""
	defaultServiceAccountKey := "<<<" + filepath.Join(dirname, "apiserver-key.pem")
	defaultTLSBootstrapToken, err := RandomTokenString()
	if err != nil {
		return nil, err
	}

	defaultEncryptionConfig, err := EncryptionConfig()
	if err != nil {
		return nil, err
	}

	r := new(RawAssetsOnDisk)

	type entry struct {
		name         string
		data         *PlaintextFile
		defaultValue *string
		expiryCheck  bool
	}

	// Uses a random token as default value
	files := []entry{
		{name: "tokens.csv", data: &r.AuthTokens, defaultValue: &defaultTokensFile, expiryCheck: false},
		{name: "kubelet-tls-bootstrap-token", data: &r.TLSBootstrapToken, defaultValue: &defaultTLSBootstrapToken, expiryCheck: false},
		{name: "encryption-config.yaml", data: &r.EncryptionConfig, defaultValue: &defaultEncryptionConfig, expiryCheck: false},
	}

	if manageCertificates {
		// Assumes no default values for any cert
		files = append(files, []entry{
			{name: "ca.pem", data: &r.CACert, defaultValue: nil, expiryCheck: true},
			{name: "worker-ca.pem", data: &r.WorkerCACert, defaultValue: nil, expiryCheck: true},
			{name: "apiserver.pem", data: &r.APIServerCert, defaultValue: nil, expiryCheck: true},
			{name: "apiserver-key.pem", data: &r.APIServerKey, defaultValue: nil, expiryCheck: false},
			{name: "kube-controller-manager.pem", data: &r.KubeControllerManagerCert, defaultValue: nil, expiryCheck: true},
			{name: "kube-controller-manager-key.pem", data: &r.KubeControllerManagerKey, defaultValue: nil, expiryCheck: false},
			{name: "kube-scheduler.pem", data: &r.KubeSchedulerCert, defaultValue: nil, expiryCheck: true},
			{name: "kube-scheduler-key.pem", data: &r.KubeSchedulerKey, defaultValue: nil, expiryCheck: false},
			{name: "worker.pem", data: &r.WorkerCert, defaultValue: nil, expiryCheck: true},
			{name: "worker-key.pem", data: &r.WorkerKey, defaultValue: nil, expiryCheck: false},
			{name: "admin.pem", data: &r.AdminCert, defaultValue: nil, expiryCheck: false},
			{name: "admin-key.pem", data: &r.AdminKey, defaultValue: nil, expiryCheck: false},
			{name: "etcd.pem", data: &r.EtcdCert, defaultValue: nil, expiryCheck: true},
			{name: "etcd-key.pem", data: &r.EtcdKey, defaultValue: nil, expiryCheck: false},
			{name: "etcd-client.pem", data: &r.EtcdClientCert, defaultValue: nil, expiryCheck: true},
			{name: "etcd-client-key.pem", data: &r.EtcdClientKey, defaultValue: nil, expiryCheck: false},
			{name: "etcd-trusted-ca.pem", data: &r.EtcdTrustedCA, defaultValue: nil, expiryCheck: true},
			{name: "apiserver-aggregator-key.pem", data: &r.APIServerAggregatorKey, defaultValue: nil, expiryCheck: false},
			{name: "apiserver-aggregator.pem", data: &r.APIServerAggregatorCert, defaultValue: nil, expiryCheck: true},
			// allow setting service-account-key from the apiserver-key by default.
			{name: "service-account-key.pem", data: &r.ServiceAccountKey, defaultValue: &defaultServiceAccountKey},
		}...)

		if caKeyRequiredOnController {
			files = append(files, entry{name: "worker-ca-key.pem", data: &r.WorkerCAKey, defaultValue: nil, expiryCheck: true})
		}

		if kiamEnabled {
			files = append(files, entry{name: "kiam-server-key.pem", data: &r.KIAMServerKey, defaultValue: nil, expiryCheck: false})
			files = append(files, entry{name: "kiam-server.pem", data: &r.KIAMServerCert, defaultValue: nil, expiryCheck: true})
			files = append(files, entry{name: "kiam-agent-key.pem", data: &r.KIAMAgentKey, defaultValue: nil, expiryCheck: false})
			files = append(files, entry{name: "kiam-agent.pem", data: &r.KIAMAgentCert, defaultValue: nil, expiryCheck: true})
			files = append(files, entry{name: "kiam-ca.pem", data: &r.KIAMCACert, defaultValue: nil, expiryCheck: true})
		}
	}

	for _, file := range files {
		path := filepath.Join(dirname, file.name)
		data, err := RawCredentialFileFromPath(path, file.defaultValue)
		if err != nil {
			return nil, fmt.Errorf("error reading credential file %s: %v", path, err)
		}
		if file.expiryCheck {
			certs, err := pki.CertificatesFromBytes(data.Bytes())
			if err != nil {
				return nil, err
			}
			for _, cert := range certs {
				if cert.IsExpired() {
					return nil, fmt.Errorf("the following certificate in file %s has expired:-\n\n%s", path, cert)
				}
			}
		}

		*file.data = *data
	}

	return r, nil
}

func ReadOrEncryptAssets(dirname string, manageCertificates bool, caKeyRequiredOnController bool, kiamEnabled bool, store Store) (*EncryptedAssetsOnDisk, error) {
	defaultTokensFile := ""
	defaultServiceAccountKey := "<<<" + filepath.Join(dirname, "apiserver-key.pem")
	defaultTLSBootstrapToken, err := RandomTokenString()
	if err != nil {
		return nil, err
	}

	defaultEncryptionConfig, err := EncryptionConfig()
	if err != nil {
		return nil, err
	}
	r := new(EncryptedAssetsOnDisk)

	type entry struct {
		name          string
		data          *EncryptedFile
		defaultValue  *string
		readEncrypted bool
		expiryCheck   bool
	}

	files := []entry{
		{name: "tokens.csv", data: &r.AuthTokens, defaultValue: &defaultTokensFile, readEncrypted: true, expiryCheck: false},
		{name: "kubelet-tls-bootstrap-token", data: &r.TLSBootstrapToken, defaultValue: &defaultTLSBootstrapToken, readEncrypted: true, expiryCheck: false},
		{name: "encryption-config.yaml", data: &r.EncryptionConfig, defaultValue: &defaultEncryptionConfig, readEncrypted: true, expiryCheck: false},
	}

	if manageCertificates {
		files = append(files, []entry{
			{name: "ca.pem", data: &r.CACert, defaultValue: nil, readEncrypted: false, expiryCheck: true},
			{name: "worker-ca.pem", data: &r.WorkerCACert, defaultValue: nil, readEncrypted: false, expiryCheck: true},
			{name: "apiserver.pem", data: &r.APIServerCert, defaultValue: nil, readEncrypted: false, expiryCheck: true},
			{name: "apiserver-key.pem", data: &r.APIServerKey, defaultValue: nil, readEncrypted: true, expiryCheck: false},
			{name: "kube-controller-manager.pem", data: &r.KubeControllerManagerCert, defaultValue: nil, readEncrypted: false, expiryCheck: true},
			{name: "kube-controller-manager-key.pem", data: &r.KubeControllerManagerKey, defaultValue: nil, readEncrypted: true, expiryCheck: false},
			{name: "kube-scheduler.pem", data: &r.KubeSchedulerCert, defaultValue: nil, readEncrypted: false, expiryCheck: true},
			{name: "kube-scheduler-key.pem", data: &r.KubeSchedulerKey, defaultValue: nil, readEncrypted: true, expiryCheck: false},
			{name: "worker.pem", data: &r.WorkerCert, defaultValue: nil, readEncrypted: false, expiryCheck: true},
			{name: "worker-key.pem", data: &r.WorkerKey, defaultValue: nil, readEncrypted: true, expiryCheck: false},
			{name: "admin.pem", data: &r.AdminCert, defaultValue: nil, readEncrypted: false, expiryCheck: false},
			{name: "admin-key.pem", data: &r.AdminKey, defaultValue: nil, readEncrypted: true, expiryCheck: false},
			{name: "etcd.pem", data: &r.EtcdCert, defaultValue: nil, readEncrypted: false, expiryCheck: true},
			{name: "etcd-key.pem", data: &r.EtcdKey, defaultValue: nil, readEncrypted: true, expiryCheck: false},
			{name: "etcd-client.pem", data: &r.EtcdClientCert, defaultValue: nil, readEncrypted: false, expiryCheck: true},
			{name: "etcd-client-key.pem", data: &r.EtcdClientKey, defaultValue: nil, readEncrypted: true, expiryCheck: false},
			{name: "etcd-trusted-ca.pem", data: &r.EtcdTrustedCA, defaultValue: nil, readEncrypted: false, expiryCheck: true},
			{name: "apiserver-aggregator-key.pem", data: &r.APIServerAggregatorKey, defaultValue: nil, readEncrypted: true, expiryCheck: false},
			{name: "apiserver-aggregator.pem", data: &r.APIServerAggregatorCert, defaultValue: nil, readEncrypted: false, expiryCheck: true},
			{name: "service-account-key.pem", data: &r.ServiceAccountKey, defaultValue: &defaultServiceAccountKey, readEncrypted: true, expiryCheck: false},
		}...)

		if caKeyRequiredOnController {
			files = append(files, entry{name: "worker-ca-key.pem", data: &r.WorkerCAKey, defaultValue: nil, readEncrypted: true, expiryCheck: false})
		}

		if kiamEnabled {
			files = append(files, entry{name: "kiam-server-key.pem", data: &r.KIAMServerKey, defaultValue: nil, readEncrypted: true, expiryCheck: false})
			files = append(files, entry{name: "kiam-server.pem", data: &r.KIAMServerCert, defaultValue: nil, readEncrypted: false, expiryCheck: true})
			files = append(files, entry{name: "kiam-agent-key.pem", data: &r.KIAMAgentKey, defaultValue: nil, readEncrypted: true, expiryCheck: false})
			files = append(files, entry{name: "kiam-agent.pem", data: &r.KIAMAgentCert, defaultValue: nil, readEncrypted: false, expiryCheck: true})
			files = append(files, entry{name: "kiam-ca.pem", data: &r.KIAMCACert, defaultValue: nil, readEncrypted: false, expiryCheck: true})
		}
	}

	for _, file := range files {
		path := filepath.Join(dirname, file.name)
		if file.readEncrypted {
			data, err := store.EncryptedCredentialFromPath(path, file.defaultValue)
			if err != nil {
				return nil, fmt.Errorf("error encrypting %s: %v", path, err)
			}

			*file.data = *data
			if err := data.Persist(); err != nil {
				return nil, fmt.Errorf("error persisting %s: %v", path, err)
			}
		} else {
			raw, err := RawCredentialFileFromPath(path, file.defaultValue)
			if err != nil {
				return nil, fmt.Errorf("error reading credential file %s: %v", path, err)
			}
			if file.expiryCheck {
				certs, err := pki.CertificatesFromBytes(raw.Bytes())
				if err != nil {
					return nil, err
				}
				for _, cert := range certs {
					if cert.IsExpired() {
						return nil, fmt.Errorf("the following certificate in file %s has expired:-\n\n%s", path, cert)
					}
				}
			}
			(*file.data).SetBytes(raw.Bytes())
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
		{"kube-controller-manager.pem", r.KubeControllerManagerCert, true, ""},
		{"kube-controller-manager-key.pem", r.KubeControllerManagerKey, true, ""},
		{"kube-scheduler.pem", r.KubeSchedulerCert, true, ""},
		{"kube-scheduler-key.pem", r.KubeSchedulerKey, true, ""},
		{"worker.pem", r.WorkerCert, true, ""},
		{"worker-key.pem", r.WorkerKey, true, ""},
		{"admin.pem", r.AdminCert, true, ""},
		{"admin-key.pem", r.AdminKey, true, ""},
		{"etcd.pem", r.EtcdCert, true, ""},
		{"etcd-key.pem", r.EtcdKey, true, ""},
		{"etcd-client.pem", r.EtcdClientCert, true, ""},
		{"etcd-client-key.pem", r.EtcdClientKey, true, ""},
		{"etcd-trusted-ca.pem", r.EtcdTrustedCA, true, "ca.pem"},
		{"apiserver-aggregator-key.pem", r.APIServerAggregatorKey, true, ""},
		{"apiserver-aggregator.pem", r.APIServerAggregatorCert, true, ""},
		{"kubelet-tls-bootstrap-token", r.TLSBootstrapToken, true, ""},
		{"service-account-key.pem", r.ServiceAccountKey, true, "apiserver-key.pem"},

		// Content entirely provided by user, so do not overwrite it if
		// the file already exists
		{"tokens.csv", r.AuthTokens, false, ""},
		{"encryption-config.yaml", r.EncryptionConfig, false, ""},
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
					logger.Infof("Removing a file at %s\n", from)
					if err := os.Remove(from); err != nil {
						return err
					}
				}

				if symlinkExists {
					logger.Infof("Removing a symlink at %s\n", from)
					if err := os.Remove(from); err != nil {
						return err
					}
				}

				logger.Infof("Creating a symlink from %s to %s\n", from, to)
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
		logger.Infof("Writing %d bytes to %s\n", len(asset.data), path)
		if err := ioutil.WriteFile(path, asset.data, 0600); err != nil {
			return err
		}
	}

	return nil
}

func (r *EncryptedAssetsOnDisk) WriteToDir(dirname string, kiamEnabled bool) error {
	type asset struct {
		name string
		data EncryptedFile
	}
	assets := []asset{
		{"ca.pem", r.CACert},
		{"ca-key.pem", r.CAKey},
		{"worker-ca.pem", r.WorkerCACert},
		{"worker-ca-key.pem", r.WorkerCAKey},
		{"apiserver.pem", r.APIServerCert},
		{"apiserver-key.pem", r.APIServerKey},
		{"kube-controller-manager.pem", r.KubeControllerManagerCert},
		{"kube-controller-manager-key.pem", r.KubeControllerManagerKey},
		{"kube-scheduler.pem", r.KubeSchedulerCert},
		{"kube-scheduler-key.pem", r.KubeSchedulerKey},
		{"worker.pem", r.WorkerCert},
		{"worker-key.pem", r.WorkerKey},
		{"admin.pem", r.AdminCert},
		{"admin-key.pem", r.AdminKey},
		{"etcd.pem", r.EtcdCert},
		{"etcd-key.pem", r.EtcdKey},
		{"etcd-client.pem", r.EtcdClientCert},
		{"etcd-client-key.pem", r.EtcdClientKey},
		{"etcd-trusted-ca.pem", r.EtcdTrustedCA},
		{"apiserver-aggregator-key.pem", r.APIServerAggregatorKey},
		{"apiserver-aggregator.pem", r.APIServerAggregatorCert},
		{"service-account-key.pem", r.ServiceAccountKey},

		{"tokens.csv", r.AuthTokens},
		{"kubelet-tls-bootstrap-token", r.TLSBootstrapToken},
		{"encryption-config.yaml", r.EncryptionConfig},
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
	compact := func(c PlaintextFile) string {
		// Nothing to compact
		if len(c.Bytes()) == 0 {
			return ""
		}

		if err != nil {
			return ""
		}

		var out string
		if out, err = gzipcompressor.BytesToGzippedBase64String(c.Bytes()); err != nil {
			return ""
		}
		return out
	}
	compactAssets := CompactAssets{
		CACert:       compact(r.CACert), // why no CAKey here?
		WorkerCACert: compact(r.WorkerCACert),
		//WorkerCAKey:    compact(r.WorkerCAKey),
		APIServerCert:             compact(r.APIServerCert),
		APIServerKey:              compact(r.APIServerKey),
		KubeControllerManagerCert: compact(r.KubeControllerManagerCert),
		KubeControllerManagerKey:  compact(r.KubeControllerManagerKey),
		KubeSchedulerCert:         compact(r.KubeSchedulerCert),
		KubeSchedulerKey:          compact(r.KubeSchedulerKey),
		WorkerCert:                compact(r.WorkerCert),
		WorkerKey:                 compact(r.WorkerKey),
		AdminCert:                 compact(r.AdminCert),
		AdminKey:                  compact(r.AdminKey),
		EtcdCert:                  compact(r.EtcdCert),
		EtcdClientCert:            compact(r.EtcdClientCert),
		EtcdClientKey:             compact(r.EtcdClientKey),
		EtcdKey:                   compact(r.EtcdKey),
		EtcdTrustedCA:             compact(r.EtcdTrustedCA),
		APIServerAggregatorCert:   compact(r.APIServerAggregatorCert),
		APIServerAggregatorKey:    compact(r.APIServerAggregatorKey),
		KIAMAgentCert:             compact(r.KIAMAgentCert),
		KIAMAgentKey:              compact(r.KIAMAgentKey),
		KIAMServerCert:            compact(r.KIAMServerCert),
		KIAMServerKey:             compact(r.KIAMServerKey),
		KIAMCACert:                compact(r.KIAMCACert),
		ServiceAccountKey:         compact(r.ServiceAccountKey),

		AuthTokens:        compact(r.AuthTokens),
		TLSBootstrapToken: compact(r.TLSBootstrapToken),
		EncryptionConfig:  compact(r.EncryptionConfig),
	}
	if err != nil {
		return nil, err
	}
	return &compactAssets, nil
}

func (r *EncryptedAssetsOnDisk) Compact() (*CompactAssets, error) {
	var err error
	compact := func(c EncryptedFile) string {
		// Nothing to compact
		if len(c.Bytes()) == 0 {
			return ""
		}

		if err != nil {
			return ""
		}

		var out string
		if out, err = gzipcompressor.BytesToGzippedBase64String(c.Bytes()); err != nil {
			return ""
		}
		return out
	}
	compactAssets := CompactAssets{
		CACert:                    compact(r.CACert),
		CAKey:                     compact(r.CAKey),
		WorkerCACert:              compact(r.WorkerCACert),
		WorkerCAKey:               compact(r.WorkerCAKey),
		APIServerCert:             compact(r.APIServerCert),
		APIServerKey:              compact(r.APIServerKey),
		KubeControllerManagerCert: compact(r.KubeControllerManagerCert),
		KubeControllerManagerKey:  compact(r.KubeControllerManagerKey),
		KubeSchedulerCert:         compact(r.KubeSchedulerCert),
		KubeSchedulerKey:          compact(r.KubeSchedulerKey),
		WorkerCert:                compact(r.WorkerCert),
		WorkerKey:                 compact(r.WorkerKey),
		AdminCert:                 compact(r.AdminCert),
		AdminKey:                  compact(r.AdminKey),
		EtcdCert:                  compact(r.EtcdCert),
		EtcdClientCert:            compact(r.EtcdClientCert),
		EtcdClientKey:             compact(r.EtcdClientKey),
		EtcdKey:                   compact(r.EtcdKey),
		EtcdTrustedCA:             compact(r.EtcdTrustedCA),
		APIServerAggregatorCert:   compact(r.APIServerAggregatorCert),
		APIServerAggregatorKey:    compact(r.APIServerAggregatorKey),
		KIAMAgentKey:              compact(r.KIAMAgentKey),
		KIAMAgentCert:             compact(r.KIAMAgentCert),
		KIAMServerKey:             compact(r.KIAMServerKey),
		KIAMServerCert:            compact(r.KIAMServerCert),
		KIAMCACert:                compact(r.KIAMCACert),
		ServiceAccountKey:         compact(r.ServiceAccountKey),

		AuthTokens:        compact(r.AuthTokens),
		TLSBootstrapToken: compact(r.TLSBootstrapToken),
		EncryptionConfig:  compact(r.EncryptionConfig),
	}
	if err != nil {
		return nil, err
	}
	return &compactAssets, nil
}

type KMSConfig struct {
	KMSSvc    KMSEncryptionService
	KMSKeyARN string
}

func (c KMSConfig) Encryptor() Encryptor {
	return KMSEncryptor{
		KmsKeyARN: c.KMSKeyARN,
		KmsSvc:    c.KMSSvc,
	}
}

func (c KMSConfig) Store() Store {
	return Store{
		Encryptor: c.Encryptor(),
	}
}

func NewKMSConfig(kmsKeyARN string, encSvc KMSEncryptionService, session *session.Session) KMSConfig {
	var svc KMSEncryptionService
	if encSvc != nil {
		svc = encSvc
	} else {
		svc = kms.New(session)
	}
	return KMSConfig{
		KMSSvc:    svc,
		KMSKeyARN: kmsKeyARN,
	}
}

func ReadOrCreateEncryptedAssets(tlsAssetsDir string, manageCertificates bool, caKeyRequiredOnController bool, kiamEnabled bool, kmsConfig KMSConfig) (*EncryptedAssetsOnDisk, error) {
	store := kmsConfig.Store()

	return ReadOrEncryptAssets(tlsAssetsDir, manageCertificates, caKeyRequiredOnController, kiamEnabled, store)
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

func RandomTokenString() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func EncryptionConfig() (string, error) {
	secret, err := RandomTokenString()
	if err != nil {
		return "", err
	}
	config := `kind: EncryptionConfig
apiVersion: v1
resources:
  - resources:
    - secrets
    providers:
    - aescbc:
        keys:
        - name: default
          secret: %s
    - identity: {}
`

	return fmt.Sprintf(config, secret), nil
}

func (a *CompactAssets) HasAuthTokens() bool {
	return len(a.AuthTokens) > 0
}

func (a *CompactAssets) HasTLSBootstrapToken() bool {
	return len(a.TLSBootstrapToken) > 0
}
