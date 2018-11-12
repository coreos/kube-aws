package api

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
	mkInstance       = func(b string) string { return mkTmpl(USERDATA_INSTANCE, b) }
	mkInstanceScript = func(b string) string { return mkTmpl(USERDATA_INSTANCE_SCRIPT, b) }
	mkS3             = func(b string) string { return mkTmpl(USERDATA_S3, b) }
	tS3              = mkS3(s3Body)
	tInstance        = mkInstance("INSTANCE BODY")
	tInstanceScript  = mkInstanceScript("INSTANCE SCRIPT BODY")
	instanceOnlyOpt  = []UserDataOption{UserDataPartsOpt(PartDesc{USERDATA_INSTANCE, validateNone})}
)

type Expectation func(assert *assert.Assertions, ud UserData, err error)

func TestUserDataNew(t *testing.T) {
	tests := []struct {
		name     string
		template string
		opts     []UserDataOption
		exp      Expectation
	}{
		{"simple", tS3 + tInstance + tInstanceScript, nil,
			func(a *assert.Assertions, ud UserData, err error) {
				a.NoError(err)
				a.NotEmpty(ud)

				a.Len(ud.Parts, 3)
				if a.Contains(ud.Parts, USERDATA_S3) {
					udata, _ := ud.Parts[USERDATA_S3]
					content, err := udata.Template()
					a.NoError(err)
					a.Equal(content, s3Body)
				}

				if a.Contains(ud.Parts, USERDATA_INSTANCE) {
					udata, _ := ud.Parts[USERDATA_INSTANCE]
					content, err := udata.Template()
					a.NoError(err)
					a.Equal(content, "INSTANCE BODY")
				}

				if a.Contains(ud.Parts, USERDATA_INSTANCE_SCRIPT) {
					udata, _ := ud.Parts[USERDATA_INSTANCE_SCRIPT]
					content, err := udata.Template()
					a.NoError(err)
					a.Equal(content, "INSTANCE SCRIPT BODY")
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
		{"extra", mkInstance("{{extra.Body}}"), instanceOnlyOpt,
			func(a *assert.Assertions, ud UserData, err error) {
				a.NoError(err, "Userdata creation failed")

				p, ok := ud.Parts[USERDATA_INSTANCE]
				if a.True(ok) {
					content, err := p.Template(map[string]interface{}{"Body": "EXTRA BODY"})
					a.NoError(err, "Can't find 'extra' function")
					a.Equal("EXTRA BODY", content)
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
					content, err := p.Template()
					a.NoError(err, "Can't find 'self' function")
					a.Equal("GOOD", content, "self function doesn't return our own UserData")
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
			ud, err := NewUserDataFromTemplateFile(tmpfile.Name(), nil, test.opts...)
			test.exp(assert.New(t), ud, err)
		})
	}
}
