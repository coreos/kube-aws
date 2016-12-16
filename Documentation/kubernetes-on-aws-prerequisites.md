# Pre-requisites

If you're deploying a cluster with kube-aws:

* [EC2 instances whose types are larger than or equal to `m3.medium` should be chosen for the cluster to work reliably](https://github.com/coreos/kube-aws/issues/138)
* [At least 3 etcd, 2 controller, 2 worker nodes are required to achieve high availability](https://github.com/coreos/kube-aws/issues/138#issuecomment-266432162)

## Deploying to an existing VPC

If you're deploying a cluster to an existing VPC:

* Internet Gateway needs to be added to VPC before cluster can be created
  * Or [all the nodes will fail to launch because they can't pull docker images or ACIs required to run essential processes like fleet, hyperkube, etcd, awscli, cfn-signal, cfn-init.](https://github.com/coreos/kube-aws/issues/120)
* Existing route tables to be reused by kube-aws must be tagged with the key `KubernetesCluster` and your cluster's name for the value.
  * Or [Kubernetes will fail to create ELBs correspond to Kubernetes services with `type=LoadBalancer`](https://github.com/coreos/kube-aws/issues/135)
* ["DNS Hostnames" must be turned on before cluster can be created](https://github.com/coreos/kube-aws/issues/119)
  * Or etcd nodes are unable to communicate each other thus the cluster doesn't work at all

Once you understand pre-requisites, you are [ready to launch your first Kubernetes cluster][aws-step-1].

[aws-step-1]: kubernetes-on-aws.md
[aws-step-2]: kubernetes-on-aws-render.md
[aws-step-3]: kubernetes-on-aws-launch.md
[aws-step-4]: kube-aws-cluster-updates.md
[aws-step-5]: kubernetes-on-aws-node-pool.md
[aws-step-6]: kubernetes-on-aws-destroy.md
