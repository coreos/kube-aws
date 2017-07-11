package api

// Command is not implemented yet
type Command interface {
	Initialize(input InitializeInput) InitializeOutput
	// Render is used to generates assets required by the plugin in an idempotent manner
	Render(input RenderInput) RenderOutput
	// Validate is used to validate values configured for the plugin
	Validate(input ValidateInput) ValidateOutput
	PostDeployValidate()
}

type InitializeInput struct {
	Values Values `yaml:"values,"`
}

type InitializeOutput struct {
	ErrorMessages string `yaml:"errorMessages,"`
}

type RenderInput struct {
	Values Values `yaml:"values,"`
}

type RenderOutput struct {
	ErrorMessages string `yaml:"errorMessages,"`
}

type ValidateInput struct {
	Values Values `yaml:"values,"`
}

type ValidateOutput struct {
	ErrorMessages string `yaml:"errorMessages,"`
}
