package texttemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Masterminds/sprig"
	"io/ioutil"
	"text/template"
)

func ParseFile(filename string, funcs template.FuncMap) (*template.Template, error) {
	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return Parse(filename, string(raw), funcs)
}

func Parse(name string, raw string, funcs template.FuncMap) (*template.Template, error) {
	funcs2 := template.FuncMap{
		"checkSizeLessThan": func(size int, content string) (string, error) {
			if len(content) >= size {
				return "", fmt.Errorf("Content length exceeds maximum size %d", size)
			}
			return content, nil
		},
		"toJSON": func(v interface{}) (string, error) {
			data, err := json.Marshal(v)
			return string(data), err
		},
		"execTemplate": func(name string, ctx interface{}) (string, error) {
			panic("[bug] Stub 'execTemplate' was not replaced")
		},
	}

	t, err := template.New(name).Funcs(sprig.HermeticTxtFuncMap()).Funcs(funcs).Funcs(funcs2).Parse(raw)
	if err == nil {
		t = t.Funcs(template.FuncMap{
			"execTemplate": func(name string, ctx interface{}) (string, error) {
				b := bytes.Buffer{}
				err := t.ExecuteTemplate(&b, name, ctx)
				return b.String(), err
			},
		})
	}
	return t, err
}

func GetBytesBuffer(filename string, data interface{}) (*bytes.Buffer, error) {
	tmpl, err := ParseFile(filename, nil)
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
