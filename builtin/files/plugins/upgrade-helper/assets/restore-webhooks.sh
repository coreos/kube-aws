#!/bin/bash
# Restore webhooks that were exported and then deleted by upgrade-helper.sh

retries=5
hyperkube_image="{{ .Config.HyperkubeImage.RepoWithTag }}"
disable_webhooks="{{ if .Values.disableWebhooks }}true{{else}}false{{end}}"

kubectl() {
  /usr/bin/docker run -i --rm -v /etc/kubernetes:/etc/kubernetes:ro --net=host ${hyperkube_image} /hyperkube kubectl --kubeconfig=/etc/kubernetes/kubeconfig/admin.yaml "$@"
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
    restore_webhooks validating /srv/kubernetes/validating_webhooks.yaml
    restore_webhooks mutating /srv/kubernetes/mutating_webhooks.yaml
fi
exit 0