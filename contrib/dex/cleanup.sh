#!/usr/bin/env bash

kubectl delete -f ingress/
kubectl delete -f elb
kubectl delete secret dex-tls-secret -n kube-system