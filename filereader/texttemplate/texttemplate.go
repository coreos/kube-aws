package texttemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"text/template"

	"github.com/Masterminds/semver"
	"github.com/Masterminds/sprig"
	"github.com/kubernetes-incubator/kube-aws/fingerprint"
	"github.com/kubernetes-incubator/kube-aws/tmpl"
)

func ParseFile(filename string, funcs template.FuncMap) (*template.Template, error) {
	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return Parse(filename, string(raw), funcs)
}

var funcs2 = template.FuncMap{
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
	"fingerprint": func(data string) string {
		return fingerprint.SHA256(data)
	},
	"toLabel": func(data string) string {
		reg := regexp.MustCompile("[^a-z0-9A-Z_.-]")
		return reg.ReplaceAllString(data, "_")
	},
	"EbsOptimized": func(instanceType string) bool {
		// Amazon instance series that do not support EBS optimize
		nonSupportedSeries := map[string]bool{"t1": true, "t2": true}
		// Amazon instance types that do not support EBS optimize
		nonSupportedTypes := map[string]bool{
			"c1.medium": true,
			"c3.large":  true, "c3.8xlarge": true,
			"cc2.8xlarge": true,
			"cr1.8xlarge": true,
			"g2.8xlarge":  true,
			"i2.8xlarge":  true,
			"m1.small":    true, "m1.medium": true,
			"m2.xlarge": true,
			"m3.medium": true, "m3.large": true,
			"r3.large": true, "r3.8xlarge": true}
		series := strings.Split(instanceType, ".")

		return !nonSupportedSeries[series[0]] && !nonSupportedTypes[instanceType]
	},
	"checkVersion": func(constraint string, versionToCheck string) (bool, error) {
		c, err := semver.NewConstraint(constraint)
		if err != nil {
			return false, err
		}
		v, err := semver.NewVersion(versionToCheck)
		if err != nil {
			return false, err
		}
		return c.Check(v), nil
	},
}

func Parse(name string, raw string, funcs template.FuncMap) (*template.Template, error) {
	t, err := template.New(name).Option("missingkey=error").Funcs(sprig.HermeticTxtFuncMap()).Funcs(tmpl.New().CreateFuncMap()).Funcs(funcs).Funcs(funcs2).Parse(raw)
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
