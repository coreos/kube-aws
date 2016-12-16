# End-to-end testing for kube-aws

This directory contains a set of tools to run end-to-end testing for kube-aws.
It is composed of:

* Cluster creation using `kube-aws`
* [Kubernetes Conformance Tests](https://github.com/kubernetes/kubernetes/blob/master/docs/devel/e2e-tests.md#conformance-tests)

To run e2e tests, you should have set all he required env vars.
For convenience, creating `.envrc` used by `direnv` like as follows would be good.

```
export KUBE_AWS_KEY_NAME=...
export KUBE_AWS_KMS_KEY_ARN=arn:aws:kms:us-west-1:<account id>:key/<id>
export KUBE_AWS_DOMAIN=example.com
export KUBE_AWS_REGION=us-west-1
export KUBE_AWS_AVAILABILITY_ZONE=us-west-1b
export KUBE_AWS_HOSTED_ZONE_ID=...
export KUBE_AWS_AZ_1=...

export DOCKER_REPO=quay.io/mumoshu/
export SSH_PRIVATE_KEY=path/to/private/key/matching/key/name
```

Finally, run e2e tests like:

```
$ KUBE_AWS_CLUSTER_NAME=kubeawstest ./run all
```

Enable all the features including experimental ones:

```
KUBE_AWS_DEPLOY_TO_EXISTING_VPC=1 \
  KUBE_AWS_SPOT_FLEET_ENABLED=1 \
  KUBE_AWS_CLUSTER_AUTOSCALER_ENABLED=1 \
  KUBE_AWS_NODE_POOL_INDEX=1 \
  KUBE_AWS_WAIT_SIGNAL_ENABLED=1 \
  KUBE_AWS_AWS_NODE_LABELS_ENABLED=1 \
  KUBE_AWS_NODE_LABELS_ENABLED=1 \
  KUBE_AWS_AWS_ENV_ENABLED=1 \
  KUBE_AWS_USE_CALICO=true \
  KUBE_AWS_CLUSTER_NAME=kubeawstest1 sh -c './run all'
```
