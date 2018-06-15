package helper

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

type TestPlugin struct {
	Name  string
	Yaml  string
	Files map[string]string
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

		files := p.Files
		if files == nil {
			files = map[string]string{}
		}
		for relFilePath, content := range files {
			absFilePath := filepath.Join(pluginDir, relFilePath)
			absDirPath := filepath.Dir(absFilePath)
			if err := os.MkdirAll(absDirPath, 0755); err != nil {
				panic(err)
			}
			if err := ioutil.WriteFile(absFilePath, []byte(content), 0644); err != nil {
				panic(err)
			}
		}
	}

	fn()
}
