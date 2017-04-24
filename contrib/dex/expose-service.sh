#!/usr/bin/env bash

RED='\033[0;31m'
GREEN='\033[0;32m'
ORANGE='\033[0;33m'
BLUE='\033[0;34m'

PS3='Please select the method to expose the service for dex:'
options=("PublicELB" "PrivateELB" "Ingress" "Quit")
select opt in "${options[@]}"
do
    case $opt in
        "PublicELB")
            echo "Creating Public LaodBalancer"
            echo -e ${BLUE}
            read -r -p "Please replace the default values for the 'domain' and 'arn' in 'elb/external-elb.yaml'.Done? [y/N]" response
            tput sgr0
            if [[ $response =~ ^([yY][eE][sS]|[yY])$ ]]
            then
                kubectl apply -f ./elb/external-elb.yaml
            fi
            #get ELB hostname
            while
            ELB=$(kubectl get svc dex -o jsonpath='{.status.loadBalancer.ingress[*].hostname}' -n kube-system);
            :; grep -v elb.amazonaws.com <<<$ELB; do echo "Waiting for the ELB to become available"; sleep 5; done
            DOMAIN=$(kubectl get svc dex -o jsonpath='{.metadata.annotations.domainName}' -n kube-system)
            echo -e "Done. Please create a CNAME record or ALIAS with this value:"
            echo -e "${BLUE}$DOMAIN > $ELB"

            tput sgr0
            break
            ;;

        "PrivateELB")
            echo "Creating Public LaodBalancer"
            echo -e ${BLUE}
            read -r -p "Please replace the default values for the 'domain' and 'arn' in 'elb/external-elb.yaml'.Done? [y/N]" response
            tput sgr0
            if [[ $response =~ ^([yY][eE][sS]|[yY])$ ]]
            then
                kubectl apply -f ./elb/internal-elb.yaml
            fi
            #get ELB hostname
            while
            ELB=$(kubectl get svc dex -o jsonpath='{.status.loadBalancer.ingress[*].hostname}' -n kube-system);
            :; grep -v elb.amazonaws.com <<<$ELB; do echo "Waiting for the ELB to become available"; sleep 5; done
            DOMAIN=$(kubectl get svc dex -o jsonpath='{.metadata.annotations.domainName}' -n kube-system)
            echo -e "Please create a CNAME record or ALIAS with this value:"
            echo -e "${BLUE}$DOMAIN > $ELB"
            tput sgr0
            break
            ;;

        "Ingress")
            echo "Creating Ingress"
            prompt="Please insert the path for your 'credentials' directory:"
            while IFS= read -p "$prompt" -r -s -n 1 char
            do
                   if [[ $char == $'\0' ]]
                   then
                        break
                   fi
            prompt=$char
            credentials_path+="$char"
            done
            echo

            prompt="Please set the enpoint for dex:"
            while IFS= read -p "$prompt" -r -s -n 1 char
            do
                   if [[ $char == $'\0' ]]
                   then
                        break
                   fi
            prompt=$char
            endpoint+="$char"
            done
            echo

            sed -i -e 's/dex.example.com/'"$endpoint"'/g' ingress/dex.ingress.yaml

            echo "Creating TLS secret"
            kubectl create secret tls dex-tls-secret --cert=$credentials_path/dex.pem --key=$credentials_path/dex-key.pem -n kube-system

            echo " Creating Ingress"
            kubectl apply -f ./ingress;

            echo -e ${RED}"Waiting 10 seconds for the Ingress Controller to become available."
            tput sgr0

            sleep 10

            echo " Please create a DNS record for Ingress"

            echo
            kubectl get ing -o wide -n kube-system

            break
            ;;
        "Quit")
            break
            ;;
        *) echo invalid option;;
    esac
done


