# Why Goals
Stating goals (and non-goals) are intended to sharpen the join understanidng the the kube-aws product vision. Expressiong the vision by 'Goals' seem to be understandable approach.

#Goals
Kube-aws is Command Line Tool (CLI) that allow me as experienced console-user to:

* to plan and prepare my kubernetes cluster alongside need AWS resources
* browse through intermediate details of the configuration (of not yet provisioned clusters)

Kube-aws CLI's aims to support different experience levels of the users in terms of 
* kubernetes administration expertise
* CoreOS administration expertise
* Amazone Web Services expertise

to support this goal it
* maintains well-documented set of default settings
* but allows to configure every aspect of the cluster
* enables reusing of the existing AWS resources whenever possible (if and only if those resources conforms to the requirements of kube-aws and/or Kubernetes)
* provides built-in! documentation, that is available online as well

Further more kube-aws allow me to 
* manage several clusters in convenient way
* is capable of query statuses of maintained clusters and e.g. cab list maintained AWS resources
* provides controlled verbosity, warns and guides users 
* CLI command structure is self-explainable

#Non Goals

* Not intended to be used with other cloud providers than Amazon Web Services at the moment.
* No plan to support host OSes other than CoreOS
