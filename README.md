# Kubernetes on AWS (kube-aws)

This is the source of the `kube-aws` tool and the installation artifacts used by the official Kubernetes on AWS documentation.

View the full instructions at [GitHub](/Documentation/kubernetes-on-aws.md) or at [the CoreOS documentation website](https://coreos.com/kubernetes/docs/latest/kubernetes-on-aws.html).

---

CoreOS is interested in learning more about how people are launching clusters on AWS - fill out [this survey](https://docs.google.com/forms/d/e/1FAIpQLSf7rktZ_1QABIPkzYqxlMuGSE4W82ZFUYmyYk0ER7tqHMPBRg/viewform) to help us out.

This survey is meant for those who are currently running at least some workloads (dev, staging, or prod) on Kubernetes on AWS. Once we have received enough responses, we will share our learnings, anonymized and in aggregate, with the general Kubernetes community.

---

`kube-aws` is a command-line tool to create/update/destroy Kubernetes clusters on AWS.

## Features

* Create, update and destroy Kubernetes clusters on AWS
* Highly available and scalable Kubernetes clusters backed by multi-AZ deployment and Node Pools
* Deployment to an existing VPC
* Powered by various AWS services including CloudFormation, KMS, Auto Scaling, Spot Fleet, EC2, ELB, S3, etc.

## Design Decisions

* CoreOS as the host OS
* Allow exporting CloudFormation stack templates for further customizations

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

# Or export your cloudformation stack
$ kube-aws up --export

# Access the cluster
$ KUBECONFIG=kubeconfig kubectl get nodes --show-labels
```

Update:

```
$ $EDITOR cluster.yaml
$ kube-aws update --s3-uri s3://<your-bucket>/<optional-prefix>
```

Node Pool:

```
$ kube-aws node-pools init --node-pool-name my-pool
$ kube-aws node-pools render stack --node-pool-name my-pool
$ kube-aws node-pools validate --node-pool-name my-pool \
  --s3-uri s3://<your-bucket>/<optional-prefix>
$ kube-aws node-pools up --node-pool-name my-pool \
  --s3-uri s3://<your-bucket>/<optional-prefix>
$ $EDITOR node-pools/my-pool/cluster.yaml
$ kube-aws node-pools update --node-pool-name my-pool \
  --s3-uri s3://<your-bucket>/<optional-prefix>
```

Destroy:

```
$ kube-aws node-pools destroy --node-pool-name my-pool
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

The various templates are located in the `config/templates/` and the `nodepool/config/templates/` directory of the source repo. `go generate` is used to pack these templates into the source code. In order for changes to templates to be reflected in the source code:

```sh
make build
```

## Other Resources

Extra or advanced topics in for kube-aws:

* [Known Limitations](/Documentation/kubernetes-on-aws-limitations.md)
* [Roadmap](/ROADMAP.md)

The following links can be useful for development:

- [AWS CloudFormation resource types](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-template-resource-type-ref.html)

## Contributing

Submit a PR to this repository, following the [contributors guide](CONTRIBUTING.md).
The documentation is published from [this source](Documentation/kubernetes-on-aws.md).
