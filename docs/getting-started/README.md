# Getting Started

Deploy a fully-functional Kubernetes cluster using AWS CloudFormation. Your cluster will be configured to use AWS features to enhance Kubernetes. For example, Kubernetes may automatically provision an Elastic Load Balancer for each Kubernetes Service.

The [kube-aws](https://github.com/kubernetes-incubator/kube-aws/releases) CLI tool can be used to automate cluster deployment to AWS.

After completing this guide, a deployer will be able to interact with the Kubernetes API from their workstation using the `kubectl` CLI tool.

Each of the steps will cover:

* [Pre-requisites][getting-started-prerequisites.md]
* [Step 1: Configure][getting-started-step-1]
  * Download the latest release of kube-aws
  * Define account and cluster settings
* [Step 2: Render][getting-started-step-2]
  * Compile a re-usable CloudFormation template for the cluster
  * Optionally adjust template configuration
  * Validate the rendered CloudFormation stack
* [Step 3: Launch][getting-started-step-3]
  * Create the CloudFormation stack and start our EC2 machines
  * Set up CLI access to the new cluster
* [Step 4: Update][getting-started-step-4]
  * Update the CloudFormation stack
* [Step 5: Add Node Pool][getting-started-step-5]
  * Create the additional pool of worker nodes
  * Adjust template configuration for each pool of worker nodes
  * Required to support [cluster-autoscaler](https://github.com/kubernetes/contrib/tree/master/cluster-autoscaler)
* [Step 6: Configure add-ons][getting-started-step-6]
  * Configure various Kubernetes add-ons
* [Step 7: Destroy][getting-started-step-7]
  * Destroy the cluster

Let's get started with [Step 1](step-1-configure.md)!

[getting-started-prerequisites.md]: prerequisites.md
[getting-started-step-1]: step-1-configure.md
[getting-started-step-2]: step-2-render.md
[getting-started-step-3]: step-3-launch.md
[getting-started-step-4]: step-4-update.md
[getting-started-step-5]: step-5-add-node-pool.md
[getting-started-step-6]: step-6-configure-add-ons.md
[getting-started-step-7]: step-7-destroy.md
