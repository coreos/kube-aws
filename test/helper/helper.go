package helper

import (
	"fmt"
	"io/ioutil"
	"os"
)

const (
	dummyKey = `-----BEGIN RSA PRIVATE KEY-----
ZHVtbXkK
-----END RSA PRIVATE KEY-----`

	dummyCert = `-----BEGIN CERTIFICATE-----
MIIBvjCCAWgCCQDQ4pUwqdLIIDANBgkqhkiG9w0BAQsFADBlMQswCQYDVQQGEwJV
UzESMBAGA1UECAwJQW50YXJ0aWNhMRowGAYDVQQKDBFUZXN0IFdpZGdldHMgSW5j
LjERMA8GA1UECwwIVGVzdCBMYWIxEzARBgNVBAMMCmR1bW15LWNlcnQwIBcNMTgw
NDMwMDk1NDExWhgPMjUxNzEyMzAwOTU0MTFaMGUxCzAJBgNVBAYTAlVTMRIwEAYD
VQQIDAlBbnRhcnRpY2ExGjAYBgNVBAoMEVRlc3QgV2lkZ2V0cyBJbmMuMREwDwYD
VQQLDAhUZXN0IExhYjETMBEGA1UEAwwKZHVtbXktY2VydDBcMA0GCSqGSIb3DQEB
AQUAA0sAMEgCQQDgd2lsmEBDXMxZsaFUSwnC/FF3x/62SIb3/f8mrGrBtb6Vim11
s7T0zFCm9cWbTi63bzWRFs3gP2FwwU1MF5RDAgMBAAEwDQYJKoZIhvcNAQELBQAD
QQA0bLc3+5kpZuJaAK+C0XvTPZFz8Vx1nv8YnwoIJdEvvGOPGAqvrA8Y0Fvs7L11
Z3leoFbVQmybV7EcduIrOANA
-----END CERTIFICATE-----`
)

func WithTempDir(fn func(dir string)) {
	dir, err := ioutil.TempDir("", "test-temp-dir")

	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(dir)

	fn(dir)
}

func WithDummyCredentials(fn func(dir string)) {
	withDummyCredentials(true, fn)
}

func WithDummyCredentialsButCAKey(fn func(dir string)) {
	withDummyCredentials(false, fn)
}

func withDummyCredentials(alsoWriteCAKey bool, fn func(dir string)) {
	dir, err := ioutil.TempDir("", "dummy-credentials")

	if err != nil {
		panic(err)
	}

	// Remove all the contents in the dir including *.pem.enc created by ReadOrUpdateCompactAssets()
	// Otherwise we end up with a lot of garbage directories we failed to remove as they aren't empty in
	// config/temp, nodepool/config/temp, test/integration/temp
	defer os.RemoveAll(dir)

	for _, pairName := range []string{"ca", "apiserver", "kube-controller-manager", "kube-scheduler", "worker", "admin", "etcd", "etcd-client", "apiserver-aggregator"} {
		certFile := fmt.Sprintf("%s/%s.pem", dir, pairName)
		if err := ioutil.WriteFile(certFile, []byte(dummyCert), 0644); err != nil {
			panic(err)
		}
		defer os.Remove(certFile)

		if pairName != "ca" || alsoWriteCAKey {
			keyFile := fmt.Sprintf("%s/%s-key.pem", dir, pairName)
			if err := ioutil.WriteFile(keyFile, []byte(dummyKey), 0644); err != nil {
				panic(err)
			}
			defer os.Remove(keyFile)
		}
	}

	type symlink struct {
		from string
		to   string
	}

	symlinks := []symlink{
		{"ca.pem", "worker-ca.pem"},
		{"ca.pem", "etcd-trusted-ca.pem"},
	}

	if alsoWriteCAKey {
		symlinks = append(symlinks, symlink{"ca-key.pem", "worker-ca-key.pem"})
	}

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	if err := os.Chdir(dir); err != nil {
		panic(err)
	}

	for _, sl := range symlinks {
		from := sl.from
		to := sl.to

		if _, err := os.Lstat(to); err == nil {
			if err := os.Remove(to); err != nil {
				panic(err)
			}
		}

		if err := os.Symlink(from, to); err != nil {
			panic(err)
		}
		defer os.Remove(to)
	}

	if err := os.Chdir(wd); err != nil {
		panic(err)
	}

	fn(dir)
}
