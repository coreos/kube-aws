# CLI Reference

[AWS credentials](aws-credentials.md) need to be configured for commands that run against your AWS account.

# `init`

Initialize the base configuration for a cluster ready for customization prior to deployment.

| Flag | Description | Default |
| -- | -- | -- |
| `ami-id` | The AMI ID of CoreOS Container Linux to deploy | The latest AMI for the Container Linux release channel specified in `cluster.yaml` |
| `availability-zone` | The AWS availability-zone to deploy to. Note, this can be changed to multi AZ in `cluster.yaml` | none |
| `cluster-name` | The name of this cluster. This will be the name of the cloudformation stack | none |
| `external-dns-name` | The hostname that will route to the api server | none |
| `hosted-zone-id` | The hosted zone in which a Route53 record set for a k8s API endpoint is created | none |
| `key-name` | The AWS key-pair for SSH access to nodes | none |
| `kms-key-arn` | The ARN of the AWS KMS key for encrypting TLS assets |
| `no-record-set` | Instruct kube-aws to not manage Route53 record sets for your K8S API | `false` |
| `region` | The AWS region to deploy to | none |

### `init` example

```bash
$ kube-aws init \
  --cluster-name=my-cluster \
  --region=us-west-1 \
  --availability-zone=us-west-1c \
  --hosted-zone-id=xxxxxxxxxxxxxx \
  --external-dns-name=my-cluster-endpoint.mydomain.com \
  --key-name=key-pair-name \
  --kms-key-arn="arn:aws:kms:us-west-1:xxxxxxxxxx:key/xxxxxxxxxxxxxxxxxxx"
```

# `render credentials`

Render TLS credentials required for cluster administration and communication between cluster nodes.

| Flag | Description | Default |
| -- | -- | -- |
| `ca-cert-path` | Path to pem-encoded CA x509 certificate | `./credentials/ca.pem` |
| `ca-key-path` | Path to pem-encoded CA RSA key | `./credentials/ca-key.pem` |
| `generate-ca` | If generating credentials, generate root CA key and cert. **NOT RECOMMENDED FOR PRODUCTION USE**, use `-ca-key-path` and `-ca-cert-path` options to provide your own certificate authority assets. | `false` |

### `render credentials` example

```bash
$ kube-aws render credentials \
  --ca-cert-path=/path/to/ca-cert.pem
  --ca-key-path=/path/to/ca-key.pem
```

# `render stack`

Render [CloudFormation](https://aws.amazon.com/cloudformation/) stack templates and [coreos-cloudinit](https://github.com/coreos/coreos-cloudinit) userdata ready for customization prior to deployment.

`render stack` has no CLI flags.

### `render stack` example

```bash
$ kube-aws render stack
```

# `validate`

Validate cluster assets prior to deployment.

| Flag | Description | Default |
| -- | -- | -- |
| `aws-debug` | Log debug information coming from the AWS SDK library | `false` |
| `s3-uri` | When your template is bigger than the [CloudFormation limit of 51,200 bytes](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cloudformation-limits.html), kube-aws needs to upload the template to S3 to perform the deploy. The S3 location expressed as `s3://<bucket>/path/to/dir`. Multiple clusters can use the same S3 bucket. | none |

### `validate` example

```bash
$ kube-aws validate \
  --s3-uri=s3://my-kube-aws-assets-bucket
```

# `up`

Deploy a new Kubernetes cluster.

| Flag | Description | Default |
| -- | -- | -- |
| `aws-debug` | Log debug information coming from the AWS SDK library | `false` |
| `export` | Do not create cluster, instead export the CloudFormation stack file | `false` |
| `pretty-print` | Pretty print the resulting CloudFormation | `false` |
| `s3-uri` | When your template is bigger than the [CloudFormation limit of 51,200 bytes](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cloudformation-limits.html), kube-aws needs to upload the template to S3 to perform the deploy. The S3 location expressed as `s3://<bucket>/path/to/dir`. Multiple clusters can use the same S3 bucket. | none |
| `skip-wait` | Do not wait for the cluster components be ready before the CLI exits | `false` |

### `up` example

```bash
$ kube-aws up \
  --s3-uri=s3://my-kube-aws-assets-bucket
```

# `update`

Update an existing Kubernetes cluster that was created by kube-aws.

| Flag | Description | Default |
| -- | -- | -- |
| `aws-debug` | Log debug information coming from the AWS SDK library | `false` |
| `pretty-print` | Pretty print the resulting CloudFormation | `false` |
| `s3-uri` | When your template is bigger than the [CloudFormation limit of 51,200 bytes](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cloudformation-limits.html), kube-aws needs to upload the template to S3 to perform the deploy. The S3 location expressed as `s3://<bucket>/path/to/dir`. Multiple clusters can use the same S3 bucket. | none |
| `skip-wait` | Do not wait for the cluster components be ready before the CLI exits | `false` |

### `update` example

```bash
$ kube-aws update \
  --s3-uri=s3://my-kube-aws-assets-bucket
```

# `destroy`

Destroy an existing Kubernetes cluster that was created by kube-aws.

| Flag | Description | Default |
| -- | -- | -- |
| `aws-debug` | Log debug information coming from the AWS SDK library | `false` |

### `destroy` example

```bash
$ kube-aws destory
```