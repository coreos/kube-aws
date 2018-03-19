# Roadmap

This document is meant to provide high-level but actionable objectives for future kube-aws deveploment.
Please file an issue to make suggestions on this roadmap!

## Every release

  * Provide article walking users through:
    * Setting up a cluster from scratch
    * Using/enabling new features
    * (Breaking changes)
  * Drop deprecated configuration syntax and flags, options
  * Revise this roadmap

## v0.9.2

  * Node Pools
    * Worker nodes optionally powered by Spot Fleet
  * Clean cluster upgrades (preventing downtime, make sure they succeed)

## v0.9.3

  * Kubernetes 1.5.1
     * Auto-scaled kube-dns
  * Self-hosted Calico
  * Very limited, almost theoretical support for automatic reconfiguration of cluster-autoscaler

## v0.9.4

  * Kubernetes 1.5.3
  * Work-around the 16KB userdata limit in size
  * Experimental support for
    * Private subnets and NAT gateways for etcd, controller and worker nodes
    * Deployments to existing subnets
  * Rethink how node pools are implemented
    * See https://github.com/kubernetes-incubator/kube-aws/issues/238

## v0.9.5

  * Kubernetes 1.5.4
  * etcd: Automatic recovery from temporary etcd node failures
  * etcd: Experimental support for an internal domain and custom hostnames for etcd nodes

## v0.9.6

  * Kubernetes 1.6
  * etcd: etcd v3 support #381
    * It is enabled by default in 1.6: https://github.com/kubernetes/kubernetes/issues/22448#event-913208648
  * etcd: Manual/Automatic recovery from permanent etcd node failures #417

## v0.9.7

  * Cluster Auto Scaling
    * Support for auto-scaling worker nodes via:
      * Dynamic reconfiguration of cluster-autoscaler
      * Automatic discovery of target node pools for cluster-autoscaler
    * Requires much work on CA side

## v0.9.8

  * Kubernetes 1.7
  * More and [more RBAC support](https://github.com/kubernetes-incubator/kube-aws/pull/675#issuecomment-303660360) (@camilb, @c-knowles)
  * Experimental support for kube-aws plugins
  * Tiller installed by default
    * For use from the plugin support
  * Scalability improvements
    * More efficient node draining(@danielfm)
  * Cluster-provisioning observability improvements
    * Streaming stack events & journald logs (@jollinshead)

## v0.9.9

  * Kubernetes 1.8
  * RBAC enabled by default
  * Security improvements
    * NodeRestriction admission controller + Node authorizer + Kubelet’s credential rotation (@danielfm)
  * [Optional] Several kube-aws core features as plugins

## v0.9.10

  * Kubernetes 1.9.x
  * Security+Usability improvements
    * [kiam](https://github.com/uswitch/kiam/) integration (#1055)
    * [authenticator](https://github.com/heptio/authenticator) integration (#1153)
    * Support for pregenerating IAM roles used by kube2iam/kiam (#1145, #1150)
  * Operatability improvements
    * [More manageable Calico + Flannel](https://github.com/kubernetes-incubator/kube-aws/pull/675#issuecomment-303669142) (@redbaron) (#909)
    * Graduate from relying on CloudFormation nested stacks (#1112)
    * Ease certificate rotation (#1146)

## v0.9.11

  * Kubernetes 1.10
  * (After easy H/A controller support) kubeadm support to simplify k8s components configuration (#654)
    * Reduces the amount of code required in kube-aws
    * To better follow upstream improvements on how k8s components are deployed
  * (After scalability/reliability/upgradability cleared) istio integration
    * Probably after k8s supported injecting init containers from PodPreset
      * [Upstream issue](https://github.com/kubernetes/kubernetes/issues/43874)
  * Migrate from coreos-cloudinit to ignition for node bootstrapping (@redbaron)

## v0.9.12

  * Bootkube switch
    * `kube-aws` can largely go into maintenance mode when k8s upgrades can be safely achieved on self-hosted clusters.

## v0.9.x

  * YAML CloudFormation templates?
