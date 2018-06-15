## Destroy the cluster

When you are done with your cluster `kube-aws destroy` to destroy all the cluster components. It will as for confirmation. Use `--force` to skip confirmation step.

If you created any Kubernetes Services of type `LoadBalancer`, you must delete these first, as the CloudFormation cannot be fully destroyed if any externally-managed resources still exist.
