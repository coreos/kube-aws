package provisioner

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"os"
	"strings"
)

type S3ObjectPutter interface {
	PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error)
}

func (t TransferredFile) S3URI() string {
	return fmt.Sprintf("%s/%s", t.s3DirURI, t.BaseName())
}

func (t TransferredFile) ReceiveCommand() string {
	return fmt.Sprintf(`aws s3 cp %s %s`, t.S3URI(), t.Path)
}

func (t TransferredFile) Send(client S3ObjectPutter) error {
	opened, err := os.Open(t.Source.Path)
	if err != nil {
		return err
	}
	defer opened.Close()

	splits1 := strings.Split(t.s3DirURI, "s3://")
	s3prefix := splits1[1]

	splits := strings.SplitN(s3prefix, "/", 2)
	bucket := splits[0]
	prefix := splits[1]

	fmt.Fprintf(os.Stderr, "putting %s onto %s with prefix %s\n", t.BaseName(), bucket, prefix)
	_, err = client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(prefix + "/" + t.BaseName()),
		Body:   opened,
	})
	if err != nil {
		return err
	}
	return nil
}
