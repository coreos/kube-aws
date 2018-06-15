# Configure

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

## Test Credentials

Test that your credentials work by describing any instances you may already have running on your account:

```sh
$ aws ec2 describe-instances
```

**Did you download kube-aws?**
**Did your credentials work?** We will use the AWS CLI in the next step.

[Yes, ready to configure my cluster options][getting-started-step-2]

[No, I need more info on the AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html)

[getting-started-step-1]: step-1-configure.md
[getting-started-step-2]: step-2-render.md
[getting-started-step-3]: step-3-launch.md
[getting-started-step-4]: step-4-update.md
[getting-started-step-5]: step-5-add-node-pool.md
[getting-started-step-6]: step-6-configure-add-ons.md
[getting-started-step-7]: step-7-destroy.md