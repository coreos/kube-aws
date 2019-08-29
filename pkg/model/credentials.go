package model

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/kubernetes-incubator/kube-aws/credential"
	"github.com/kubernetes-incubator/kube-aws/pkg/api"
)

func LoadCredentials(sess *session.Session, cfg *Config, opts api.StackTemplateOptions) (*credential.CompactAssets, error) {
	s := &Context{Session: sess}
	return s.LoadCredentials(cfg, opts)
}

func (s *Context) LoadCredentials(cfg *Config, opts api.StackTemplateOptions) (*credential.CompactAssets, error) {
	if cfg.AssetsEncryptionEnabled() {
		kmsConfig := credential.NewKMSConfig(cfg.KMSKeyARN, s.ProvidedEncryptService, s.Session)
		compactAssets, err := credential.ReadOrCreateCompactAssets(opts.AssetsDir, cfg.ManageCertificates, true, kmsConfig)
		if err != nil {
			return nil, err
		}

		return compactAssets, nil
	} else {
		rawAssets, err := credential.ReadOrCreateUnencryptedCompactAssets(opts.AssetsDir, cfg.ManageCertificates, true)
		if err != nil {
			return nil, err
		}

		return rawAssets, nil
	}
}

func NewCredentialGenerator(c *Config) *credential.Generator {
	r := &credential.Generator{
		TLSCADurationDays:                c.TLSCADurationDays,
		TLSCertDurationDays:              c.TLSCertDurationDays,
		ManageCertificates:               c.ManageCertificates,
		Region:                           c.Region.String(),
		APIServerExternalDNSNames:        c.ExternalDNSNames(),
		APIServerAdditionalDNSSans:       c.CustomApiServerSettings.AdditionalDnsSANs,
		APIServerAdditionalIPAddressSans: c.CustomApiServerSettings.AdditionalIPAddresses,
		EtcdNodeDNSNames:                 c.EtcdCluster().DNSNames(),
		ServiceCIDR:                      c.ServiceCIDR,
	}

	return r
}

func GenerateAssetsOnDisk(sess *session.Session, c *Config, dir string, opts credential.GeneratorOptions) (*credential.RawAssetsOnDisk, error) {
	s := &Context{Session: sess}
	return s.GenerateAssetsOnDisk(c, dir, opts)
}

func (s *Context) GenerateAssetsOnDisk(c *Config, dir string, opts credential.GeneratorOptions) (*credential.RawAssetsOnDisk, error) {
	r := NewCredentialGenerator(c)
	return r.GenerateAssetsOnDisk(dir, opts)
}
