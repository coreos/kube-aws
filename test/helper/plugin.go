package helper

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"testing"
)

type TestPlugin struct {
	Name  string
	Yaml  string
	Files map[string]string
}

func isNotExist(err error) bool {
	return err == syscall.ENOENT || err == os.ErrNotExist
}

func WithPlugins(t *testing.T, plugins []TestPlugin, fn func()) {
	dir, err := filepath.Abs("./")
	if err != nil {
		panic(err)
	}
	pluginsDir := path.Join(dir, "plugins")

	//if _, err := os.Stat(pluginsDir); isNotExist(err) {
	if err := os.Mkdir(pluginsDir, 0755); err != nil {
		t.Errorf("%+v", err)
		t.FailNow()
	}
	//}

	defer os.RemoveAll(pluginsDir)

	for _, p := range plugins {
		pluginDir := path.Join(pluginsDir, p.Name)
		if err := os.Mkdir(pluginDir, 0755); err != nil {
			t.Errorf("%+v", err)
			t.FailNow()
		}

		pluginYamlFile := path.Join(pluginDir, "plugin.yaml")
		if err := ioutil.WriteFile(pluginYamlFile, []byte(p.Yaml), 0644); err != nil {
			t.Errorf("%+v", err)
			t.FailNow()
		}

		files := p.Files
		if files == nil {
			files = map[string]string{}
		}
		for relFilePath, content := range files {
			absFilePath := filepath.Join(pluginDir, relFilePath)
			absDirPath := filepath.Dir(absFilePath)
			if err := os.MkdirAll(absDirPath, 0755); err != nil {
				t.Errorf("%+v", err)
				t.FailNow()
			}
			if err := ioutil.WriteFile(absFilePath, []byte(content), 0644); err != nil {
				t.Errorf("%+v", err)
				t.FailNow()
			}
		}
	}

	fn()
}
