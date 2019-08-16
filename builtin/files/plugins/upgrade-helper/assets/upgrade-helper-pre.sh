#!/bin/bash
# Smooths upgrades/roll-backs where the release of kubernetes jumps a release
# It kills old controllers so that this one takes over all api functions, so we don't get an 
# extended period of old and new running side-by-side and the incompatibilities that this can bring.
# It also removes any mutating and validating webhooks in the system so that install-kube-system can run without interference.
# 
# A request to disable is a configmap matching the hostname and kubernetes version containing a list of core service to stop: -
# apiVersion: v1
# kind: ConfigMap
# metadata:
#   name: kube-aws-migration-disable-ip-10-29-26-83.us-west-2.compute.internal
#   namespace: kube-system
# data:
#   kubernetesVersion: v1.9.3
#   disable: "kube-apiserver kube-controller-manager kube-scheduler"

retries=5
hyperkube_image="{{ .Config.HyperkubeImage.RepoWithTag }}"
my_kubernetes_version="{{ .Config.HyperkubeImage.Tag }}"
myhostname=$(hostname -f)
disable_webhooks="{{ .Values.disableWebhooks }}"
webhooks_save_path="/srv/kubernetes"

kubectl() {
    /usr/bin/docker run -i --rm -v /etc/kubernetes:/etc/kubernetes:ro -v ${webhooks_save_path}:${webhooks_save_path}:rw --net=host ${hyperkube_image} /hyperkube kubectl --kubeconfig=/etc/kubernetes/kubeconfig/admin.yaml "$@"
}

kubectl_with_retries() {
  local tries=0
  local result_text=""
  local return_code=0

  while [ "$tries" -lt "$retries" ]; do
    result_text=$(kubectl "$@")
    return_code=$?
    if [ "$return_code" -eq "0" ]; then
      echo "${result_text}"
      break
    fi
    sleep 10
    tries=$((tries+1))
  done
  return $return_code
}

log() {
  echo "$@" >&2
}

get_masters() {
  kubectl get nodes -l kubernetes.io/role=master --no-headers -o custom-columns=NAME:metadata.name,VERSION:status.nodeInfo.kubeletVersion | awk '{printf "%s:%s\n", $1, $2}'
}

valid_version() {
  match=$(echo $1 | awk -e '(/^v[0-9]+\.[0-9]+\.[0-9]+/){print "match"}')
  [[ "$match" == "match" ]]
}

version_jumps() {
  # only a minor release change is NOT a version jump
  if [[ "${1%.*}" != "${2%.*}" ]]; then
    return 0
  fi
  return 1
}

# stop a controller by writing a special kube-aws disable service configmap
disable_controller() {
  local controller=$1
  local version=$2

  local request="$(cat <<EOT
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-aws-migration-disable-${controller}
  namespace: kube-system
data:
  kubernetesVersion: ${version}
  disable: "kube-controller-manager kube-scheduler kube-apiserver"
EOT
)"

  log "Creating disable service configmap kube-system/kube-aws-migration-disable-${controller}"
  echo "${request}" | kubectl_with_retries -n kube-system apply -f - || return 1
  return 0
}

find_pod() {
  local name=$1
  local host=$2

  kubectl -n kube-system get pod "${name}-${host}" --no-headers -o wide --ignore-not-found
}

node_running() {
  local node=$1

  ready=$(kubectl -n kube-system get node "${node}" --no-headers --ignore-not-found | awk '{print $2}')
  if [[ "${ready}" == "Ready" ]]; then
    return 0
  fi

  return 1
}

wait_stopped_or_timeout() {
  local controllers=$1
  log ""
  log "WAITING FOR ALL MATCHED CONTROLLERS TO STOP:-"
  log "${controllers}"
  log ""
  local max_wait=300
  local wait=0

  local test=1
  while [ "$test" -eq "1" ]; do
    test=0

    for cont in $controllers; do
      if node_running $cont; then
        test=1
      fi
    done

    if [ "$test" -eq "1" ]; then
      if [[ "${wait}" -ge "${max_wait}" ]]; then
        log "Wait for controllers timed out after ${wait} seconds."
        break
      fi
      log "Controllers still active, waiting 5 seconds..."
      wait=$[$wait+5]
      sleep 5
    else
      log "All target controllers are now inactive."
    fi
  done
}

save_webhooks() {
  local type=$1
  local file=$2

  echo "Storing and removing all ${type} webhooks"
  if [[ -s "${file}.index" ]]; then
    echo "${file}.index already saved"
  else
    local hooks=$(kubectl get ${type}webhookconfigurations -o custom-columns=NAME:.metadata.name --no-headers)
    local count=$(echo "${hooks}" | wc -w | sed -e 's/ //g')
    echo "Found ${count} ${type} webhooks..."
    if [[ -n "${hooks}" ]]; then
      echo -n "${hooks}" >${file}.index
      for h in ${hooks}; do
        echo "backing up ${type} webhook ${h}..."
        kubectl get ${type}webhookconfiguration ${h} -o yaml --export >${file}.${type}.${h}.yaml
        echo "deleting $type webhook ${h}..."
        ensuredelete ${file}.${type}.${h}.yaml
      done
    fi
  fi
}

ensuredelete() {
  kubectl delete --cascade=true --ignore-not-found=true -f $(echo "$@" | tr ' ' ',')
}

# MAIN

if ! $(valid_version ${my_kubernetes_version}); then
  log "My kubernetes version ${my_kubernetes_version} is invalid - aborting!"
  exit 1
fi

while ! kubectl get ns kube-system; do
  echo "waiting for apiserver to be available..."
  sleep 3
done

# Disable all mutating and validating webhooks because they can interfere with the stack migration)
if [[ "${disable_webhooks}" == "true" ]]; then
  echo "Storing and removing all validating and mutating webhooks..."
  mkdir -p ${webhooks_save_path}
  save_webhooks validating ${webhooks_save_path}/validating_webhooks
  save_webhooks mutating ${webhooks_save_path}/mutating_webhooks
fi

log ""
log "CHECKING CONTROLLER VERSIONS..."
log ""
found=""
for controller in $(get_masters); do
  controller_name=$(echo "${controller%%:*}")
  controller_version=$(echo "${controller##*:}")
  if [[ "${controller_name}" != "$myhostname" ]]; then
    if ! $(valid_version ${controller_version}); then
      log "Controller ${controller_name} has an invalid version number ${controller_version}!"
      continue
    fi

    if $(version_jumps ${my_kubernetes_version} ${controller_version}); then
      log "Detected a version jump on ${controller_name}: my version is ${my_kubernetes_version} and theirs is ${controller_version}"
      log "Disabling kube-apiserver, kube-scheduler and kube-controller-manager..."
      if [[ -z "${found}" ]]; then
        found="${controller_name}"
      else
        found="${found} ${controller_name}"
      fi
      disable_controller ${controller_name} ${controller_version}
    else
      log "No version jump on ${controller_name}: my version is ${my_kubernetes_version} and theirs is ${controller_version}"
    fi
  fi
done

if [[ -n "${found}" ]]; then
    log ""
    log "WAITING FOR FOUND CONTROLLERS TO STOP..."
    log ""
    wait_stopped_or_timeout "${found}"
fi
exit 0