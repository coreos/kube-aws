package model

import (
	"github.com/coreos/coreos-cloudinit/config/validate"
	"github.com/kubernetes-incubator/kube-aws/filereader/texttemplate"
	"github.com/kubernetes-incubator/kube-aws/fingerprint"
	"github.com/kubernetes-incubator/kube-aws/gzipcompressor"

	"bytes"
	"encoding/base64"
	"fmt"
	"path"
	"strings"
	"text/template"
)

// UserDataValidateFunc returns error if templated Part content doesn't pass validation
type UserDataValidateFunc func(content []byte) error
type UserDataTemplateExtraParams map[string]interface{}

const (
	USERDATA_S3       = "s3"
	USERDATA_INSTANCE = "instance"
)

// UserData represents userdata which might be split across multiple storage types
type UserData struct {
	Parts            map[string]*UserDataPart
	templateFilePath string
}

type UserDataPart struct {
	Asset   Asset
	Content string
}

type PartDesc struct {
	templateName string
	validateFunc UserDataValidateFunc
}

var (
	defaultParts = []PartDesc{{USERDATA_INSTANCE, validateNone}, {USERDATA_S3, validateCoreosCloudInit}}
)

type UserDataOpt struct {
	Extra UserDataTemplateExtraParams
	Parts []PartDesc // userdata Parts in template file
}

type UserDataOption func(*UserDataOpt)

// Parts to find in UserData template file
func UserDataPartsOpt(Parts ...PartDesc) UserDataOption {
	return func(o *UserDataOpt) {
		o.Parts = Parts
	}
}

func UserDataPartFromTemplate(name string, tmpl *template.Template, tmplData interface{}, validate UserDataValidateFunc, assetFactory AssetFactory, extra ...UserDataTemplateExtraParams) (*UserDataPart, error) {
	content, err := renderUserdataFromTemplate(tmpl, tmplData, validate, extra...)
	if err != nil {
		return nil, err
	}
	nameWithHash := fmt.Sprintf("%s-%s", name, fingerprint.SHA256(content))
	asset, err := assetFactory.Create(nameWithHash, content)
	if err != nil {
		return nil, err
	}
	p := &UserDataPart{
		Content: content,
		Asset:   asset,
	}
	return p, nil
}

// NewUserData creates userdata struct from template file.
// Template file is expected to have defined subtemplates (Parts) which are of various part and storage types
func NewUserDataFromTemplateFile(path string, context interface{}, assetFactory AssetFactory, opts ...UserDataOption) (UserData, error) {
	v := UserData{
		Parts:            make(map[string]*UserDataPart),
		templateFilePath: path,
	}

	funcs := template.FuncMap{
		"self": func() UserData { return v },
		// we add 'extra' stub so templates can be parsed successfully
		"extra": func() (r string) { panic("[bug] Stub 'extra' was not replaced") },
	}

	tmpl, err := texttemplate.Parse(path, funcs)
	if err != nil {
		return UserData{}, err
	}

	var o UserDataOpt
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
		// The template file `tmpl` is composed of multiple sub-templates.
		// Each sub-template `t` is for a single `part` in the userdata
		t := tmpl.Lookup(p.templateName)
		if t == nil {
			return UserData{}, fmt.Errorf("Can't find requested template %s in %s", p.templateName, path)
		}

		part, err := UserDataPartFromTemplate(p.templateName, t, context, p.validateFunc, assetFactory, o.Extra)
		if err != nil {
			return UserData{}, err
		}
		v.Parts[p.templateName] = part
	}
	return v, nil
}

func (self *UserData) templateFileBaseName() string {
	return path.Base(self.templateFilePath)
}

func (self *UserData) s3Part() *UserDataPart {
	return self.Parts[USERDATA_S3]
}

func (self *UserData) S3PartAsset() Asset {
	return self.s3Part().Asset
}

func (self *UserData) S3PartContent() string {
	return self.s3Part().Content
}

func (self UserDataPart) Base64(compress bool) (string, error) {
	content := self.Content
	if compress {
		return gzipcompressor.CompressString(content)
	} else {
		return base64.StdEncoding.EncodeToString([]byte(content)), nil
	}
}

func renderUserdataFromTemplate(tmpl *template.Template, tmplData interface{}, validate UserDataValidateFunc, extra ...UserDataTemplateExtraParams) (string, error) {
	buf := bytes.Buffer{}
	funcs := template.FuncMap{}
	switch len(extra) {
	case 0:
	case 1:
		funcs["extra"] = func() map[string]interface{} { return extra[0] }
	default:
		return "", fmt.Errorf("Provide single extra context")
	}

	if err := tmpl.Funcs(funcs).Execute(&buf, tmplData); err != nil {
		return "", err
	}

	// we validate userdata at render time, because we need to wait for
	// optional extra context to produce final output
	return buf.String(), validate(buf.Bytes())
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
