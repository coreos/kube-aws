package api

import (
	"log"

	"github.com/coreos/coreos-cloudinit/config/validate"
	"github.com/kubernetes-incubator/kube-aws/filereader/texttemplate"

	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"

	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"text/template"
)

// UserDataValidateFunc returns error if templated Part content doesn't pass validation
type UserDataValidateFunc func(content []byte) error

const (
	USERDATA_S3       = "s3"
	USERDATA_INSTANCE = "instance"

	USERDATA_INSTANCE_SCRIPT = "instance-script"
)

// UserData represents userdata which might be split across multiple storage types
type UserData struct {
	Parts map[string]*UserDataPart
	Path  string
}

type UserDataPart struct {
	Asset    Asset
	tmpl     *template.Template
	tmplData interface{}
	validate UserDataValidateFunc
}

type PartDesc struct {
	templateName string
	validateFunc UserDataValidateFunc
}

var (
	defaultParts = []PartDesc{
		{USERDATA_INSTANCE, validateNone},
		{USERDATA_INSTANCE_SCRIPT, validateNone},
		{USERDATA_S3, validateCoreosCloudInit}}
)

type userDataOpt struct {
	Parts []PartDesc // userdata Parts in template file
}

type UserDataOption func(*userDataOpt)

// Parts to find in UserData template file
func UserDataPartsOpt(Parts ...PartDesc) UserDataOption {
	return func(o *userDataOpt) {
		o.Parts = Parts
	}
}

// NewUserDataFromTemplateFile creates userdata struct from template file.
// Template file is expected to have defined subtemplates (Parts) which are of various part and storage types
// TODO Extract this out of the clusterapi package as this is an "implementation"
func NewUserDataFromTemplateFile(templateFile string, context interface{}, opts ...UserDataOption) (UserData, error) {
	v := UserData{Parts: make(map[string]*UserDataPart), Path: templateFile}

	funcs := template.FuncMap{
		"self": func() UserData { return v },
		// we add 'extra' stub so templates can be parsed successfully
		"extra": func() (r string) { panic("[bug] Stub 'extra' was not replaced") },
	}

	tmpl, err := texttemplate.ParseFile(templateFile, funcs)
	if err != nil {
		return UserData{}, err
	}

	var o userDataOpt
	for _, opt := range opts {
		opt(&o)
	}

	if len(o.Parts) == 0 {
		o.Parts = defaultParts
	}

	for _, p := range o.Parts {
		if p.validateFunc == nil {
			return UserData{}, fmt.Errorf("ValidateFunc must not be nil. Use 'validateNone' if you don't require part validation")
		}
		t := tmpl.Lookup(p.templateName)
		if t == nil {
			return UserData{}, fmt.Errorf("Can't find requested template %s in %s", p.templateName, templateFile)
		}

		v.Parts[p.templateName] = &UserDataPart{
			tmpl:     t,
			tmplData: context,
			validate: p.validateFunc,
		}
	}
	return v, nil
}

func (self UserDataPart) Base64(compress bool, extra ...map[string]interface{}) (string, error) {
	content, err := self.Template(extra...)
	if err != nil {
		return "", err
	}
	if compress {
		return gzipcompressor.StringToGzippedBase64String(content)
	} else {
		return base64.StdEncoding.EncodeToString([]byte(content)), nil
	}
}

func (self UserDataPart) Template(extra ...map[string]interface{}) (string, error) {
	buf := bytes.Buffer{}
	funcs := template.FuncMap{}
	switch len(extra) {
	case 0:
	case 1:
		funcs["extra"] = func() map[string]interface{} { return extra[0] }
	default:
		return "", fmt.Errorf("Provide single extra context")
	}

	if err := self.tmpl.Funcs(funcs).Execute(&buf, self.tmplData); err != nil {
		return "", err
	}

	result := buf.String()

	if len(result) == 0 {
		return "", fmt.Errorf("failed to render template: result should'nt be empty for asset: %s", self.Asset.Key)
	}

	// we validate userdata at render time, because we need to wait for
	// optional extra context to produce final output
	return result, self.validate(buf.Bytes())
}

func validateCoreosCloudInit(content []byte) error {
	report, err := validate.Validate(content)
	if err != nil {
		return err
	}
	errors := []string{}
	for _, entry := range report.Entries() {
		errors = append(errors, fmt.Sprintf("%+v", entry))
	}
	if len(errors) > 0 {
		log.Printf("Bad cloud-config:-\n%s\n", content)
		return fmt.Errorf("cloud-config validation errors:\n%s\n", strings.Join(errors, "\n"))
	}
	return nil
}

func validateNone([]byte) error {
	return nil
}
