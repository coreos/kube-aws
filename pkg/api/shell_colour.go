package api

import "fmt"

type ShellColour int

const (
	DefaultColour ShellColour = iota
	Black
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
	DarkGray
	LightRed
	LightGreen
	LightYellow
	LightBlue
	LightMagenta
	LightCyan
	LightWhite
)

var ShellColourCodeMap map[ShellColour]string = map[ShellColour]string{
	DefaultColour: `0m`,
	Black:         `0;30m`,
	Red:           `0;31m`,
	Green:         `0;32m`,
	Yellow:        `0;33m`,
	Blue:          `0;34m`,
	Magenta:       `0;35m`,
	Cyan:          `0;36m`,
	White:         `0;37m`,
	DarkGray:      `1;90m`,
	LightRed:      `1;31m`,
	LightGreen:    `1;32m`,
	LightYellow:   `1;33m`,
	LightBlue:     `1;34m`,
	LightMagenta:  `1;35m`,
	LightCyan:     `1;36m`,
	LightWhite:    `1;37m`,
}

func (colour ShellColour) PCOn() string {
	return fmt.Sprintf("\\[%s\\]", colour.On())
}

func (colour ShellColour) PCOff() string {
	return fmt.Sprintf("\\[%s\\]", colour.Off())
}

func (colour ShellColour) On() string {
	if colour.IsAShellColour() {
		return fmt.Sprintf("\\033[%s", ShellColourCodeMap[colour])
	} else {
		return fmt.Sprintf("\\033[%s", ShellColourCodeMap[DefaultColour])
	}
}

func (colour ShellColour) Off() string {
	return fmt.Sprintf("\\033[%s", ShellColourCodeMap[DefaultColour])
}
