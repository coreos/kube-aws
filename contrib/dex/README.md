## Configure Dex as a custom provider in cluster.yaml

Example: 

       oidc:
         enabled: true
         url: "https://dex.example.com"
         clientId: "example-app"
         username: "email"
         groups: "groups"
         selfSignedCa: false


## Deploy Dex

 1. Edit the configMap `contrib/dex/dex.cm.yaml` according to your setup. By default only the GitHub provider and static clients are enabled.
 If you have a different setup, please check [Dex's documentation](https://github.com/coreos/dex/tree/master/Documentation)
 
 2. Create a secret containing your [GitHub OAuth2 client credentials](https://github.com/settings/applications/new)
 
     kubectl create secret \
         generic github-client \
         --from-literal=client-id=$GITHUB_CLIENT_ID \
         --from-literal=client-secret=$GITHUB_CLIENT_SECRET
         
 3. Deploy Dex: `kubectl apply -f contrib/dex/dex.de.yaml`
 
 **Deploy Dex using Helm**
 
 For those who prefer deploying it using helm, Samsung-CNT has a [chart](https://github.com/samsung-cnct/chart-dex) available.
 
## Exposing DEX service
After Dex is deployed, you have to manually expose dex service using a ELB or Ingress.
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

##Configure `kubectl` for token authentication

* `kubectl` config using command line example:


    kubectl config set-credentials admin@example.com  \
    --auth-provider=oidc \   
    --auth-provider-arg=idp-issuer-url=https://dex.example.com \
    --auth-provider-arg=client-id=example-app \
    --auth-provider-arg=client-secret=ZXhhbXBsZS1hcHAtc2VjcmV0 \   
    --auth-provider-arg=refresh-token=refresh_token \   
    --auth-provider-arg=idp-certificate-authority=/etc/kubernetes/ssl/ca.pem \   
    --auth-provider-arg=id-token=id_token \
    --auth-provider-arg=extra-scopes=groups

* `kubectl` config file example:


    apiVersion: v1
    clusters:
    - cluster:
        certificate-authority-data: ca.pem_base64_encoded
        server: https://kubeapi.example.com
      name: your_cluster_name
    contexts:
    - context:
        cluster: your_cluster_name
        user: admin@example.com
      name: your_cluster_name
    current-context: your_cluster_name
    kind: Config
    preferences: {}
    users:
    - name: admin@example.com
      user:
        auth-provider:
          config:
            access-token: id_token
            client-id: example-app 
            client-secret: ZXhhbXBsZS1hcHAtc2VjcmV0
            extra-scopes: groups
            id-token: id_token
            idp-issuer-url: https://dex.example.com
            refresh-token: refresh_token
          name: oidc
          