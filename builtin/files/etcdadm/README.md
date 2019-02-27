# etcdadm (etcd administrator)

`etcdadm` is a simple tool which is written in bash to support administrative tasks required in order to
build a highly available etcd3 cluster.

`etcdadm` is named after [kubeadm](https://kubernetes.io/docs/admin/kubeadm/) just for user-friendliness but the tool doesn't relate to kubeadm at all.

## Usage

```bash
# Optional settings
AWS_ACCESS_KEY_ID=... \
AWS_SECRET_ACCESS_KEY=... \
ETCDADM_AWSCLI_DOCKER_IMAGE=quay.io/coreos/awscli \
# Required settings
AWS_DEFAULT_REGION=ap-northeast-1 \
ETCD_VERSION=3.2.13 \
ETCD_DATA_DIR=/var/lib/etcd \
ETCD_INITIAL_CLUSTER=etcd0=http://127.0.0.1:3080,etcd1=http://127.0.0.1:3180,etcd2=http://127.0.0.1:3280 \
ETCDCTL_ENDPOINTS=http://127.0.0.1:3079,etcd1=http://127.0.0.1:3179,etcd2=http://127.0.0.1:3279, \
ETCDADM_MEMBER_COUNT=3 \
ETCDADM_MEMBER_INDEX=0 \
ETCDADM_MEMBER_SYSTEMD_SERVICE_NAME=etcd-member \
ETCDADM_CLUSTER_SNAPSHOTS_S3_URI=s3://myetcdsnapshots/snapshots \
ETCDADM_ETCDCTL_CONTAINER_RUNTIME=rkt \
ETCDADM_MEMBER_FAILURE_PERIOD_LIMIT=10 \
ETCDADM_CLUSTER_FAILURE_PERIOD_LIMIT=30 \
ETCDADM_STATE_FILES_DIR=/var/run/coreos/etcdadm \
  etcdadm [save|restore|check|reconfigure|replace]
```

* `etcdadm save` takes a snapshot of an etcd cluster from the etcd member running on the same node as etcdadm and then
save it in S3
* `etcdadm restore` restores the etcd member running on the same node as etcdadm from a snapshot saved in S3
* `etcdadm check` runs health checks against all the members in an etcd cluster so that `kubeadm reconfigure` updates the etcd member accordingly to the situation
* `etcdadm reconfigure` reconfigures the etcd member on the same node as etcdadm so that it survives:
  * `N/2` or less permanently failed members, by automatically removing a permanently failed member and then re-add it as a brand-new member with empty data according to ["Replace a failed etcd member on CoreOS Container Linux"](https://coreos.com/etcd/docs/latest/etcd-live-cluster-reconfiguration.html#replace-a-failed-etcd-member-on-coreos-container-linux)
  * `(N/2)+1` or more permanently failed members, by automatically initiating a new cluster, from a snapshot if it exists, according to ["etcd disaster recovery on CoreOS Container Linux"](https://coreos.com/etcd/docs/latest/etcd-live-cluster-reconfiguration.html#etcd-disaster-recovery-on-coreos-container-linux)  
* `etcdadm replace` is used to manually recover from an etcd member from a permanent failure. It resets the etcd member running on the same node as etcdadm by:
  1. clearing the contents of the etcd data dir
  2. removing and then re-adding the etcd member by running `etcdctl member remove` and then `etcdctl memer add`
* `etcdadm compact` performs a compaction of the etcd cluster (i.e. removes all version history of the keys leaving the last one) - warning, the operation can adversely affect etcd cluster performance whilst it is running.
* `etcdadm defrag` performed a de-fragmentation operation on the current etcd servers datastore (does not perform this cluster-wide) - - warning, the operation can adversely affect etcd cluster performance whilst it is running.


## Pre-requisites

* etcd3
* rkt
* docker
* systemd
* bash 3+
* jq

## Configuration

### Required settings

**cluster-wise settings**

* `ETCD_MEMBER_COUNT` is the number of etcd members in the cluster. For example, `3` for a 3-nodes etcd cluster
* `ETCD_INITIAL_CLUSTER` should be set to the same value as the one passed to etcd3. For more details, see [the etcd doc about ETCD_INITIAL_CLUSTER](https://coreos.com/etcd/docs/latest/op-guide/configuration.html#initial-cluster)
* `ETCDCTL_ENDPOINTS` should be set to the same value as the one passed to etcd clients. For more details, see [the etcd doc about ETCD_ENDPOINTS](https://coreos.com/kubernetes/docs/latest/getting-started.html#deployment-options)

**Member-wise settings**

* `ETCD_MEMBER_INDEX` is the index of the etcd member in the same node as the etcdadm. For example, `0` when it is the first etcd node in the cluster  
* `ETCDADM_STATE_FILES_DIR` is to where etcdadm stores its state including various rkt uuid files and health check status files

### Optional settings

* `ETCDADM_AWSCLI_DOCKER_IMAGE` is the reference to the `awscli` docker image used from `etcdadm`. If omitted, `quay.io/coreos/awscli` is used as the default

## Limitations

Beware that this tool does not support etcd2.

## Other notes

Although it might be a bad idea, `etcdadm` is implemented in bash (for now) due to the fact it requires access to rkt, docker, systemd and etcd data dir on a host machine to reconfigure an etcd member.

## Developing

### Running integration tests

The following snippet creates a [coreos-vagrant](https://github.com/coreos/coreos-vagrant) virtualbox vm and then bootstraps a 3-nodes etcd3 cluster inside a Container Linux machine and run through various steps to verify etcdadm works.

```bash
$ cat > test.env
AWS_ACCESS_KEY_ID=<YOUR KEY>
AWS_SECRET_ACCESS_KY=<YOUR KEY>
ETCD_CLUSTER_FAILURE_PERIOD_LIMIT=1000
ETCD_MEMBER_FAILURE_PERIOD_LIMIT=1000
TESTER_WORK_DIR=/home/core/tester
AWS_DEFAULT_REGION=<YOUR AWS REGION>
ETCDADM_CLUSTER_SNAPSHOTS_S3_URI=s3://<YOUR BUCKET>/snapshots
ETCDADM_MEMBER__COUNT=3
ETCD_INITIAL_CLUSTER=etcd0=http://127.0.0.1:3080,etcd1=http://127.0.0.1:3180,etcd2=http://127.0.0.1:3280
ETCD_ENDPOINTS=http://127.0.0.1:3079,http://127.0.0.1:3179,http://127.0.0.1:3279

$ make test-integration
```

### Notes for kube-aws users

You can manually invoke `etcdadm` on an etcd node by running:
```bash
set -a; source /var/run/coreos/etcdadm-environment; set +a
/opt/bin/etcdadm <any command>
```
