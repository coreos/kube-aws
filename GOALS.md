# Why Goals
Stating goals (and non-goals) are intended to sharpen the join understanidng the the kube-aws product vision. Expressiong the vision by 'Goals' seem to be understandable approach.

#Goals
Kube-aws is Command Line Tool (CLI) that allow me as experienced console-user to:

* to plan ant prepare my kubernetes cluster alongside need AWS resources
* browse throgh intermediate details of the configuration (of not yet provisioned clusters)

Kube-aws CLI's goal is to support different experience levels of the users in terms of 
* kubernetes administration expertise
* CoreOS administration expertise
* Amazone Web Services expertise

to support this goal it 
    * maintanes well-documented set of default settings
    * but allows to configure every aspect of the cluster
    * enables reuse of (all) custom predefined AWS resources that are precreated in AWS
    * provides build in! documentation, that is available online as well

Further more kube-aws allow me to 
* manages several clusters in convenient way
* is capable of query statuses of maintaned clusters and e.g. list out maintaned AWS resources
* provides controlled verbosity, warns and guides users 
* CLI command structure is self-explainable

#Non Goals

* Not intended to be used with other cloud providers than Amazon Web Services at the moment.
