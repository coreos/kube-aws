#!/bin/bash
# Restore webhooks that were exported and then deleted by upgrade-helper.sh

retries=5
hyperkube_image="{{ .Config.HyperkubeImage.RepoWithTag }}"
webhooks_save_path="/srv/kubernetes"
disable_webhooks="{{ .Values.disableWebhooks }}"
disable_worker_communication_check="{{ .Values.disableWorkerCommunicationChecks }}"

kubectl() {
  /usr/bin/docker run -i --rm -v /etc/kubernetes:/etc/kubernetes:ro -v ${webhooks_save_path}:${webhooks_save_path}:rw --net=host ${hyperkube_image} /hyperkube kubectl --kubeconfig=/etc/kubernetes/kubeconfig/admin.yaml "$@"
}

applyall() {
  kubectl apply --force -f $(echo "$@" | tr ' ' ',')
}

restore_webhooks() {
  local type=$1
  local file=$2

  if [[ -s "${file}.index" ]]; then
    echo "Restoring all ${type} webhooks from ${file}.index"
    hooks=$(cat "${file}.index")
    for h in ${hooks}; do
      echo "restoring ${type} webhook ${h}..."
      exists=$(kubectl get ${type}webhookconfiguration ${h} --no-headers --ignore-not-found)
      if [[ -n "${exists}" ]]; then
        echo "${h} found - not restoring!"
      else
        if [[ -s "${file}.${type}.${h}.yaml" ]]; then
          echo "restoring from ${file}.${type}.${h}.yaml..."
          applyall ${file}.${type}.${h}.yaml
        else
          echo "error! file ${file}.${type}.${h}.yaml not found or is empty"
        fi
      fi
    done
  else
      echo "no webhooks to restore in $file"
  fi
}

if [[ "${disable_webhooks}" == "true" ]]; then
    echo "Restoring all validating and mutating webhooks..."
    restore_webhooks validating ${webhooks_save_path}/validating_webhooks
    restore_webhooks mutating ${webhooks_save_path}/mutating_webhooks
fi

if [[ "${disable_worker_communication_check}" == "true" ]]; then
    echo "Removing the worker communication check from cfn-signal service..."
    cat >/opt/bin/check-worker-communication <<EOT
#!/bin/bash
exit 0
EOT
fi

exit 0