package gzipcompressor

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
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
