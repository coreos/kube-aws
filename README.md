# Kubernetes on AWS (kube-aws)

[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-incubator/kube-aws)](https://goreportcard.com/report/github.com/kubernetes-incubator/kube-aws)
[![Build Status](https://travis-ci.org/kubernetes-incubator/kube-aws.svg?branch=master)](https://travis-ci.org/kubernetes-incubator/kube-aws)
[![License](https://img.shields.io/badge/license-Apache%20License%202.0-blue.svg)](LICENSE)


**Note**: The `master` branch may be in an *unstable or even broken state* during development. Please use [releases](https://github.com/kubernetes-incubator/kube-aws/releases) instead of the `master` branch in order to get stable binaries.

`kube-aws` is a command-line tool to create/update/destroy Kubernetes clusters on AWS.

## Features

* Create, update and destroy Kubernetes clusters on AWS
* Highly available and scalable Kubernetes clusters backed by multi-AZ deployment and Node Pools
* Deployment to an existing VPC
* Powered by various AWS services including CloudFormation, KMS, Auto Scaling, Spot Fleet, EC2, ELB, S3, etc.

## Getting Started / Manual

[View the latest manual for `kube-aws`](https://kubernetes-incubator.github.io/kube-aws/)

Check out our [getting started tutorial](https://kubernetes-incubator.github.io/kube-aws/getting-started/) 
to launch your first Kubernetes cluster on AWS.

## Examples

Generate `cluster.yaml`:

```
$ mkdir my-cluster
$ cd my-cluster
$ kube-aws init --cluster-name=my-cluster \
--external-dns-name=<my-cluster-endpoint> \
--region=us-west-1 \
--availability-zone=us-west-1c \
--key-name=<key-pair-name> \
--kms-key-arn="arn:aws:kms:us-west-1:xxxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
```

Here `us-west-1c` is used for parameter `--availability-zone`, but supported availability zone varies among AWS accounts.
Please check if `us-west-1c` is supported by `aws ec2 --region us-west-1 describe-availability-zones`, if not switch to other supported availability zone. (e.g., `us-west-1a`, or `us-west-1b`)

Generate assets:

```
$ kube-aws render credentials --generate-ca
$ kube-aws render stack
```

Validate configuration:

```
$ kube-aws validate --s3-uri s3://<your-bucket>/<optional-prefix>
```

Launch:

```
$ kube-aws up --s3-uri s3://<your-bucket>/<optional-prefix>

# Or export your cloudformation stack and dependent assets into the `exported/` directory
$ kube-aws up --s3-uri s3://<your-bucket>/<optional-prefix> --export

# Access the cluster
$ KUBECONFIG=kubeconfig kubectl get nodes --show-labels
```

Update:

```
$ $EDITOR cluster.yaml
# Update all the cfn stacks including the one for control-plane and the ones for worker node pools
$ kube-aws update --s3-uri s3://<your-bucket>/<optional-prefix>
```

Destroy:

```
# Destroy all the cfn stacks including the one for control-plane and the ones for worker node pools. Use `--force` for skip confirmation. 
$ kube-aws destroy
```

## Other Resources

Extra or advanced topics in for kube-aws:

* [Known Limitations](/docs/troubleshooting/known-limitations.md)
* [Roadmap](/ROADMAP.md)

The following links can be useful for development:

- [AWS CloudFormation resource types](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-template-resource-type-ref.html)

Please feel free to reach out to the kube-aws community on: [#kube-aws in the kubernetes slack](https://kubernetes.slack.com/messages/C5GP8LPEC/)

## Kubernetes Incubator

This is a [Kubernetes Incubator project](https://github.com/kubernetes/community/blob/master/incubator.md). The project was established 2017-03-15. The incubator team for the project is:

- Sponsor: Tim Hockin (@thockin)
- Champion: Mike Danese (@mikedanese)
- SIG: sig-aws

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

## Contributing

Submit a PR to this repository, following the [contributors guide](CONTRIBUTING.md).

Details of how to develop kube-aws are in our [Developer Guide](https://kubernetes-incubator.github.io/kube-aws/guides/developer-guide.html).
