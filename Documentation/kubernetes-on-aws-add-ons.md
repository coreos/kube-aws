# Configuring Kubernetes Add-ons

kube-aws has built-in supports for several Kubernetes add-ons known to require additional configurations beforehand.

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

When you are done with your cluster, [destroy your cluster][aws-step-7]

[aws-step-1]: kubernetes-on-aws.md
[aws-step-2]: kubernetes-on-aws-render.md
[aws-step-3]: kubernetes-on-aws-launch.md
[aws-step-4]: kube-aws-cluster-updates.md
[aws-step-5]: kubernetes-on-aws-node-pool.md
[aws-step-6]: kubernetes-on-aws-add-ons.md
[aws-step-7]: kubernetes-on-aws-destroy.md
