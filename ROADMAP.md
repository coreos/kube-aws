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
    * See https://github.com/coreos/kube-aws/issues/238

## v0.9.5

  * etcd improvements
    * Automatic recovery from temporary node failures
    * Manual disaster recovery w/ or w/o recreating etcd nodes
  * Experimental support for an internal domain and custom hostnames for etcd nodes
    
## v0.9.6

  * Cluster Auto Scaling
    * Support for auto-scaling worker nodes via:
      * Dynamic reconfiguration of cluster-autoscaler
      * Automatic discovery of target node pools for cluster-autoscaler
    * Requires much work on CA side
  
## v0.9.7

  * Kubernetes 1.6
  * Etcd v3 support as it is enabled by default in 1.6: https://github.com/kubernetes/kubernetes/issues/22448#event-913208648

## v0.9.x

  * Migrate from coreos-cloudinit to ignition for node bootstrapping
  * YAML CloudFormation templates
  * Bootkube switch
    * `kube-aws` can largely go into maintenance mode when k8s upgrades can be safely achieved on self-hosted clusters.
