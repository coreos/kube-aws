# Backup and Restore for etcd

## Backup

### Manually taking an etcd snapshot

ssh into one of etcd nodes and run the following command:

```bash
set -a; source /var/run/coreos/etcdadm-environment; set +a
/opt/bin/etcdadm save
```

The command takes an etcd snapshot by running an appropriate `etcdctl snapshot save` command.
The snapshot is then exported to the S3 URI: `s3://<your-bucket-name>/.../<your-cluster-name>/exported/etcd-snapshots/snapshot.db`.

### Automatically taking an etcd snapshot

A feature to periodically take a snapshot of an etcd cluster can be enabled by specifying the following in `cluster.yaml`:

```yaml
etcd:
  snapshot:
    automated: true
```

When enabled, the command `etcdadm save` is called periodically (every 1 minute by default) via a systemd timer.

## Restore

Please beware that you must have taken an etcd snapshot beforehand to restore your cluster.
An etcd snapshot can be taken manually or automatically according to the steps described above.

### Manually restoring a permanently failed etcd node from etcd snapshot

It is impossible!
However, you can recover a permanently failed etcd node, without losing data, by "resetting" the node.
More concretely, you can run the following commands to remove the etcd member from the cluster, wipe etcd data, and then re-add the member to the cluster:

```bash
sudo systemctl stop etcd-member.service

set -a; source /var/run/coreos/etcdadm-environment; set +a
/opt/bin/etcdadm replace

sudo systemctl start etcd-member.service
```

The reset member eventually catches up data from the etcd cluster hence the recovery is done without losing data.

For more details, I'd suggest you to read [the revelant upstream issue](https://github.com/kubernetes/kubernetes/issues/40027#issuecomment-283501556).

### Manually restoring a cluster from etcd snapshot

ssh into every etcd node and stop the etcd3 process:

```bash
for h in $hosts; do
  ssh -i path/to/your/key core@$h sudo systemctl stop etcd-member.service
done
```

and then sart the etcd3 process:

```bash
for h in $hosts; do
  ssh -i path/to/your/key core@$h sudo systemctl start etcd-member.service
done
```

Doing this triggers the automated disaster recovery processes across etcd nodes by running `etcdadm-reconfigure.service`
and your cluster will eventually be restored from the snapshot stored at `s3://<your-bucket-name>/.../<your-cluster-name>/exported/etcd-snapshots/snapshot.db`.

### Automatic recovery

A feature to automatically restore a permanently failed etcd member or a cluster can be enabled by specifying:

```yaml
etcd:
  disasterRecovery:
    automated: true
```

When enabled,
- The command `etcdadm check` is called periodically by a systemd timer
  - The etcd cluster and each etcd node(=member) is checked by running `etcdctl endpoint health` command
- When up to `1/N` etcd nodes failed successive health checks, it will be removed as an etcd member and then added again as a new member
   - The new member eventually catches up data from the etcd cluster
- When more than `1/N` etcd nodes failed successive health checks, a disaster recovery process is executed to recover all the etcd nodes from the latest etcd snapshot
