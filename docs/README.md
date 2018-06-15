# Introduction {#introduction}

kube-aws is a command-line tool to create/update/destroy Kubernetes clusters on AWS. The [full manual can be found here](https://kubernetes-incubator.github.io/kube-aws/).

To start using kube-aws, try the [Getting Started Guide](getting-started/README.md).

Once you are familiar with the basic setup, the sections [Add-Ons](add-ons/README.md) and some [Advanced Topics](advanced-topics/README.md) cover additional setup, use cases and configuration.

[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-incubator/kube-aws)](https://goreportcard.com/report/github.com/kubernetes-incubator/kube-aws) [![Build Status](https://travis-ci.org/kubernetes-incubator/kube-aws.svg?branch=master)](https://travis-ci.org/kubernetes-incubator/kube-aws) [![License](https://img.shields.io/badge/license-Apache%20License%202.0-blue.svg)](LICENSE)


# Features {#features}

* Create, update and destroy Kubernetes clusters on AWS
* Highly available and scalable Kubernetes clusters backed by multi-AZ deployment and Node Pools
* Deployment to an existing VPC
* Powered by various AWS services including CloudFormation, KMS, Auto Scaling, Spot Fleet, EC2, ELB, S3, etc

# Kubernetes Incubator

kube-aws is a [Kubernetes Incubator project](https://github.com/kubernetes/community/blob/master/incubator.md). The project was established 2017-03-15. The incubator team for the project is:

- Sponsor: Tim Hockin (@thockin)
- Champion: Mike Danese (@mikedanese)
- SIG: sig-aws

# Announcements

Older releases of kube-aws had been signed by the CoreOS key and were verifiable with the [CoreOS Application Signing Public Key](https://coreos.com/security/app-signing-key/). This was when kube-aws was maintained primarily by CoreOS. However, the signing process has been postponed since v0.9.3 since the comm. Please read the [issue \#288](https://github.com/kubernetes-incubator/kube-aws/issues/288) for more information.

# Documentation Updates

Please [contact us](getting-in-touch.md) if you wish to see a topic added to the manual.
