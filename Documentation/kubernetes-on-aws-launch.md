# Launch your Kubernetes cluster on AWS

This is the [third step of running Kubernetes on AWS][aws-step-1]. We're ready to launch the Kubernetes cluster.

## Create the instances defined in the CloudFormation template

Now for the exciting part, creating your cluster:

```sh
$ kube-aws up --s3-uri s3://<your-bucket-name>/<prefix>
```

**NOTE**: It can take some time after `kube-aws up` completes before the cluster is available. When the cluster is first being launched, it must download all container images for the cluster components (Kubernetes, dns, heapster, etc). Depending on the speed of your connection, it can take a few minutes before the Kubernetes api-server is available.

## Configure DNS

If you configured Route 53 settings in your configuration above via `createRecordSet`, a host record has already been created for you.

Otherwise, navigate to the DNS registrar hosting the zone for the provided external DNS name. Ensure a single A record exists, routing the value of `externalDNSName` defined in `cluster.yaml` to the externally-accessible IP of the master node instance.

You can invoke `kube-aws status` to get the cluster API endpoint after cluster creation, if necessary. This command can take a while.

## Access the cluster

A kubectl config file will be written to a `kubeconfig` file, which can be used to interact with your Kubernetes cluster like so:

```sh
$ kubectl --kubeconfig=kubeconfig get nodes
```

If the container images are still downloading and/or the API server isn't accessible yet, the kubectl command above may show output similar to:

```
The connection to the server <externalDNSName>:443 was refused - did you specify the right host or port?
```

Wait a few more minutes for everything to complete.

Once the API server is running, you should see:

```sh
$ kubectl --kubeconfig=kubeconfig get nodes
NAME                                       STATUS                     AGE
ip-10-0-0-xxx.us-west-1.compute.internal   Ready                      5m
ip-10-0-0-xxx.us-west-1.compute.internal   Ready                      5m
ip-10-0-0-xx.us-west-1.compute.internal    Ready,SchedulingDisabled   5m
```

<div class="co-m-docs-next-step">
  <p><strong>You're all done!</strong> The cluster is ready to use.</p>
  <p>For full lifecycle information, read on below.</p>
</div>

## Export the CloudFormation stack

If you want to share, audit or back up your stack, use the export flag:

```sh
$ kube-aws up --export
```

Once you have successfully launched your cluster, you are ready to [update your cluster][aws-step-4].

[aws-step-1]: kubernetes-on-aws.md
[aws-step-2]: kubernetes-on-aws-render.md
[aws-step-3]: kubernetes-on-aws-launch.md
[aws-step-4]: kube-aws-cluster-updates.md
[aws-step-5]: kubernetes-on-aws-node-pool.md
[aws-step-6]: kubernetes-on-aws-destroy.md
