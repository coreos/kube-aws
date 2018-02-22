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

kube-aws creates a node pool in a single AZ by default.
On top of that, you can add one or more node pool in an another AZ to achieve Multi-AZ.

Assuming you already have a subnet and a node pool in the subnet:

```yaml
subnets:
- name: managedPublicSubnetIn1a
  availabilityZone: us-west-1a
  instanceCIDR: 10.0.0.0/24

worker:
  nodePools:
    - name: pool1
      subnets:
      - name: managedPublicSubnetIn1a
```


Edit the `cluster.yaml` file to add the second node pool:

```yaml
subnets:
- name: managedPublicSubnetIn1a
  availabilityZone: us-west-1a
  instanceCIDR: 10.0.0.0/24
- name: managedPublicSubnetIn1c
  availabilityZone: us-west-1c
  instanceCIDR: 10.0.1.0/24

worker:
  nodePools:
    - name: pool1
      subnets:
      - name: managedPublicSubnetIn1a
    - name: pool2
      subnets:
      - name: managedPublicSubnetIn1c
```

Launch the secondary node pool by running `kube-aws update`:

```
$ kube-aws update \
  --s3-uri s3://<my-bucket>/<optional-prefix>
```

Beware that you have to associate only 1 AZ to a node pool or cluster-autoscaler may end up failing to reliably add nodes on demand due to the fact
that what cluster-autoscaler does is to increase/decrease the desired capacity hence it has no way to selectively add node(s) in a desired AZ.

Also note that deployment of cluster-autoscaler is currently out of scope of this documentation.
Please read [cluster-autoscaler's documentation](https://github.com/kubernetes/contrib/blob/master/cluster-autoscaler/cloudprovider/aws/README.md) for instructions on it.

## Customizing min/max size of the auto scaling group

If you've chosen to power your worker nodes in a node pool with an auto scaling group, you can customize `MinSize`, `MaxSize`, `RollingUpdateMinInstancesInService` in `cluster.yaml`:

Please read [the AWS documentation](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-as-group.html#aws-properties-as-group-prop) for more information on `MinSize`, `MaxSize`, `MinInstancesInService` for ASGs.

```yaml
worker:
  nodePools:
  - name: pool1
    autoScalingGroup:
      minSize: 1
      maxSize: 3
      rollingUpdateMinInstancesInService: 2
```

See [the detailed comments in `cluster.yaml`](https://github.com/kubernetes-incubator/kube-aws/blob/master/core/controlplane/config/templates/cluster.yaml) for further information.

## Deploying a node pool powered by Spot Fleet

Utilizing Spot Fleet gives us chances to dramatically reduce cost being spent on EC2 instances powering Kubernetes worker nodes while achieving reasonable availability.
AWS says cost reduction is up to 90% but the cost would slightly vary among instance types and other users' bids.

Spot Fleet support may change in backward-incompatible ways as it is still an experimental feature.
So, please use this feature at your own risk.
However, we'd greatly appreciate your feedbacks because they do accelerate improvements in this area!

### Known Limitations

* Running `kube-aws update` to increase or decrease `targetCapacity` of a spot fleet results in a complete replacement of the Spot Fleet hence some downtime. [This is due to how CloudFormation works for updating a Spot Fleet](http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-spotfleet.html#d0e60520)
   * It is recommended to temporarily bring up an another, spare node pool to maintain the whole cluster capacity at a certain level while replacing the spot fleet.

### Pre-requisites

This feature assumes you already have the IAM role with ARN like "arn:aws:iam::youraccountid:role/aws-ec2-spot-fleet-role" in your own AWS account.
It implies that you've arrived "Spot Requests" in EC2 Dashboard in the AWS console at least once.
See [the AWS documentation describing pre-requisites for Spot Fleet](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-fleet-requests.html#spot-fleet-prerequisites) for details.

### Steps

To add a node pool powered by Spot Fleet, edit node pool's `cluster.yaml`:

```yaml
worker:
  nodePools:
  - name: pool1
    spotFleet:
      targetCapacity: 3
```

To customize your launch specifications to diversify your pool among instance types other than the defaults, edit `cluster.yaml`:

```yaml
worker:
  nodePools:
  - name: pool1
    spotFleet:
      targetCapacity: 5
      launchSpecifications:
      - weightedCapacity: 1
        instanceType: t2.medium
      - weightedCapacity: 2
        instanceType: m3.large
      - weightedCapacity: 2
        instanceType: m4.large
```

This configuration would normally result in Spot Fleet to bring up 3 instances to meet your target capacity:

* 1x t2.medium = 1 capacity
* 1x m3.large = 2 capacity
* 1x m4.large = 2 capacity

This is achieved by the `diversified` strategy of Spot Fleet.
Please read [the AWS documentation describing Spot Fleet Allocation Strategy](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-fleet.html#spot-fleet-allocation-strategy) for more details.

Please also see [the detailed comments in `cluster.yaml`](https://github.com/kubernetes-incubator/kube-aws/blob/master/core/controlplane/config/templates/cluster.yaml) and [the GitHub issue summarizing the initial implementation](https://github.com/kubernetes-incubator/kube-aws/issues/112) of this feature for further information.

You can optionally [configure various Kubernetes add-ons][getting-started-step-6] according to your requirements.
When you are done with your cluster, [destroy your cluster][getting-started-step-7]

[getting-started-step-1]: step-1-configure.md
[getting-started-step-2]: step-2-render.md
[getting-started-step-3]: step-3-launch.md
[getting-started-step-4]: step-4-update.md
[getting-started-step-5]: step-5-add-node-pool.md
[getting-started-step-6]: step-6-configure-add-ons.md
[getting-started-step-7]: step-7-destroy.md
