# Kubernetes Resource Backup / Autosave

A feature to backup of Kubernetes resources can be enabled by specifying the following in `cluster.yaml`: 

```yaml
kubeResourcesAutosave:
    enabled: true
```

When active, a kube-system Deployment schedules a single pod to take and upload snapshots of all Kubernetes resources to S3.
- Backups are taken and exported when the pod (re)starts and then continues to backup in 24 hours intervals.
- Each snapshot resides in a timestamped folder
- The resources have several fields omitted such as status , uid, etc ... this is to allow the possibility of restoring resources inside a fresh cluster
- Resources that reside within namespaces are grouped inside folders labeled with the namespace name
- Resources outside namespaces are grouped at the same directory level as the namespace folders
- The backups are exported to the S3 URI: ```s3://<your-bucket-name>/.../<your-cluster-name>/backup/*```

# Example

A Kubernetes environment has the namespaces:
 - kube-system
 - alpha
 - beta
 
A backup is created on 04/05/2017 at 13:48:33.

The backup is exported to S3 to the path: 

```
s3://my-bucket-name/my-cluster-name/backup/17-05-04_13-48-33
```

Inside the ```17-05-04_13-48-33``` directory are be several .json files of the Kubernetes resources that reside outside namespaces, in addition to a number of folders with names matching the namespaces:

```
17-05-04_13-48-33/kube-system
17-05-04_13-48-33/alpha
17-05-04_13-48-33/beta
17-05-04_13-48-33/persistentvolumes.json
17-05-04_13-48-33/storageclasses.json
...
...
...

```
Inside each namespace folder are be several .json files of the Kubernetes resources that reside inside the respective namespace
```
17-05-04_13-48-33/kube-system/deployments.json
17-05-04_13-48-33/kube-system/statefulsets.json
...
...
...

```

# Notes

- The exportation/synchronisation of the backup files to S3 is not guaranteed to succeed every time, because of this there may be few instances of the pusher container reporting the error: `Could not connect to the endpoint URL: "https://......"`. This is an AWS issue.
If such an error does occur then the pushing process will be attempted at the next push interval - default: 1 minute later.
Please see [#591](https://github.com/kubernetes-incubator/kube-aws/issues/591) for more information
