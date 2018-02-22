# AWS Credentials

Configure your local workstation with AWS credentials using one of the following methods.

## Method 1: `configure` command

Provide the values of your AWS access and secret keys, and optionally default region and output format:

```bash
$ aws configure
AWS Access Key ID [None]: AKID1234567890
AWS Secret Access Key [None]: MY-SECRET-KEY
Default region name [None]: us-west-1
Default output format [None]: text
```

## Method 2: Environment Variables

Provide AWS credentials to kube-aws by exporting the following environment variables:

```bash
export AWS_ACCESS_KEY_ID=AKID1234567890
export AWS_SECRET_ACCESS_KEY=MY-SECRET-KEY
```

Alternatively, you can provide a AWS profile to kube-aws via the `AWS_PROFILE` environment variable such as:

```
AWS_PROFILE=my-profile-name kube-aws init ...
```

### Multi-Factor Authentication (MFA)

If you are using MFA, you need to export the Session Token as well:

```bash
export AWS_SESSION_TOKEN=MY-SESSION-TOKEN
```