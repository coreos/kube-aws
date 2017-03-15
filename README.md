# Kubernetes on AWS (kube-aws)

[![Go Report Card](https://goreportcard.com/badge/github.com/coreos/kube-aws)](https://goreportcard.com/report/github.com/coreos/kube-aws)
[![Build Status](https://travis-ci.org/coreos/kube-aws.svg?branch=master)](https://travis-ci.org/coreos/kube-aws)

**Note**: The `master` branch may be in an *unstable or even broken state* during development. Please use [releases](https://github.com/coreos/kube-aws/releases) instead of the `master` branch in order to get stable binaries.

`kube-aws` is a command-line tool to create/update/destroy Kubernetes clusters on AWS.

View the latest manual for the `kube-aws` tool on [GitHub](/Documentation/kubernetes-on-aws.md).

## Features

* Create, update and destroy Kubernetes clusters on AWS
* Highly available and scalable Kubernetes clusters backed by multi-AZ deployment and Node Pools
* Deployment to an existing VPC
* Powered by various AWS services including CloudFormation, KMS, Auto Scaling, Spot Fleet, EC2, ELB, S3, etc.

## Getting Started

Check out our getting started tutorial on launching your first Kubernetes cluster in AWS.

* [Pre-requisites](/Documentation/kubernetes-on-aws-prerequisites.md)
* [Step 1: Configure](/Documentation/kubernetes-on-aws.md)
  * Download the latest release of kube-aws
  * Define account and cluster settings
* [Step 2: Render](/Documentation/kubernetes-on-aws-render.md)
  * Compile a re-usable CloudFormation template for the cluster
  * Optionally adjust template configuration
  * Validate the rendered CloudFormation stack
* [Step 3: Launch](/Documentation/kubernetes-on-aws-launch.md)
  * Create the CloudFormation stack and start our EC2 machines
  * Set up CLI access to the new cluster
* [Step 4: Update](/Documentation/kube-aws-cluster-updates.md)
  * Update the CloudFormation stack
* [Step 5: Add Node Pool](/Documentation/kubernetes-on-aws-node-pool.md)
  * Create the additional pool of worker nodes
  * Adjust template configuration for each pool of worker nodes
  * Required to support [cluster-autoscaler](https://github.com/kubernetes/contrib/tree/master/cluster-autoscaler)
* [Step 6: Destroy](/Documentation/kubernetes-on-aws-destroy.md)
  * Destroy the cluster

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
# Destroy all the cfn stacks including the one for control-plane and the ones for worker node pools
$ kube-aws destroy
```

## Development

### Build

Clone this repository to the appropriate path under the GOPATH.

```
$ export GOPATH=$HOME/go
$ mkdir -p $GOPATH/src/github.com/coreos/
$ git clone git@github.com:coreos/kube-aws.git $GOPATH/src/github.com/coreos/kube-aws
```

Run `make build` to compile `kube-aws` locally.

This depends on having:
* golang >= 1.7

The compiled binary will be available at `bin/kube-aws`.

### Run Unit Tests

```sh
make test
```

### Reformat Code

```sh
make format
```

### Modifying Templates

The various templates are located in the `core/controlplane/config/templates/` and the `core/nodepool/config/templates/` directory of the source repo. `go generate` is used to pack these templates into the source code. In order for changes to templates to be reflected in the source code:

```sh
make build
```

## Other Resources

Extra or advanced topics in for kube-aws:

* [Known Limitations](/Documentation/kubernetes-on-aws-limitations.md)
* [Roadmap](/ROADMAP.md)

The following links can be useful for development:

- [AWS CloudFormation resource types](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-template-resource-type-ref.html)

## Kubernetes Incubator

This is a [Kubernetes Incubator project](https://github.com/kubernetes/community/blob/master/incubator.md). The project was established 2017-03-15. The incubator team for the project is:

- Sponsor: Tim Hockin (@thockin)
- Champion: Mike Danese (@mikedanese)
- SIG: sig-aws

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).

## Contributing

Submit a PR to this repository, following the [contributors guide](CONTRIBUTING.md).
The documentation is published from [this source](Documentation/kubernetes-on-aws.md).
