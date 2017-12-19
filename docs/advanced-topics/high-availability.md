# High Availability

To achieve high availability using kube-aws, it is recommended to:

* Specify at least 3 for `etcd.count` in `cluster.yaml`. See [Optimal Cluster Size](https://coreos.com/etcd/docs/latest/v2/admin_guide.html#optimal-cluster-size) for details of etcd recommendations
* Specify at least 2 for `controller.count` in `cluster.yaml`
* Use 2 or more worker nodes,
* Avoid `t2.medium` or smaller instances for etcd and controller nodes. See [this issue](https://github.com/kubernetes-incubator/kube-aws/issues/138) for some additional discussion.

# Additional Reading

There's some additional documentation about [Building High-Availability Clusters](https://kubernetes.io/docs/admin/high-availability/) on the main Kubernetes documentation site. Although kube-aws will taken care of most of those concerns for you, it can be worth a read for a deeper understanding.
