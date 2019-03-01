package tmpl

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type Context struct {
	BasePath   string
	FsReadFile func(string) ([]byte, error)
}

func New() *Context {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return &Context{
		BasePath:   dir,
		FsReadFile: ioutil.ReadFile,
	}
}

func (c *Context) CreateFuncMap() template.FuncMap {
	return template.FuncMap{
		"readFile": c.ReadFile,

		"indent": indent,
	}
}

func (c *Context) ReadFile(filename string) (string, error) {
	path := filepath.Join(c.BasePath, filename)

	bytes, err := c.FsReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func indent(spaces int, input string) string {
	idt := strings.Repeat(" ", spaces)

	buf := new(bytes.Buffer)
	for _, line := range strings.Split(input, "\n") {
		_, err := buf.WriteString(idt + line + "\n")
		if err != nil {
			panic(err)
		}
	}
	return buf.String()
}

// WriteTemplateWithOptions parses the template with its options
// and writes the result to the provided Writer
func WriteTemplateWithOptions(w io.Writer, fileTemplate string, templateOpts interface{}) error {
	cfgTemplate, err := template.ParseFiles(fileTemplate)
	if err != nil {
		return err
	}

	return cfgTemplate.Execute(w, templateOpts)
}
