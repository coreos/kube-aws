package model

import (
	"github.com/stretchr/testify/assert"

	"fmt"
	"io/ioutil"
	"os"
	"testing"
)

const (
	s3Body = `#cloud-config
---
coreos: {}
`
)

func mkTmpl(t, b string) string {
	return fmt.Sprintf(`{{define "%s"}}%s{{end}}`, t, b)
}

var (
	mkInstance      = func(b string) string { return mkTmpl(USERDATA_INSTANCE, b) }
	mkS3            = func(b string) string { return mkTmpl(USERDATA_S3, b) }
	tS3             = mkS3(s3Body)
	tInstance       = mkInstance("INSTANCE BODY")
	instanceOnlyOpt = []UserDataOption{UserDataPartsOpt(PartDesc{USERDATA_INSTANCE, validateNone})}
)

type Expectation func(assert *assert.Assertions, ud UserData, err error)

func TestUserDataNew(t *testing.T) {
	tests := []struct {
		name     string
		template string
		opts     []UserDataOption
		exp      Expectation
	}{
		{"simple", tS3 + tInstance, nil,
			func(a *assert.Assertions, ud UserData, err error) {
				a.NoError(err)
				a.NotEmpty(ud)

				a.Len(ud.Parts, 2)
				if a.Contains(ud.Parts, USERDATA_S3) {
					p, _ := ud.Parts[USERDATA_S3]
					a.Equal(p.Content, s3Body)
				}

				if a.Contains(ud.Parts, USERDATA_INSTANCE) {
					p, _ := ud.Parts[USERDATA_INSTANCE]
					a.Equal(p.Content, "INSTANCE BODY")
				}
			},
		},
		{"missing S3", tInstance, nil,
			func(a *assert.Assertions, ud UserData, err error) {
				if a.Error(err) {
					a.Contains(err.Error(), "Can't find requested template")
				}
			},
		},
		{"extra", mkInstance("{{extra.Body}}"), []UserDataOption{
				func(o *UserDataOpt) {
					o.Extra = UserDataTemplateExtraParams{"Bodu": "EXTRA BODY"}
					o.Parts = []PartDesc{{USERDATA_INSTANCE, validateNone}}
				},
			},
			func(a *assert.Assertions, ud UserData, err error) {
				a.NoError(err, "Userdata creation failed")

				p, ok := ud.Parts[USERDATA_INSTANCE]
				if a.True(ok) {
					a.Equal("EXTRA BODY", p.Content)
				}
			},
		},
		{"self", mkInstance("{{if self.Parts}}GOOD{{else}}BAD{{end}}"), instanceOnlyOpt,
			func(a *assert.Assertions, ud UserData, err error) {
				if !a.NoError(err, "Userdata creation failed") {
					return
				}

				p, ok := ud.Parts[USERDATA_INSTANCE]
				if a.True(ok, "Can't find Instance template") {
					a.Equal("GOOD", p.Content, "self function doesn't return our own UserData")
				}
			},
		},
	}

	for _, test := range tests {
		tmpfile, _ := ioutil.TempFile("", "ud")
		tmpfile.WriteString(test.template)
		tmpfile.Close()
		defer os.Remove(tmpfile.Name())
		t.Run(test.name, func(t *testing.T) {
			assetFactory := &TestAssetFactory{}
			ud, err := NewUserDataFromTemplateFile(tmpfile.Name(), nil, assetFactory, test.opts...)
			test.exp(assert.New(t), ud, err)
		})
	}
}

type TestAssetFactory struct {
}

func (f *TestAssetFactory) Create(filename string, content string) (Asset, error) {
	return Asset{
		AssetLocation: AssetLocation{ID: filename},
		Content:       content,
	}, nil
}
