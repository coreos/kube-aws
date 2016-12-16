# Roadmap

This document is meant to provide high-level but actionable objectives for future kube-aws deveploment.
Please file an issue to make suggestions on this roadmap!

## Every stage

  * Provide article walking users through:
    * setting up a cluster from scratch
    * using/enabling new features
    * (breaking changes)

## Stage 1: v0.9.2

  * Node Pools
    * Worker nodes optionally powered by Spot Fleet
  * Clean cluster upgrades (preventing downtime, make sure they succeed)

## Stage 2: v0.9.3

  * Cluster Auto Scaling
    * Including partial support for auto-scaling worker nodes, kube-dns
  * Self-hosted Calico

## Stage 3: v0.9.4

  * etcd improvements
    * Backups
    * Recovery
  * YAML CloudFormation templates

## Stage N: v0.9.x

  * Bootkube switch
    * `kube-aws` can largely go into maintenance mode when k8s upgrades can be safely achieved on self-hosted clusters.
