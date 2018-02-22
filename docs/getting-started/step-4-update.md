# Updating the Kubernetes cluster

## Types of cluster update
There are two distinct categories of cluster update.

* **Parameter-level update**: Only changes to `cluster.yaml` and/or TLS assets in `credentials/` folder are reflected. To enact this type of update. Modifications to CloudFormation or cloud-config userdata templates will not be reflected. In this case, you do not have to re-render:

```sh
kube-aws update --s3-uri s3://<your-bucket-name>/<prefix>
```

* **Full update**: Any change (besides changes made to the etcd cluster- more on that later) will be enacted, including structural changes to CloudFormation and cloudinit templates. This is the type of upgrade that must be run on installing a new version of kube-aws, or more generally when cloudinit or CloudFormation templates are modified:

```sh
kube-aws render stack
kube-aws render credentials
git diff # view changes to rendered assets
kube-aws update --s3-uri s3://<your-bucket-name>/<prefix>
```

## Certificate and access token rotation

The parameter-level update mechanism can be used to rotate in new TLS credentials and access tokens.

More concretely, steps should be taken in order to rotate your certs on nodes are:

* Optionally modify the `externalDNSName` attribute in `cluster.yaml`
* Remove all the `credentials/*.enc` which are cached encrypted certs/keys/tokens to prevent unnecessary node replacement when there's actually no update. See #107 and #237 for more context.
* Render new credentials using kube-aws render credentials:

  ```sh
  kube-aws render credentials
  ```
* Execute the update command like:

  ```sh
  kube-aws update --s3-uri s3://<your-bucket-name>/<prefix>
  ```

## The etcd caveat

There is no solution for hosting an etcd cluster in a way that is easily updateable in this fashion- so updates are automatically masked for the etcd instances. This means that, after the cluster is created, nothing about the etcd ec2 instances is allowed to be updated.

Fortunately, CoreOS update engine will take care of keeping the members of the etcd cluster up-to-date, but you as the operator will not be able to modify them after creation via the update mechanism.

In the (near) future, etcd will be hosted on Kubernetes and this problem will no longer be relevant. Rather than concocting overly complex band-aid, we've decided to "punt" on this issue of the time being.

Once you have successfully updated your cluster, you are ready to [add node pools to your cluster][getting-started-step-5].

[getting-started-step-1]: step-1-configure.md
[getting-started-step-2]: step-2-render.md
[getting-started-step-3]: step-3-launch.md
[getting-started-step-4]: step-4-update.md
[getting-started-step-5]: step-5-add-node-pool.md
[getting-started-step-6]: step-6-configure-add-ons.md
[getting-started-step-7]: step-7-destroy.md
