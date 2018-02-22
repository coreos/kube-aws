# Quick Start

Get started with kube-aws and deploy a fully-functional Kubernetes cluster running on CoreOS Container Linux using AWS CloudFormation.

After completing this guide, you will be able to deploy applications to Kubernetes on AWS and interact with the Kubernetes API using the `kubectl` CLI tool.

# Pre-requisites

Prior to using setting up your first Kubernetes cluster using kube-aws, you will need to setup the following. More details on each pre-requisite are available in the rest of the documentation.

1. [Install](http://docs.aws.amazon.com/cli/latest/userguide/installing.html) and [Configure](http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html) the AWS CLI
1. [Install and Set Up kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) which is the CLI for controlling a Kubernetes cluster
1. Create an [EC2 Key Pair](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-key-pairs.html) in your chosen AWS region and record the name of the key for step 2 in this guide
1. Have a Route 53 hosted zone ready to expose the Kubernetes API and record the hosted zone ID and domain name for step 2 in this guide
1. Create a [KMS Key](http://docs.aws.amazon.com/kms/latest/developerguide/create-keys.html) in your chosen AWS region and record the ARN for step 2 in this guide
1. Create an [S3 Bucket](http://docs.aws.amazon.com/AmazonS3/latest/gsg/CreatingABucket.html) to store kube-aws assets and record the bucket name for step 2 and 3 in this guide

# Step 1: Download kube-aws

Go to the [releases](https://github.com/kubernetes-incubator/kube-aws/releases) and download the latest release tarball for your architecture. Extract the binary and add kube-aws to your path:

```bash
➜ tar zxvf kube-aws-${PLATFORM}.tar.gz
➜ sudo mv ${PLATFORM}/kube-aws /usr/local/bin
➜ kube-aws --help
```

# Step 2: Render

First run `init` using the information from the pre-requisites section. For example:

```bash
➜ kube-aws init \
  --cluster-name=quick-start-k8 \
  --region=us-west-1 \
  --availability-zone=us-west-1a \
  --hosted-zone-id=ZBN159WIK8JJD \
  --external-dns-name=quick-start-k8s.mycompany.com \
  --key-name=ec2-key-pair-name \
  --kms-key-arn="arn:aws:kms:us-west-1:123456789012:key/c4f79cb0-f9fb-434a-ac3c-47c5697d51e6"
```

This will generate a `cluster.yaml` file which forms the main configuration for your new cluster. The `cluster.yaml` has many options to adjust your cluster, leave them as the defaults for now.

Next use `render credentials` to generate new credentials for your cluster into the `credentials` directory:

```bash
➜ kube-aws render credentials --generate-ca
```

The files generated are TLS assets which allow communication between nodes and also allow super admins to administer the cluster. After the quick start you may wish to use your own CA assets.

Next use `render stack` to generate the CloudFormation stack templates and user data into the `stack-templates` and `userdata` directories:

```bash
➜ kube-aws render stack
```

The files generated form the basis of the deployment.

Before we move onto deploying, let's run `validate` to check the work above using the S3 bucket name from the pre-requisites section. For example:

```bash
➜ kube-aws validate --s3-uri=s3://kube-aws-assets/
```

# Step 3: Launch

Now you've generated and validated the various assets needed to launch a new cluster, let's run the deploy! Run `up` using the S3 bucket name from the pre-requisites section. For example:

```bash
➜ kube-aws up --s3-uri=s3://kube-aws-assets/
```

# Step 4: Deploy an Application

Let's deploy our first application to the new cluster, nginx is easy to start with:

```bash
➜ export KUBECONFIG=$PWD/kubeconfig
➜ kubectl run quick-start-nginx --image=nginx --port=80
deployment "quick-start-nginx" created

➜ kubectl get pods
NAME                                 READY     STATUS    RESTARTS   AGE
quick-start-nginx-6687bdfc67-6qsr8   1/1       Running   0          10s
```

You can see above the pod is running and ready. To try it out we can forward a local port to the pod:

```bash
➜ kubectl port-forward $(kubectl get pods -l "run=quick-start-nginx" -o jsonpath="{.items[0].metadata.name}") 8080:80  
Forwarding from 127.0.0.1:8080 -> 80
```

Then load the nginx home page in a browser:

```bash
➜ open http://localhost:8080/
```

You should see a `Welcome to nginx!` page.

If you'd like to try exposing a public load balancer, first run:

```bash
➜ kubectl expose deployment quick-start-nginx --port=80 --type=LoadBalancer
```

Wait a few seconds for Kubernetes to create an AWS ELB to to expose the service and then run:

```bash
➜ open http://$(kubectl get svc quick-start-nginx -o jsonpath="{.status.loadBalancer.ingress[0].hostname}")
```

You should see the same `Welcome to nginx!` page as above.

The above commands demonstrate some basic `kubectl` imperative commands to create a Kubernetes Deployment and Service object. Declarative object configuration is also available, for more information see [Kubernetes Object Management](https://kubernetes.io/docs/tutorials/object-management-kubectl/object-management/).

# Step 5: Tear Down

Once you no longer need the quick start cluster created during this guide, tear it down:

```bash
➜ kubectl delete svc quick-start-nginx
service "quick-start-nginx" deleted

➜ kube-aws destroy
```

The first command deletes the Service object created in step 4 so the AWS ELB is removed otherwise the network interface attachments may block the CloudFormation stack from being deleted.