# Node Pool

Node Pool allows you to bring up additional pools of worker nodes each with a separate configuration including:

* Instance Type
* Storage Type/Size/IOPS
* Instance Profile
* Additional, User-Provided Security Group(s)
* Spot Price
* AWS service to manage your EC2 instances: [Auto Scaling](http://docs.aws.amazon.com/autoscaling/latest/userguide/WhatIsAutoScaling.html) or [Spot Fleet](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-fleet.html)
* [Node labels](http://kubernetes.io/docs/user-guide/node-selection/)
* [Taints](https://github.com/kubernetes/kubernetes/issues/17190)

## Deploying a Multi-AZ cluster with cluster-autoscaler support with Node Pools

Edit the `cluster.yaml` file to decrease `workerCount`, which is meant to be number of worker nodes in the "main" cluster, down to zero:

```yaml
workerCount: 0
subnets:
  - availabilityZone: us-west-1a
    instanceCIDR: "10.0.0.0/24"
```

Update the main cluster to catch up changes made in `cluster.yaml`:

```
$ kube-aws update \
  --s3-uri s3://<my-bucket>/<optional-prefix>
```

Create two node pools, each with a different subnet and an availability zone:

```
$ kube-aws node-pools init --node-pool-name first-pool-in-1a \
  --availability-zone us-west-1a \
  --key-name ${KUBE_AWS_KEY_NAME} \
  --kms-key-arn ${KUBE_AWS_KMS_KEY_ARN}

$ kube-aws node-pools init --node-pool-name second-pool-in-1b \
  --availability-zone us-west-1a \
  --key-name ${KUBE_AWS_KEY_NAME} \
  --kms-key-arn ${KUBE_AWS_KMS_KEY_ARN}
```

Edit the `cluster.yaml` for the first zone:

```
$ $EDITOR node-pools/first-pool-in-1a/cluster.yaml
```

```yaml
workerCount: 1
subnets:
  - availabilityZone: us-west-1a
    instanceCIDR: "10.0.1.0/24"
```

Edit the `cluster.yaml` for the second zone:

```
$ $EDITOR node-pools/second-pool-in-1b/cluster.yaml
```

```yaml
workerCount: 1
subnets:
  - availabilityZone: us-west-1b
    instanceCIDR: "10.0.2.0/24"
```

Launch the node pools:

```
$ kube-aws node-pools up --node-pool-name first-pool-in-1a \
  --s3-uri s3://<my-bucket>/<optional-prefix>

$ kube-aws node-pools up --node-pool-name second-pool-in-1b \
  --s3-uri s3://<my-bucket>/<optional-prefix>
```

Deployment of cluster-autoscaler is currently out of scope of this documentation.
Please read [cluster-autoscaler's documentation](https://github.com/kubernetes/contrib/blob/master/cluster-autoscaler/cloudprovider/aws/README.md) for instructions on it.

## Customizing min/max size of the auto scaling group

If you've chosen to power your worker nodes in a node pool with an auto scaling group, you can customize `MinSize`, `MaxSize`, `MinInstancesInService` in `cluster.yaml`:

Please read [the AWS documentation](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-as-group.html#aws-properties-as-group-prop) for more information on `MinSize`, `MaxSize`, `MinInstancesInService` for ASGs.

```
worker:
  # Auto Scaling Group definition for workers. If only `workerCount` is specified, min and max will be the set to that value and `rollingUpdateMinInstancesInService` will be one less.
  autoScalingGroup:
    minSize: 1
    maxSize: 3
    rollingUpdateMinInstancesInService: 2
```

See [the detailed comments in `cluster.yaml`](https://github.com/coreos/kube-aws/blob/master/nodepool/config/templates/cluster.yaml) for further information.

## Deploying a node pool powered by Spot Fleet

Utilizing Spot Fleet gives us chances to dramatically reduce cost being spent on EC2 instances powering Kubernetes worker nodes while achieving reasonable availability.
AWS says cost reduction is up to 90% but the cost would slightly vary among instance types and other users' bids.

Spot Fleet support may change in backward-incompatible ways as it is still an experimenta feature.
So, please use this feature at your own risk.
However, we'd greatly appreciate your feedbacks because they do accelerate improvements in this area!

This feature assumes you already have the IAM role with ARN like "arn:aws:iam::youraccountid:role/aws-ec2-spot-fleet-role" in your own AWS account.
It implies that you've arrived "Spot Requests" in EC2 Dashboard in the AWS console at least once.
See [the AWS documentation describing pre-requisites for Spot Fleet](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-fleet-requests.html#spot-fleet-prerequisites) for details.

To add a node pool powered by Spot Fleet, edit node pool's `cluster.yaml`:

```yaml
worker:
  spotFleet:
    targetCapacity: 3
```

To customize your launch specifications to diversify your pool among instance types other than the defaults, edit `cluster.yaml`:

```yaml
worker:
  spotFleet:
    targetCapacity: 5
    launchSpecifications:
    - weightedCapacity: 1
      instanceType: m3.medium
    - weightedCapacity: 2
      instanceType: m3.large
    - weightedCapacity: 2
      instanceType: m4.large
```

This configuration would normally result in Spot Fleet to bring up 3 instances to meet your target capacity:

* 1x m3.medium = 1 capacity
* 1x m3.large = 2 capacity
* 1x m4.large = 2 capacity

This is achieved by the `diversified` strategy of Spot Fleet.
Please read [the AWS documentation describing Spot Fleet Allocation Strategy](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-fleet.html#spot-fleet-allocation-strategy) for more details.

Please also see [the detailed comments in `cluster.yaml`](https://github.com/coreos/kube-aws/blob/master/nodepool/config/templates/cluster.yaml) and [the GitHub issue summarizing the initial implementation](https://github.com/coreos/kube-aws/issues/112) of this feature for further information.

When you are done with your cluster, [destroy your cluster][aws-step-6]

[aws-step-1]: kubernetes-on-aws.md
[aws-step-2]: kubernetes-on-aws-render.md
[aws-step-3]: kubernetes-on-aws-launch.md
[aws-step-4]: kube-aws-cluster-updates.md
[aws-step-5]: kubernetes-on-aws-node-pool.md
[aws-step-6]: kubernetes-on-aws-destroy.md
