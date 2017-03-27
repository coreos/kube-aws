package model

type Addons struct {
	Rescheduler Rescheduler `yaml:"rescheduler"`
	UnknownKeys `yaml:",inline"`
}

type Rescheduler struct {
	Enabled     bool `yaml:"enabled"`
	UnknownKeys `yaml:",inline"`
}
