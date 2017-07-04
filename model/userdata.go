package model

import (
	"github.com/coreos/coreos-cloudinit/config/validate"
	"github.com/kubernetes-incubator/kube-aws/filereader/texttemplate"
	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"

	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

// UserDataValidateFunc returns error if templated Part content doesn't pass validation
type UserDataValidateFunc func(content []byte) error

const (
	USERDATA_S3       = "s3"
	USERDATA_INSTANCE = "instance"
)

// UserData represents userdata which might be split across multiple storage types
type UserData struct {
	Parts map[string]*UserDataPart
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
	defaultParts = []PartDesc{{USERDATA_INSTANCE, validateNone}, {USERDATA_S3, validateCoreosCloudInit}}
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

// NewUserData creates userdata struct from template file.
// Template file is expected to have defined subtemplates (Parts) which are of various part and storage types
func NewUserData(templateFile string, context interface{}, opts ...UserDataOption) (UserData, error) {
	v := UserData{make(map[string]*UserDataPart)}

	funcs := template.FuncMap{
		"self": func() UserData { return v },
		// we add 'extra' stub so templates can be parsed successfully
		"extra": func() (r string) { panic("[bug] Stub 'extra' was not replaced") },
	}

	tmpl, err := texttemplate.Parse(templateFile, funcs)
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
		return gzipcompressor.CompressString(content)
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

	// we validate userdata at render time, because we need to wait for
	// optional extra context to produce final output
	return buf.String(), self.validate(buf.Bytes())
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
		return fmt.Errorf("cloud-config validation errors:\n%s\n", strings.Join(errors, "\n"))
	}
	return nil
}

func validateNone([]byte) error {
	return nil
}

func (self UserDataPart) Base64IgnitionConfigRef() (string, error) {
	base64Part, err := self.Base64(false)
	if err != nil {
		return "", fmt.Errorf("failed to generate ignition config: %v", err)
	}

	// TODO Append user-provided ignition config via `ignition.config.append.source`
	// once it supports sourcing from a s3 object
	// https://coreos.com/ignition/docs/0.14.0/configuration-v2_0.html

	head, err := json.Marshal(`{
  "ignition": {
    "version": "2.0.0"
  },
  "storage": {
    "files": [
      {
        "filesystem": "root",
        "path": "/kube-aws-stack-name",
        "mode": 420,
        "contents": {
          "source": "data:;base64,`)
	if err != nil {
		return "", fmt.Errorf("failed to generate ignition config: %v", err)
	}

	tail, err := json.Marshal(
		fmt.Sprintf(`"
        }
      },
      {
        "filesystem": "root",
        "path": "/opt/bin/bootstrap-worker-node",
        "mode": 493,
        "contents": {
          "source": "data:;base64,%s"
        }
      }
    ]
  },
  "systemd": {
    "units": [
      {
        "name": "bootstrap-worker-node.service",
        "enable": true,
        "contents": "[Unit]\nRequires=network-online.target\nAfter=network-online.target\n\n[Service]\nType=oneshot\nRemainAfterExit=yes\nExecStart=/opt/bin/bootstrap-worker-node\n\n[Install]\nWantedBy=multi-user.target"
      }
    ]
  }
}`,
			base64Part,
		),
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate ignition config: %v", err)
	}

	splits := []string{
		string(head),
		`{"Fn::Base64":{"Ref":"AWS::StackName"}}`,
		string(tail),
	}

	// TODO should emit a validation error when sum(sizeOf(split in splits)) > 16384

	return fmt.Sprintf(`{"Fn::Base64": {"Fn::Join": ["", [%s]]} }`, strings.Join(splits, ",")), nil
}
