package gzipcompressor

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
)

func BytesToGzippedBytes(d []byte) ([]byte, error) {
	var buff bytes.Buffer
	gzw := gzip.NewWriter(&buff)
	if _, err := gzw.Write(d); err != nil {
		return []byte{}, err
	}
	if err := gzw.Close(); err != nil {
		return []byte{}, err
	}
	return buff.Bytes(), nil
}

func BytesToGzippedBase64String(d []byte) (string, error) {
	bytes, err := BytesToGzippedBytes(d)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

func StringToGzippedBase64String(str string) (string, error) {
	bytes := []byte(str)
	return BytesToGzippedBase64String(bytes)
}

func GzippedBase64StringToString(source string) (string, error) {
	b64Decoded, err := base64.StdEncoding.DecodeString(source)
	if err != nil {
		return "", fmt.Errorf("could not base 64 decode the data")
	}

	bread := bytes.NewReader(b64Decoded)
	gzr, _ := gzip.NewReader(bread)
	out, err := ioutil.ReadAll(gzr)
	if err != nil {
		return "", fmt.Errorf("Could not uncompress data")
	}
	return string(out), nil
}
