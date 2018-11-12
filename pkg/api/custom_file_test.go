package api

import (
	"testing"

	"github.com/go-yaml/yaml"
	"github.com/stretchr/testify/assert"
)

type TestContextFruit struct {
	ChoiceOfFruit string
}

type TestContextAnimal struct {
	IsADog      bool
	Name        string
	YearOfBirth string
}

var (
	// simple plain file case
	customFileContent = `path: /tmp/testfile
permissions: 0777
content: hello world
`

	// customfile contains a template
	customFileTemplate = `path: /tmp/testfile
permissions: 0777
template: I love {{ .ChoiceOfFruit }}
`
)

func TestCustomFileRendersContent(t *testing.T) {
	cfile := CustomFile{}
	err := yaml.Unmarshal([]byte(customFileContent), &cfile)
	assert.NoError(t, err)
	output, err := cfile.RenderContent(TestContextFruit{})
	assert.NoError(t, err)
	assert.Equal(t, "hello world", output)
}

func TestCustomFileRenderWithEncoding(t *testing.T) {
	helloWorldEncoded := `H4sIAAAAAAAA/8pIzcnJVyjPL8pJAQQAAP//hRFKDQsAAAA=`

	cfile := CustomFile{}
	err := yaml.Unmarshal([]byte(customFileContent), &cfile)
	assert.NoError(t, err)
	output, err := cfile.RenderGzippedBase64Content(TestContextFruit{})
	assert.NoError(t, err)
	assert.Equal(t, helloWorldEncoded, output)
}

func TestCustomFileRenderTemplate(t *testing.T) {
	cfile := CustomFile{}
	err := yaml.Unmarshal([]byte(customFileTemplate), &cfile)
	assert.NoError(t, err)
	output, err := cfile.RenderContent(TestContextFruit{ChoiceOfFruit: "apples"})
	assert.NoError(t, err)
	assert.Equal(t, "I love apples", output)
}

func TestCustomFileRenderTemplateWithEncoding(t *testing.T) {
	iLoveApplesEncoded := `H4sIAAAAAAAA//JUyMkvS1VILCjISS0GBAAA//+rYX5CDQAAAA==`

	cfile := CustomFile{}
	err := yaml.Unmarshal([]byte(customFileTemplate), &cfile)
	assert.NoError(t, err)
	output, err := cfile.RenderGzippedBase64Content(TestContextFruit{ChoiceOfFruit: "apples"})
	assert.NoError(t, err)
	assert.Equal(t, iLoveApplesEncoded, output)
}

func TestCustomBadTemplate(t *testing.T) {
	badTemplate := `path: /tmp/testfile
permissions: 0777
template: I love {{ if .ChoiceOfFruit }}`

	cfile := CustomFile{}
	err := yaml.Unmarshal([]byte(badTemplate), &cfile)
	assert.NoError(t, err)
	output, err := cfile.RenderContent(TestContextAnimal{})
	assert.Error(t, err)
	assert.Equal(t, "", output)
}

func TestDualCustomFileContent(t *testing.T) {
	// customFile contains both content AND template
	cf := `path: /tmp/testfile
permissions: 0777
content: I love bananas
template: I love {{ .ChoiceOfFruit }}
`

	cfile := CustomFile{}
	err := yaml.Unmarshal([]byte(cf), &cfile)
	assert.NoError(t, err)
	output, err := cfile.RenderContent(TestContextFruit{ChoiceOfFruit: "apples"})
	assert.NoError(t, err)
	assert.Equal(t, "I love apples", output)
}
