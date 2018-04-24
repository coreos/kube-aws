package gzipcompressor

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/ioutil"
)

func BytesToBytes(d []byte) ([]byte, error) {
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

func CompressData(d []byte) (string, error) {
	var buff bytes.Buffer
	gzw := gzip.NewWriter(&buff)
	if _, err := gzw.Write(d); err != nil {
		return "", err
	}
	if err := gzw.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buff.Bytes()), nil
}

func CompressString(str string) (string, error) {
	bytes := []byte(str)
	return CompressData(bytes)
}

func DecompressString(source string) (string, error) {
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
