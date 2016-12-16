## Destroy the cluster

When you are done with your cluster, run `kube-aws node-pools destroy` and `kube-aws destroy` then all cluster components will be destroyed.

If you created any node pool, you must delete these first by running `kube-aws node-pools destroy`, or `kube-aws destroy` will end up failing because node pools still references
AWS resources managed by the main cluster.

If you created any Kubernetes Services of type `LoadBalancer`, you must delete these first, as the CloudFormation cannot be fully destroyed if any externally-managed resources still exist.
