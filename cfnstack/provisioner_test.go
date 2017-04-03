package cfnstack

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/kubernetes-incubator/kube-aws/model"
	"testing"
)

type dummyS3ObjectPutterService struct {
	ExpectedBucket        string
	ExpectedKey           string
	ExpectedBody          string
	ExpectedContentType   string
	ExpectedContentLength int64
}

func (s3Svc dummyS3ObjectPutterService) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {

	if s3Svc.ExpectedContentLength != *input.ContentLength {
		return nil, fmt.Errorf(
			"expected content length does not match supplied content length\nexpected=%v, supplied=%v",
			s3Svc.ExpectedContentLength,
			input.ContentLength,
		)
	}

	if s3Svc.ExpectedBucket != *input.Bucket {
		return nil, fmt.Errorf(
			"expected bucket does not match supplied bucket\nexpected=%v, supplied=%v",
			s3Svc.ExpectedBucket,
			input.Bucket,
		)
	}

	if s3Svc.ExpectedKey != *input.Key {
		return nil, fmt.Errorf(
			"expected key does not match supplied key\nexpected=%v, supplied=%v",
			s3Svc.ExpectedKey,
			*input.Key,
		)
	}

	if s3Svc.ExpectedContentType != *input.ContentType {
		return nil, fmt.Errorf(
			"expected content type does not match supplied content type\nexpected=%v, supplied=%v",
			s3Svc.ExpectedContentType,
			input.ContentType,
		)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(input.Body)
	suppliedBody := buf.String()

	if s3Svc.ExpectedBody != suppliedBody {
		return nil, fmt.Errorf(
			"expected body does not match supplied body\nexpected=%v, supplied=%v",
			s3Svc.ExpectedBody,
			suppliedBody,
		)
	}

	resp := &s3.PutObjectOutput{}

	return resp, nil
}

func TestUploadTemplateWithDirectory(t *testing.T) {
	body := "{}"
	s3URI := "s3://mybucket/mykey"
	s3Svc := dummyS3ObjectPutterService{
		ExpectedBucket:        "mybucket",
		ExpectedKey:           "mykey/test-cluster-name/stack.json",
		ExpectedContentLength: 2,
		ExpectedContentType:   "application/json",
		ExpectedBody:          body,
	}

	provisioner := NewProvisioner("test-cluster-name", map[string]string{}, s3URI, model.RegionForName("us-east-1"), body, nil)

	suppliedURL, err := provisioner.uploadFile(s3Svc, body, "stack.json")

	if err != nil {
		t.Errorf("error uploading template: %v", err)
	}

	expectedURL := "https://s3.amazonaws.com/mybucket/mykey/test-cluster-name/stack.json"
	if suppliedURL != expectedURL {
		t.Errorf("supplied template url doesn't match expected one: expected=%s, supplied=%s", expectedURL, suppliedURL)
	}
}

func TestUploadTemplateWithDirectoryOnChina(t *testing.T) {
	body := "{}"
	s3URI := "s3://mybucket/mykey"
	s3Svc := dummyS3ObjectPutterService{
		ExpectedBucket:        "mybucket",
		ExpectedKey:           "mykey/test-cluster-name/stack.json",
		ExpectedContentLength: 2,
		ExpectedContentType:   "application/json",
		ExpectedBody:          body,
	}

	provisioner := NewProvisioner("test-cluster-name", map[string]string{}, s3URI, model.RegionForName("cn-north-1"), body, nil)

	suppliedURL, err := provisioner.uploadFile(s3Svc, body, "stack.json")

	if err != nil {
		t.Errorf("error uploading template: %v", err)
	}

	expectedURL := "https://s3.cn-north-1.amazonaws.com.cn/mybucket/mykey/test-cluster-name/stack.json"
	if suppliedURL != expectedURL {
		t.Errorf("supplied template url doesn't match expected one: expected=%s, supplied=%s", expectedURL, suppliedURL)
	}
}

func TestUploadTemplateWithoutDirectory(t *testing.T) {
	body := "{}"
	s3URI := "s3://mybucket"
	s3Svc := dummyS3ObjectPutterService{
		ExpectedBucket:        "mybucket",
		ExpectedKey:           "test-cluster-name/stack.json",
		ExpectedContentLength: 2,
		ExpectedContentType:   "application/json",
		ExpectedBody:          body,
	}

	provisioner := NewProvisioner("test-cluster-name", map[string]string{}, s3URI, model.RegionForName("us-east-1"), body, nil)

	suppliedURL, err := provisioner.uploadFile(s3Svc, body, "stack.json")

	if err != nil {
		t.Errorf("error uploading template: %v", err)
	}

	expectedURL := "https://s3.amazonaws.com/mybucket/test-cluster-name/stack.json"
	if suppliedURL != expectedURL {
		t.Errorf("supplied template url doesn't match expected one: expected=%s, supplied=%s", expectedURL, suppliedURL)
	}
}

func TestUploadTemplateWithoutDirectoryOnChina(t *testing.T) {
	body := "{}"
	s3URI := "s3://mybucket"
	s3Svc := dummyS3ObjectPutterService{
		ExpectedBucket:        "mybucket",
		ExpectedKey:           "test-cluster-name/stack.json",
		ExpectedContentLength: 2,
		ExpectedContentType:   "application/json",
		ExpectedBody:          body,
	}

	provisioner := NewProvisioner("test-cluster-name", map[string]string{}, s3URI, model.RegionForName("cn-north-1"), body, nil)

	suppliedURL, err := provisioner.uploadFile(s3Svc, body, "stack.json")

	if err != nil {
		t.Errorf("error uploading template: %v", err)
	}

	expectedURL := "https://s3.cn-north-1.amazonaws.com.cn/mybucket/test-cluster-name/stack.json"
	if suppliedURL != expectedURL {
		t.Errorf("supplied template url doesn't match expected one: expected=%s, supplied=%s", expectedURL, suppliedURL)
	}
}
