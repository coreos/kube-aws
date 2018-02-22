# Configuring Kubernetes Add-ons

kube-aws has built-in supports for several Kubernetes add-ons known to require additional configurations beforehand.

## cluster-autoscaler

[cluster-autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) is an add-on which automatically
scales in/out your k8s cluster by removing/adding worker nodes according to resource utilization per node.

To enable cluster-autoscaler, add the below settings to your cluster.yaml:

```yaml
addons:
  clusterAutoscaler:
    enabled: true
worker:
  nodePools:
  - name: scaled
    autoScalingGroup:
      minSize: 1
      maxSize: 10
    autoscaling:
      clusterAutoscaler:
        enabled: true
  - name: notScaled
    autoScalingGroup:
      minSize: 2
      maxSize: 4
```

The above example configuration would:

* By `addons.clusterAutoscaler.enabled`:
  * Provide controller nodes appropriate IAM permissions to call necessary AWS APIs from CA
  * Create a k8s deployment to run CA on one of controller nodes, so that CA can utilize the IAM permissions
* By `worker.nodePools[0].autoscaling.clusterAutoscaler.enabled`:
  * If there are unschedulable, pending pod(s) that is requesting more capacity, CA will add more nodes to the `scaled` node pool, up until the max size `10`
  * If there are no unschdulable, pending pod(s) that is waiting for more capacity and one or more nodes are in low utlization, CA will remove node(s), down until the min size `1`
* The second node pool `notScaled` is scaled manually by YOU, because you had not the autoscaling on it(=missing `autoscaling.clusterAutoscaler.enabled`)

## kube2iam
 
[kube2iam](https://github.com/jtblin/kube2iam) is an add-on which provides IAM credentials for target IAM roles to pods running inside a Kubernetes cluster based on annotations.
To allow kube2iam deployed to worker and controller nodes to assume target roles, you need the following configurations.

1. IAM roles associated to worker and controller nodes requires an IAM policy:
 
  ```json
  {
    "Action": "sts:AssumeRole",
    "Resource": "*",
    "Effect": "Allow"
  }
  ```

  To add the policy to controller nodes, set `experimental.kube2IamSupport.enabled` to `true` in your `cluster.yaml`.
  For worker nodes, it is `worker.nodePools[].kube2IamSupport.enabled`.

2. Target IAM roles needs to change trust relationships to allow kube-aws worker/controller IAM role to assume the target roles.

  As CloudFormation generates unpredictable role names containing random IDs by default, it is recommended to make them predictable at first so that you can easily automate configuring trust relationships afterwards.
  To make worker/controller role names predictable, set `controller.managedIamRoleName` for controller and `worker.nodePools[].managedIamRoleName` for worker nodes.
  `managedIamRoleName`s becomes suffixes of the resulting worker/controller role names. 
  
  Please beware that configuration of target roles' trust relationships are out-of-scope of kube-aws.
  Please see [the part of kube2iam doc](https://github.com/jtblin/kube2iam#iam-roles) for more information. Basically,
  you need to point `Principal` to the ARN of a resulting worker/controller IAM role which would look like `arn:aws:iam::<your aws account id>:role/<stack-name>-<managed iam role name>`. 

Finally, an example `cluster.yaml` usable with kube2iam would look like:

```yaml
# for controller nodes
controller:
  managedIamRoleName: mycontrollerrole
 
experimental:
  kube2IamSupport:
    enabled: true

# for worker nodes
worker:
  nodePools:
  - name: mypool
    managedIamRoleName: myworkerrole
    kube2IamSupport:
      enabled: true
 ```

See [the relevant GitHub issue](https://github.com/kubernetes-incubator/kube-aws/issues/253) for more information.

You can reference controller and worker IAM Roles in a separate CloudFormation stack that provides roles to assume:

```yaml
...
Parameters:
  KubeAWSStackName:
    Type: String
Resources:
  IAMRole:
    Type: AWS::IAM::Role
    Properties:
      AssumeRolePolicyDocument:
        Statement:
          - Effect: Allow
            Action: sts:AssumeRole
            Principal:
              Service: ec2.amazonaws.com
          - Effect: Allow
            Action: sts:AssumeRole
            Principal:
              AWS:
                Fn::ImportValue: !Sub "${KubeAWSStackName}-ControllerIAMRoleArn"
          - Effect: Allow
            Action: sts:AssumeRole
            Principal:
              AWS:
                Fn::ImportValue: !Sub "${KubeAWSStackName}-NodePool<Node Pool Name>WorkerIAMRoleArn"
      ...
```

When you are done with your cluster, [destroy your cluster][getting-started-step-7]

[getting-started-step-1]: step-1-configure.md
[getting-started-step-2]: step-2-render.md
[getting-started-step-3]: step-3-launch.md
[getting-started-step-4]: step-4-update.md
[getting-started-step-5]: step-5-add-node-pool.md
[getting-started-step-6]: step-6-configure-add-ons.md
[getting-started-step-7]: step-7-destroy.md
