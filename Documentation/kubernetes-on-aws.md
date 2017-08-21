# Kubernetes on AWS

Deploy a fully-functional Kubernetes cluster using AWS CloudFormation.
Your cluster will be configured to use AWS features to enhance Kubernetes.
For example, Kubernetes may automatically provision an Elastic Load Balancer for each Kubernetes Service.
At CoreOS, we use the [kube-aws](https://github.com/kubernetes-incubator/kube-aws/releases) CLI tool to automate cluster deployment to AWS.

After completing this guide, a deployer will be able to interact with the Kubernetes API from their workstation using the `kubectl` CLI tool.

Each of the steps will cover:

* Step 1: Configure (this document)
  * Download the kube-aws CloudFormation generator
  * Define account and cluster settings
* [Step 2: Render][aws-step-2]
  * Compile a re-usable CloudFormation template for the cluster
  * Optionally adjust template configuration
  * Validate the rendered CloudFormation stack
* [Step 3: Launch][aws-step-3]
  * Create the CloudFormation stack and start our EC2 machines
  * Set up CLI access to the new cluster
* [Step 4: Update][aws-step-4]
  * Update the CloudFormation stack
* [Step 5: Add Node Pool][aws-step-5]
  * Create the additional pool of worker nodes
  * Adjust template configuration for each pool of worker nodes
  * Required to support [cluster-autoscaler](https://github.com/kubernetes/contrib/tree/master/cluster-autoscaler)
* [Step 6: Configure add-ons][aws-step-6]
  * Configure various Kubernetes add-ons
* [Step 7: Destroy][aws-step-7]
  * Destroy the cluster

Let's get started.

## Announcement to regular users of kube-aws

Older versions of kube-aws releases had been signed by the CoreOS key and were able to be verified with [CoreOS Application Signing Public Key](https://coreos.com/security/app-signing-key/).
However, the signing process is postponed since v0.9.3. Please read the issue [#288](https://github.com/kubernetes-incubator/kube-aws/issues/288) for more information.

## Download kube-aws

Go to the [releases](https://github.com/kubernetes-incubator/kube-aws/releases) and download the latest release tarball for your architecture.

Extract the binary:

```sh
tar zxvf kube-aws-${PLATFORM}.tar.gz
```

Add kube-aws to your path:

```sh
mv ${PLATFORM}/kube-aws /usr/local/bin
```

## Configure AWS credentials

Configure your local workstation with AWS credentials using one of the following methods:

### Method 1: Configure command

Provide the values of your AWS access and secret keys, and optionally default region and output format:

```sh
$ aws configure
AWS Access Key ID [None]: AKID1234567890
AWS Secret Access Key [None]: MY-SECRET-KEY
Default region name [None]: us-west-2
Default output format [None]: text
```

### Method 2: Config file

Write your credentials into the file `~/.aws/credentials` using the following template:

```
[default]
aws_access_key_id = AKID1234567890
aws_secret_access_key = MY-SECRET-KEY
```

### Method 3: Environment variables

Provide AWS credentials to kube-aws by exporting the following environment variables:

```sh
export AWS_ACCESS_KEY_ID=AKID1234567890
export AWS_SECRET_ACCESS_KEY=MY-SECRET-KEY
```

If you are using MFA, pls export the Session Token

```sh
export AWS_SESSION_TOKEN=MY-SESSION-TOKEN
```


## Test Credentials

Test that your credentials work by describing any instances you may already have running on your account:

```sh
$ aws ec2 describe-instances
```

<div class="co-m-docs-next-step">
  <p><strong>Did you download kube-aws?</strong></p>
  <p><strong>Did your credentials work?</strong> We will use the AWS CLI in the next step.</p>
  <a href="kubernetes-on-aws-render.md" class="btn btn-primary btn-icon-right"  data-category="Docs Next" data-event="Kubernetes: AWS Render">Yes, ready to configure my cluster options</a>
  <a href="https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html" class="btn btn-default btn-icon-right"  data-category="Docs Next" data-event="Configure AWS CLI">No, I need more info on the AWS CLI</a>
</div>

[aws-step-1]: kubernetes-on-aws.md
[aws-step-2]: kubernetes-on-aws-render.md
[aws-step-3]: kubernetes-on-aws-launch.md
[aws-step-4]: kube-aws-cluster-updates.md
[aws-step-5]: kubernetes-on-aws-node-pool.md
[aws-step-6]: kubernetes-on-aws-add-ons.md
[aws-step-7]: kubernetes-on-aws-destroy.md
