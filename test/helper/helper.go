package helper

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
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
	dir, err := ioutil.TempDir("", "dummy-credentials")

	if err != nil {
		panic(err)
	}

	// Remove all the contents in the dir including *.pem.enc created by ReadOrUpdateCompactAssets()
	// Otherwise we end up with a lot of garbage directories we failed to remove as they aren't empty in
	// config/temp, nodepool/config/temp, test/integration/temp
	defer os.RemoveAll(dir)

	for _, pairName := range []string{"ca", "apiserver", "worker", "admin", "etcd", "etcd-client", "dex"} {
		certFile := fmt.Sprintf("%s/%s.pem", dir, pairName)
		if err := ioutil.WriteFile(certFile, []byte("dummycert"), 0644); err != nil {
			panic(err)
		}
		defer os.Remove(certFile)

		keyFile := fmt.Sprintf("%s/%s-key.pem", dir, pairName)
		if err := ioutil.WriteFile(keyFile, []byte("dummykey"), 0644); err != nil {
			panic(err)
		}
		defer os.Remove(keyFile)
	}

	fn(dir)
}

type TestPlugin struct {
	Name string
	Yaml string
}

func WithPlugins(plugins []TestPlugin, fn func()) {
	dir, err := filepath.Abs("./")
	if err != nil {
		panic(err)
	}
	pluginsDir := path.Join(dir, "plugins")
	if err := os.Mkdir(pluginsDir, 0755); err != nil {
		panic(err)
	}

	defer os.RemoveAll(pluginsDir)

	for _, p := range plugins {
		pluginDir := path.Join(pluginsDir, p.Name)
		if err := os.Mkdir(pluginDir, 0755); err != nil {
			panic(err)
		}

		pluginYamlFile := path.Join(pluginDir, "plugin.yaml")
		if err := ioutil.WriteFile(pluginYamlFile, []byte(p.Yaml), 0644); err != nil {
			panic(err)
		}
	}

	fn()
}
