package awsconn

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/kubernetes-incubator/kube-aws/model"
)

// NewSessionFromRegion creaes an AWS session from AWS region and a debug flag
func NewSessionFromRegion(region model.Region, debug bool) (*session.Session, error) {
	awsConfig := aws.NewConfig().
		WithRegion(region.String()).
		WithCredentialsChainVerboseErrors(true)

	if debug {
		awsConfig = awsConfig.WithLogLevel(aws.LogDebug)
	}

	session, err := newSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to establish aws session: %v", err)
	}
	return session, nil
}

// newSession returns an AWS session which supports source_profile and assume role with MFA
// See #1231 for more details
func newSession(config *aws.Config) (*session.Session, error) {
	return session.NewSessionWithOptions(session.Options{
		Config: *config,
		// This seems to be required for AWS_SDK_LOAD_CONFIG
		SharedConfigState: session.SharedConfigEnable,
		// This seems to be required by MFA
		AssumeRoleTokenProvider: stscreds.StdinTokenProvider,
	})
}
