# Developer Guide

If you would like to contribute towards the goals of kube-aws, the easiest way to get started is to submit a pull request to the [kube-aws repository](https://github.com/kubernetes-incubator/kube-aws/), following the [contributors guide](https://github.com/kubernetes-incubator/kube-aws/blob/master/CONTRIBUTING.md). If you need some help getting started with a contribution let us know and we can point you in the right direction.

# Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](https://github.com/kubernetes-incubator/kube-aws/blob/master/code-of-conduct.md).

# Build

Clone the [kube-aws repository](https://github.com/kubernetes-incubator/kube-aws) to the appropriate path under the GOPATH.

```bash
$ export GOPATH=$HOME/go
$ git clone git@github.com:kubernetes-incubator/kube-aws.git $GOPATH/src/github.com/kubernetes-incubator/kube-aws
```

Run `make build` to compile kube-aws locally.

This depends on having:

* golang &gt;= 1.8

The compiled binary will be available at `bin/kube-aws`.

# Run Unit Tests

```bash
make test
```

# Run e2e Tests

To run the e2e tests, you will need at least these environment variables setup with any missing values filled in with values from your AWS account:

```bash
export CLUSTER_NAME=e2etest
export KUBE_AWS_KEY_NAME=
export SSH_PRIVATE_KEY=
export KUBE_AWS_SSH_KEY=${SSH_PRIVATE_KEY}
export KUBE_AWS_KMS_KEY_ARN=
export KUBE_AWS_DOMAIN=
export KUBE_AWS_REGION=
export KUBE_AWS_AVAILABILITY_ZONE=
export KUBE_AWS_AZ_1=${KUBE_AWS_AVAILABILITY_ZONE}
export KUBE_AWS_HOSTED_ZONE_ID=
export KUBE_AWS_S3_DIR_URI=
export DOCKER_REPO=quay.io/mumoshu/
export FOCUS=.*
```

It's recommended to keep the `DOCKER_REPO` value as above to get started quickly.

Then run the e2e tests using:

```bash
rm -r assets/${CLUSTER_NAME}
KUBE_AWS_CLUSTER_NAME=${CLUSTER_NAME} ./run all
```

The `FOCUS` environment variable can be used if you wish to target a particular test or set of e2e tests. For example to target the e2e tests for the rescheduler:

```bash
export FOCUS=.*Rescheduler.*
```

# Reformat Code

```bash
make format
```

# Modifying Templates

The various templates are located in the `core/controlplane/config/templates/` and the `core/nodepool/config/templates/` directory of the source repo. `go generate` is used to pack these templates into the source code. In order for changes to templates to be reflected in the source code:

```bash
make build
```

# Documentation

Documentation source lives inside the `docs` directory of the `master` branch. Generally we aim to add documentation in the same PR as any features/updates so it can be reviewed together. To update the documentation, create/update markdown files inside the `docs` directory and use the following command to generate the full documentation site locally:

```
make serve-docs
```

__NOTE:__ Since the documentation is generated via the [gitbook CLI](https://www.npmjs.com/package/gitbook-cli) NPM module you will need `node` and `npm` installed to generate and serve the site locally.

It's worth looking through the existing sections to see where you changes would fit best. A page will not appear in the index listing unless it is added to `SUMMARY.md`.
