package helper

import (
	"fmt"
	"io/ioutil"
	"os"
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

	for _, pairName := range []string{"ca", "apiserver", "worker", "admin", "etcd", "etcd-client", "kiam-agent", "kiam-server"} {
		certFile := fmt.Sprintf("%s/%s.pem", dir, pairName)
		if err := ioutil.WriteFile(certFile, []byte("dummycert"), 0644); err != nil {
			panic(err)
		}
		defer os.Remove(certFile)

		if pairName != "ca" || alsoWriteCAKey {
			keyFile := fmt.Sprintf("%s/%s-key.pem", dir, pairName)
			if err := ioutil.WriteFile(keyFile, []byte("dummykey"), 0644); err != nil {
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
		{"ca.pem", "kiam-ca.pem"},
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
