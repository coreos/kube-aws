# Exposing DEX service
After the cluster is up, you have to manually expose dex service using a ELB or Ingress.
Faster is to use the `expose-service.sh` script or you can manually configure the services using the examples from `contrib/dex` directory.

1. ELB

First option is to use a Public or Internal ELB.

In this case you have to edit one of the files from `contrib/dex/elb` directory and set your certificate `arn` and `domainName`.

Note: 
* SSL/TLS certificates provisioned through AWS Certificate Manager are free. 
You pay only for the AWS resources you create to run your application. This is the recommended method.

2. Ingress

After deploying the Ingress you have to configure the workers security group to allow access on port 443 and optionally on port 80.
Please note that if you plan to restrict access of these ports to your IP, you also have to allow access from controller nodes, as the API server will access the dex endpoint.

