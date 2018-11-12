package jsontemplate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/filereader/texttemplate"
)

func GetBytes(filename string, data interface{}, prettyPrint bool) ([]byte, error) {
	rendered, err := texttemplate.GetString(filename, data)
	if err != nil {
		return nil, err
	}

	//Use unmarshal function to do syntax validation
	renderedBytes := []byte(rendered)
	var jsonHolder map[string]interface{}
	if err := json.Unmarshal(renderedBytes, &jsonHolder); err != nil {
		syntaxError, ok := err.(*json.SyntaxError)
		if ok {
			contextString := getContextString(renderedBytes, int(syntaxError.Offset), 10)
			return nil, fmt.Errorf("%v:\njson syntax error (offset=%d), in this region:\n-------\n%s\n-------\n", err, syntaxError.Offset, contextString)
		}
		return nil, err
	}

	// minify or pretty print JSON
	var buff bytes.Buffer
	err = nil
	if prettyPrint {
		err = json.Indent(&buff, renderedBytes, "", "  ")
	} else {
		err = json.Compact(&buff, renderedBytes)
	}
	if err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}

func GetString(filename string, data interface{}, prettyPrint bool) (string, error) {
	bytes, err := GetBytes(filename, data, prettyPrint)

	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func getContextString(buf []byte, offset, lineCount int) string {
	// Prevent index out of range errors when we meet errors at the very end of json
	bufsize := len(buf)
	if offset >= bufsize {
		if bufsize > 0 {
			offset = bufsize - 1
		} else {
			offset = 0
		}
	}

	linesSeen := 0
	var leftLimit int
	for leftLimit = offset; leftLimit > 0 && linesSeen <= lineCount; leftLimit-- {
		if buf[leftLimit] == '\n' {
			linesSeen++
		}
	}

	linesSeen = 0
	var rightLimit int
	for rightLimit = offset + 1; rightLimit < len(buf) && linesSeen <= lineCount; rightLimit++ {
		if buf[rightLimit] == '\n' {
			linesSeen++
		}
	}

	return string(buf[leftLimit:rightLimit])
}
