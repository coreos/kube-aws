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

list_not_empty() {
  local file=$1
  if ! [[ -s $file ]]; then
    return 1
  fi
  if cat $file | grep -se 'items: \[\]'; then
    return 1
  fi
  return 0
}

applyall() {
  kubectl apply --force -f $(echo "$@" | tr ' ' ',')
}

restore_webhooks() {
  local type=$1
  local file=$2

  if list_not_empty $file; then
    echo "Restoring all ${type} webhooks from ${file}"
    applyall $file
  else
      echo "no webhooks to restore in $file"
  fi
}

if [[ "${disable_webhooks}" == "true" ]]; then
    echo "Restoring all validating and mutating webhooks..."
    restore_webhooks validating ${webhooks_save_path}/validating_webhooks.yaml
    restore_webhooks mutating ${webhooks_save_path}/mutating_webhooks.yaml
fi

if [[ "${disable_worker_communication_check}" == "true" ]]; then
    echo "Removing the worker communication check from cfn-signal service..."
    cat >/opt/bin/check-worker-communication <<EOT
#!/bin/bash
exit 0
EOT
fi

exit 0