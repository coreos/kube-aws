package texttemplate

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"text/template"
)

func GetBytesBuffer(filename string, data interface{}) (*bytes.Buffer, error) {
	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	funcMap := template.FuncMap{
		"sha1":  func(v string) string { return fmt.Sprintf("%x", sha1.Sum([]byte(v))) },
		"minus": func(a, b int) int { return a - b },
	}

	tmpl, err := template.New(filename).Funcs(funcMap).Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var buff bytes.Buffer
	if err := tmpl.Execute(&buff, data); err != nil {
		return nil, err
	}
	return &buff, nil
}

func GetString(filename string, data interface{}) (string, error) {
	buf, err := GetBytesBuffer(filename, data)

	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
