# Known Limitations

## hostPort doesn't work

This isn't really an issue of kube-aws but rather Kubernetes and/or CNI issue.
Anyways, it doesn't work if `hostNetwork: false`.

If you want to deploy `nginx-ingress-controller` which requires `hostPort`, just set `hostNetwork: true`:

```
        spec:
          hostNetwork: true
          containers:
          - image: gcr.io/google_containers/nginx-ingress-controller:0.8.3
            name: nginx-ingress-lb
```

Relevant kube-aws issue: [does hostPort not work on kube-aws/CoreOS?](https://github.com/kubernetes-incubator/kube-aws/issues/91)

See [the related upstream issue](https://github.com/kubernetes/kubernetes/issues/23920#issuecomment-254918942) for more information.

This limitation is also documented in [the official Kubernetes doc](http://kubernetes.io/docs/admin/network-plugins/#cni).
