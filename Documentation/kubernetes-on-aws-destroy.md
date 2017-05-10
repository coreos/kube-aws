## Destroy the cluster

When you are done with your cluster, run `kube-aws destroy node-pools <name-of-your-node-pool>` and then `kube-aws destroy` to destroy all the cluster components.

If you created any node pool, you must delete these first by running `kube-aws destroy node-pools <name-of-your-node-pool>` or `kube-aws destroy` will end up failing because node pools still references
AWS resources managed by the main cluster.

If you created any Kubernetes Services of type `LoadBalancer`, you must delete these first, as the CloudFormation cannot be fully destroyed if any externally-managed resources still exist.
