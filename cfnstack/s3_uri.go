package cfnstack

import (
	"fmt"
	"regexp"
	"strings"
)

type S3URI interface {
	Bucket() string
	KeyComponents() []string
	BucketAndKey() string
	String() string
}

type s3URIImpl struct {
	bucket    string
	directory string
}

func (u s3URIImpl) Bucket() string {
	return u.bucket
}

func (u s3URIImpl) KeyComponents() []string {
	if u.directory != "" {
		return []string{
			u.directory,
		}
	}
	return []string{}
}

func (u s3URIImpl) BucketAndKey() string {
	components := []string{}
	path := u.KeyComponents()
	components = append(components, u.bucket)
	components = append(components, path...)
	return strings.Join(components, "/")
}

func (u s3URIImpl) String() string {
	return fmt.Sprintf("s3://%s", u.BucketAndKey())
}

func S3URIFromString(s3URI string) (S3URI, error) {
	re := regexp.MustCompile("s3://(?P<bucket>[^/]+)/(?P<directory>.+[^/])/*$")
	matches := re.FindStringSubmatch(s3URI)
	var bucket string
	var directory string
	if len(matches) == 3 {
		bucket = matches[1]
		directory = matches[2]
	} else {
		re := regexp.MustCompile("s3://(?P<bucket>[^/]+)/*$")
		matches := re.FindStringSubmatch(s3URI)

		if len(matches) == 2 {
			bucket = matches[1]
		} else {
			return nil, fmt.Errorf("failed to parse s3 uri(=%s): The valid uri pattern for it is s3://mybucket/mydir or s3://mybucket", s3URI)
		}
	}
	return s3URIImpl{
		bucket:    bucket,
		directory: directory,
	}, nil
}
