# Known Limitations

## hostPort doesn't work before Kubernetes 1.7.0

This is a known issue with Kubernetes while using CNI with no available workaround for kube-aws. `hostPort` does not work if `hostNetwork: false`.

If you want to deploy an Ingress controller such as `nginx-ingress-controller` which requires `hostPort`, just set `hostNetwork: true`:

```yaml
spec:
   hostNetwork: true
   containers:
   - image: gcr.io/google_containers/nginx-ingress-controller:0.8.3
     name: nginx-ingress-lb
```

This is fixed in Kubernetes 1.7.0 and later.

Relevant kube-aws issue: [does hostPort not work on kube-aws/CoreOS?](https://github.com/kubernetes-incubator/kube-aws/issues/91)

See [the related upstream issue](https://github.com/kubernetes/kubernetes/issues/23920#issuecomment-254918942) for more information.

This limitation is also documented in [the official Kubernetes doc](http://kubernetes.io/docs/admin/network-plugins/#cni).
