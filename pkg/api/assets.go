package api

import (
	"fmt"
	"github.com/kubernetes-incubator/kube-aws/fingerprint"
	"strings"
)

type AssetID struct {
	StackName string
	Filename  string
}

type Asset struct {
	AssetLocation
	Content string
}

type AssetLocation struct {
	ID     AssetID
	Key    string
	Bucket string
	Path   string
	Region Region
}

func NewAssetID(stack string, file string) AssetID {
	return AssetID{
		StackName: stack,
		Filename:  file,
	}
}

func (l AssetLocation) URL() (string, error) {
	if (l == AssetLocation{}) {
		return "", fmt.Errorf("[bug] Empty asset location can't have URL")
	}
	return fmt.Sprintf("%s/%s/%s", l.Region.S3Endpoint(), l.Bucket, l.Key), nil
}

func (l AssetLocation) S3URL() (string, error) {
	if (l == AssetLocation{}) {
		return "", fmt.Errorf("[bug] Empty asset location can't have S3 URL")
	}
	return fmt.Sprintf("s3://%s/%s", l.Bucket, l.Key), nil
}

func (l Asset) S3Prefix() (string, error) {
	if (l.AssetLocation == AssetLocation{}) {
		return "", fmt.Errorf("[bug] Empty asset location can't have URL")
	}
	prefix := strings.TrimSuffix(l.Key, fmt.Sprintf("-%s", fingerprint.SHA256(l.Content)))
	return fmt.Sprintf("%s/%s", l.Bucket, prefix), nil
}
