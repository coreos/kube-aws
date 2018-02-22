#!/bin/bash
set -ue

err() {
    (>&2 echo "${1} Exiting.")
    exit 1
}

main_help() {
  cat >&2 <<EOF
Usage: BUCKET_URI [ NAMESPACE1 NAMESPACE2 ... ]

Arguments:
    BUCKET_URI                      The S3 uri of the timestamped backup bucket.  Example: 's3://bucket-name/cf/cluster-name/clusterBackup/2017-04-07_09-03-11'
    [ NAMESPACE1 NAMESPACE2 ... ]   (Optional) Namespaces to create.              Example: 'alpha beta'

Description:
    Import Kubernetes resources to a cluster from a previous clusterBackup in S3.
    All namespaces (and associated resources) are restored by default with the exception of namespaces: 'kube-system' and 'default'.
    To whitelist namespaces, include the desired namespace names as additional arguments after the BUCKET_URI argument.

    Please monitor the restoration process to ensure the Kubernetes resources restored correctly.

Assumptions:
    - 'kubectl' installed locally
    - 'aws' installed locally
    - 'jq' installed locally
    - 'kubeconfig' configured correctly
    - 'jq' configured correctly
    - cluster is reachable

EOF

    exit 0
}

defaultCreate () { kubectl create -f ${1} || : ;}

createResource() {
    RESOURCE="${1}"
    NAMESPACE=""
    FILEPATH="${TEMP_DIR}/${RESOURCE}.json"
    if [[ ! -z ${2:-} ]] ; then
        NAMESPACE="${2}"
        FILEPATH="${TEMP_DIR}/${NAMESPACE}/${RESOURCE}.json"
    fi
    if [[ -s "${FILEPATH}" ]] ; then
        createFunc=${RESOURCE}Create
        type -t $createFunc >/dev/null || createFunc=defaultCreate
        $createFunc "${FILEPATH}" "${NAMESPACE}"
    fi
}

doesResourceExist() {
    TYPE="${1}"
    NAME="${2}"
    if kubectl get ${TYPE} ${NAME} >&/dev/null ; then
        return 1
    fi
    return 0
}

preFlightChecks() {
    [[ "${1}" != "--help" ]] || main_help
    which kubectl >/dev/null || err "'kubectl' not found."
    which aws >/dev/null || err "'aws' not found."
    which jq >/dev/null || err "'jq' not found."
    [[ "${1}" == s3://* ]] || err "Invalid S3 bucket URL syntax - expecting \"s3://*\"."
    aws s3 ls ${1} >/dev/null || err "Cannot not validate the S3 bucket \"${1}\"."
    kubectl get nodes >/dev/null || err "Cannot establish kubectl connection."
}

## Custom functions - For resources that require more than just 'kubectl create'
persistentvolumeclaimsCreate() { # PersistentVolumeClaims should be status=Bound before creating resources that may use them
    FILEPATH="${1}";
    NAMESPACE="${2}";
    kubectl create -f ${FILEPATH} || :

    # Validate bound status
    SUCCESS=0
    for i in {1..5} ; do
        BOUND=1
        kubectl get pvc -o json -n ${NAMESPACE} | jq -r '.items[].status.phase' \
        | grep -qv Bound || { SUCCESS=1; break; }
        sleep 1
    done
    if [ "${SUCCESS}" != 1 ] ; then
        echo "WARNING: PersistentVolumeClaims failed to achieve 'status=Bound': '$(kubectl describe pvc -n ${NAMESPACE})'"
    fi
}

main() {
    # Define the resources to restore in order.
    RESTORATION_ORDER=( storageclasses persistentvolumes ) # clusterrolebindings clusterroles
    RESTORATION_ORDER_NS=( persistentvolumeclaims configmaps endpoints ingresses jobs limitranges networkpolicies
                        podsecuritypolicies podtemplates resourcequotas secrets serviceaccounts services thirdpartyresources
                        horizontalpodautoscalers pods replicasets replicationcontrollers daemonsets deployments statefulsets
                        poddisruptionbudgets roles rolebindings)

    preFlightChecks "${@}"
    AWS_BUCKET="${1}"; shift

    echo "Downloading S3 bucket..."
    aws s3 sync ${AWS_BUCKET} ${TEMP_DIR} >/dev/null || err "Could not retrieve the backup bucket from aws!"

    if [ ! -f ${TEMP_DIR}/namespaces.json ]; then
        err "Could not find a namespaces.json file - are you sure a valid back-up bucket exists?"
    fi

    # Compile a list of namespaces to create/populate
    echo "Evaluating namespaces..."
    NAMESPACE_WHITELIST="${@:-}"
    NAMESPACES=""
    if [ -z ${NAMESPACE_WHITELIST} ]; then
        for ns in $(jq -r '.items[].metadata.name' < ${TEMP_DIR}/namespaces.json); do
            if [[ ${ns} != "kube-system" && ${ns} != "default" ]] ; then
                doesResourceExist "namespace" "${ns}" || err "Namespace '${ns}' already exists in the cluster!"
                NAMESPACES="${NAMESPACES} ${ns}"
            fi
        done
    else
        echo "Limiting Namespaces to '${NAMESPACE_WHITELIST}'..."
        for nswl in ${NAMESPACE_WHITELIST}; do
            for ns in $(jq -r '.items[].metadata.name' < ${TEMP_DIR}/namespaces.json); do
                if [ "${ns}" == "${nswl}" ] ; then
                    doesResourceExist "namespace" "${ns}" || echo "Namespace '${ns}' already exists in the cluster! Restoration will still proceed..."
                    NAMESPACES="${NAMESPACES} ${ns}"
                    break
                fi
            done
        done
    fi

    echo "Restoring resources that are not within a Namespace..."
    # Create the Kubernetes resources that reside outside of namespaces
    for r in ${RESTORATION_ORDER[@]}; do
        createResource "${r}"
    done

    # Iterate through namespaces and create the Kubernetes resources that reside inside of namespaces
    for ns in ${NAMESPACES}; do
        if [ -e "${TEMP_DIR}/${ns}" ] && [ -d "${TEMP_DIR}/${ns}" ]; then
        echo "Restoring resources for '${ns}' Namespace..."
        kubectl create -f <(echo $(cat ${TEMP_DIR}/namespaces.json | jq -r --arg NS "$ns" '.items[] | select(.metadata.name == ($NS))'))
            for r in ${RESTORATION_ORDER_NS[@]}; do
                createResource "${r}" "${ns}"
            done
        else
            echo "Could not find a backup directory for namespace '${ns}'"
        fi
    done

    echo "Restoration finished."
}


### START ###

TEMP_DIR=$(mktemp -d)
trap "rm -f -r ${TEMP_DIR}" EXIT

main "${@:-}"

