package api

type PKI struct {
	KeyPairs []KeyPairSpec `yaml:"keypairs,omitempty"`
}
