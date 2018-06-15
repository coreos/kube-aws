# Introduction

A draft design for [kube-aws plugins](https://github.com/kubernetes-incubator/kube-aws/issues/509). Plugins are initially defined as additions to a kube-aws generated
cluster that are not part of the core feature set of providing an easy to use, highly available,
best practice cluster on AWS. Plugins can also later be used for built-in functionality to provide a more modular deployment and ability to test.

# Initial Use Cases

1. A user wishes to deploy a component by default to their cluster instances outside of Kubernetes, e.g. a 
monitoring agent.
1. A user wishes to deploy a component into their cluster by default that is not part of core
Kubernetes such as cluster wide services, addons, ingress controller or even persistent volumes.
Reasons can vary from company restrictions to always having this component form part of every 
cluster. e.g. [the rescheduler](https://github.com/kubernetes-incubator/kube-aws/issues/118)
1. A user wishes to add [customised parts to the CloudFormation template](https://github.com/kubernetes-incubator/kube-aws/issues/96#issuecomment-263835808) 
produced by kube-aws. Anything they add would need access to the same parameters and template 
engine that already exists.
1. A user wishes to customise existing parts of the CloudFormation template produced by kube-aws
without necessarily having to diff all the files for changes with there sometimes being large
conflicts or huge updates due to kube-aws changes that can be difficult to digest while
being able to merge in the simple changes they need. Examples 
[here](https://github.com/kubernetes-incubator/kube-aws/issues/340) and 
[here](https://github.com/kubernetes-incubator/kube-aws/issues/338)

# Specific Examples

* Deployment outside of Kubernetes - Dex uses some additional API server parameter, Datadog agent still has more functionality if not using the containerised version
* Deployment to Kubernetes - rescheduler, kube2iam
* Add new parts to CloudFormation - bastion host
* Customisations to existing parts of CloudFormation - IAM roles/groups

# Plugin Principles

1. Plugin functionality should first be added in places where kube-aws needs it most, i.e. where
kube-aws receives more feature requests which do not fit with the core feature set.
1. kube-aws should dogfood the plugins system, i.e. use the plugins system where possible on existing functionality
1. Everything that is pluggable should be kept as modular as possible, from lists of IAM roles to kube API server parameters.
1. Where possible, checks will be added to kube-aws to ensure the integrity of the cluster via validation or disallowing particular changes. e.g. disallow overriding node pool init metadata
   * Note, this could be gradually added rather than forming part of the first implementation
   * Some of this will likely be provided by CloudFormation but kube-aws could possibly replicas the checks to save time for the stack create/update to fail

# Design

## Plugin Types

* Built-in plugins versus ad-hoc user specific plugins
  * Some plugins will be provided by kube-aws and some will be specific to a user
  * Both will use the same mechanisms to hook into kube-aws/deploy etc
  * Both types of plugins will be enabled and configured in the same way
* Built-in plugins versus contrib plugins
  * kube-aws will accept contrib plugins for features considered to be applicable to many users
  * Contrib plugins will be enabled in a similar way but specifically marked as not fully
  battle tested; they will still be configured in the same way
* Plugins will be allowed to act across the whole cluster or act on a specific set of node pools, this should form part of the base configuration for which plugins to run

## Stages

The workflow for kube-aws should consist of the following stages, each of which will be called on the plugins in turn. Each stage will complete across all plugins prior to proceeding to the next stage. All plugins will receive input based on the whole setup, not just their own. This will allow them to hook into parameters from other plugins or core logic. The stages are not intended to become kube-aws commands and should mostly be transparent to a kube-aws user.

1. initialise
   * Perform any prerequisite setup required such as calls to external system. Configuration, presence of files etc can be validated.
   * Input - configuration
   * Output - configuration with any initialised values, internal kube-aws model with primary constructs for CloudFormation, Kubernetes YAML/JSON, Helm charts, systemd units, kube API server parameters and any other key areas that allow customisation. Some of these will be file references but some will be full kube-aws internal models. The available list of constructs will be added to in further implementations.
1. render
   * Stack files are gathered and rendered locally, most plugins will perform a no-op here but some may choose to output additional CloudFormation stacks or perform additional validation. A core plugin could grab all the primary constructs and create the main CloudFormation or a kube API server plugin could validate none of the parameters it has set are conflicting.
   * Input - output of `initialise` stage
   * Output - CloudFormation file(s), other file reference(s) such as Helm Charts
1. deploy
   * Take the CloudFormation and file(s), upload the files to S3 and deploy the template(s) to CloudFormation. This is an opportunity for plugins to perform waiting tasks and/or actions that need to be done on deploy/install, for example post to an endpoint created during deploy.
   * Input - output of `render` stage, stack name
   * Output - stack details such as stack name, endpoints, certs
1. validate
   * Plugins can check they are functioning as expected which may depend on the configuration
   * Input - output of `deploy` stage, configuration
   * Output - validation errors

Validation errors can also be returned at any stage and can be:
1. fatal, kube-aws will stop processing further and log the problem
1. warning, kube-aws will continue processing but log a warning, can be used for deprecation

The output of the `initialise` stage is similar to `render stack` in kube-aws as of today with templates and a few other files. The output of the `render` stage is the fully formed CloudFormation without templating. Both of these could be stored as the user wishes and potentially version controlled.

## Examples

### Deployment Outside of Kubernetes

An example is a non-containerised monitoring agent or healthcheck. systemd units and other commands 
could be output from plugins during the `initialise` stage and hence get included in the generated cluster.

### Dex / Rescheduler / Add-on Installation

Dex installation requires changes to the kube API server parameters plus a deployment into a running Kubernetes cluster.
kube API server parameters will be one of the constructs available as an output from `initialise` and so can be added to from the Dex plugin.
The deployment can be performed either via Kubernetes YAML or Helm Chart both of which would be a file/directory reference.

### Additional Worker IAM Permissions / kube2iam

Additional worker IAM permissions requires either additions to the existing IAM policies or new IAM policies
being attached to the worker instance role. This could be done in a few ways, for example `initialise` could
output the additional CloudFormation template snippet needed along with name/ref of the IAM policy.
The policies and policy document for the workers would be primarily managed by a core plugin that deals
with the worker permissions. It would take the output of `initialise` and render the final CloudFormation required.

The kube2iam add-on requires Kubernetes YAML or a Helm Chart to be installed which is covered by the Dex
example above plus changes to the worker IAM permissions.

### Bastion Host

A bastion host requires additional CloudFormation to deploy the instance into the correct part of the
VPC. It therefore requires knowledge of the VPC/network setup as input. This plugin would take the 
configuration in `initialise` and output the additional CloudFormation based on the VPC settings.

### CloudFormation Renderer

During the `initialise` stage a core CloudFormation render plugin could output the basic CloudFormation templates for the cluster. Then during the `render` stage the same core plugin could choose to take all the output across all plugins from `initialise` and render the final CloudFormation.

### Kube API Server

A Kubernetes API server plugin could be responsible for main piece of cloud config that sets up the API server. It could then also perform additional validation during the `render` stage to ensure any plugins that have added to the kube API server parameters primary construct have not made the final state invalid.

# Implementation

* [Helm](https://github.com/kubernetes/helm) should be the main method for deploying additional components into the Kubernetes cluster
* How can kube-aws deal with plugin ordering? The ordering could be defined in a fixed YAML file and anything not present is always run last.
* What is the process to include and compile plugins?
   * One idea is to use [go-plugin](https://github.com/hashicorp/go-plugin) since the native 1.8 plugins is not mature yet
* We need to protect sensitive information if uploading files, some Helm charts have secrets as config values so how will these be input?

## Configuration

All plugins will be enabled and configured via the `cluster.yaml` plus a set of plugin specific
configuration files. However, as above, the whole configuration model will be passed to all plugins
so they can access parameters from another plugin if necessary.

## Plugin Locations

* Built-in plugins can either be part of the kube-aws binary or downloadable from a secure source 
if/when the user enables that plugin. The second is recommended.
* Built-in plugins will live under `kubernetes-incubator/kube-aws/plugins/<PLUGIN_NAME>`
  * This location can be thought of as the built-in plugin repository
* User contributed plugins will live under `kubernetes-incubator/kube-aws/contrib/plugins/<PLUGIN_NAME>`
until they are considered core or deprecated.
  * This location can be thought of as the contrib plugin repository kube-aws plugins
* User provided will live under `<USER_DIRECTORY>/plugins/<PLUGIN_NAME>`
  * This location can be thought of as user provided repository
  * A user can specify a set of directories for plugins to be read from, likely as a CLI argument
