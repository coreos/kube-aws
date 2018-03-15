package config

var CloudConfigController = []byte(`{{ define "instance" -}}
{{- $S3URI := self.Parts.s3.Asset.S3URL -}}
#!/bin/bash -xe
 . /etc/environment
export COREOS_PRIVATE_IPV4 COREOS_PRIVATE_IPV6 COREOS_PUBLIC_IPV4 COREOS_PUBLIC_IPV6
REGION=$(curl -s http://169.254.169.254/latest/dynamic/instance-identity/document | jq -r '.region')
USERDATA_FILE=userdata-controller

run() {
  bin="$1"; shift
  while ! /usr/bin/rkt run \
          --net=host \
          --volume=dns,kind=host,source=/etc/resolv.conf,readOnly=true --mount volume=dns,target=/etc/resolv.conf  \
          --volume=awsenv,kind=host,source=/var/run/coreos,readOnly=false --mount volume=awsenv,target=/var/run/coreos \
          --trust-keys-from-https \
          {{.AWSCliImage.Options}}{{.AWSCliImage.RktRepo}} --exec=$bin -- "$@"; do
    sleep 1
  done
}

run bash -c "aws configure set s3.signature_version s3v4; aws s3 --region $REGION cp {{$S3URI}} /var/run/coreos/$USERDATA_FILE"

exec /usr/bin/coreos-cloudinit --from-file /var/run/coreos/$USERDATA_FILE
{{ end }}
{{ define "s3" -}}
#cloud-config
coreos:
  update:
    reboot-strategy: "off"
  flannel:
    interface: $private_ipv4
    etcd_cafile: /etc/kubernetes/ssl/etcd-trusted-ca.pem
    etcd_certfile: /etc/kubernetes/ssl/etcd-client.pem
    etcd_keyfile: /etc/kubernetes/ssl/etcd-client-key.pem

  units:
{{- range $u := .Controller.CustomSystemdUnits}}
    - name: {{$u.Name}}
      {{- if $u.Command }}
      command: {{ $u.Command }}
      {{- end}}
      {{- if $u.Enable }}
      enable: {{ $u.Enable }}
      {{- end }}
      {{- if $u.Runtime }}
      runtime: {{ $u.Runtime }}
      {{- end }}
      {{- if $u.DropInsPresent }}
      drop-ins:
        {{- range $d := $u.DropIns }}
        - name: {{ $d.Name }}
          content: |
            {{- range $i := $d.ContentArray }}
            {{ $i }}
            {{- end}}
        {{- end }}
      {{- end}}
      {{- if $u.ContentPresent }}
      content: |
        {{- range $l := $u.ContentArray}}
        {{ $l }}
        {{- end }}
      {{- end }}
{{- end}}
    - name: systemd-modules-load.service
      command: restart
{{if and (.AmazonSsmAgent.Enabled) (ne .AmazonSsmAgent.DownloadUrl "")}}
    - name: amazon-ssm-agent.service
      command: start
      enable: true
      content: |
        [Unit]
        Description=amazon-ssm-agent
        Requires=network-online.target
        After=network-online.target

        [Service]
        Type=simple
        ExecStartPre=/opt/ssm/bin/install-ssm-agent.sh
        ExecStart=/opt/ssm/bin/amazon-ssm-agent
        KillMode=controll-group
        Restart=on-failure
        RestartSec=1min

        [Install]
        WantedBy=network-online.target
{{end}}
{{if .CloudWatchLogging.Enabled}}
    - name: journald-cloudwatch-logs.service
      command: start
      content: |
        [Unit]
        Description=Docker run journald-cloudwatch-logs to send journald logs to CloudWatch
        Requires=network-online.target
        After=network-online.target

        [Service]
        ExecStartPre=-/usr/bin/mkdir -p /var/journald-cloudwatch-logs
        ExecStart=/usr/bin/rkt run \
                  --insecure-options=image \
                  --volume resolv,kind=host,source=/etc/resolv.conf,readOnly=true \
                  --mount volume=resolv,target=/etc/resolv.conf \
                  --volume journald-cloudwatch-logs,kind=host,source=/var/journald-cloudwatch-logs \
                  --mount volume=journald-cloudwatch-logs,target=/var/journald-cloudwatch-logs \
                  --volume journal,kind=host,source=/var/log/journal,readOnly=true \
                  --mount volume=journal,target=/var/log/journal \
                  --volume machine-id,kind=host,source=/etc/machine-id,readOnly=true \
                  --mount volume=machine-id,target=/etc/machine-id \
                  --uuid-file-save=/var/journald-cloudwatch-logs/journald-cloudwatch-logs.uuid \
                  {{ .JournaldCloudWatchLogsImage.RktRepo }} -- {{.ClusterName}}
        ExecStopPost=/usr/bin/rkt rm --uuid-file=/var/journald-cloudwatch-logs/journald-cloudwatch-logs.uuid
        Restart=always
        RestartSec=60s

        [Install]
        WantedBy=multi-user.target
{{end}}
    - name: cfn-etcd-environment.service
      enable: true
      command: start
      runtime: true
      content: |
        [Unit]
        Description=Fetches etcd static IP addresses list from CF
        After=network-online.target

        [Service]
        Restart=on-failure
        RemainAfterExit=true
        ExecStartPre=/opt/bin/cfn-etcd-environment
        ExecStart=/usr/bin/mv -f /var/run/coreos/etcd-environment /etc/etcd-environment

{{if .Experimental.AwsEnvironment.Enabled}}
    - name: set-aws-environment.service
      enable: true
      command: start
      runtime: true
      content: |
        [Unit]
        Description=Set AWS environment variables in /etc/aws-environment
        After=network-online.target

        [Service]
        Type=oneshot
        RemainAfterExit=true
        ExecStartPre=/bin/touch /etc/aws-environment
        ExecStart=/opt/bin/set-aws-environment
{{end}}
    - name: docker.service
      drop-ins:
{{if .Experimental.EphemeralImageStorage.Enabled}}
        - name: 10-docker-mount.conf
          content: |
            [Unit]
            After=var-lib-docker.mount
            Wants=var-lib-docker.mount
{{end}}
        - name: 10-post-start-check.conf
          content: |
            [Service]
            RestartSec=10
            ExecStartPost=/usr/bin/docker pull {{.PauseImage.RepoWithTag}}

        - name: 40-flannel.conf
          content: |
            [Unit]
            Wants=flanneld.service
            [Service]
            EnvironmentFile=/etc/kubernetes/cni/docker_opts_cni.env
            ExecStartPre=/usr/bin/systemctl is-active flanneld.service

        - name: 60-logfilelimit.conf
          content: |
            [Service]
            Environment="DOCKER_OPTS=--log-opt max-size=50m --log-opt max-file=3"

    - name: flanneld.service
      drop-ins:
        - name: 10-etcd.conf
          content: |
            [Unit]
            Wants=cfn-etcd-environment.service
            After=cfn-etcd-environment.service

            [Service]
            EnvironmentFile=-/etc/etcd-environment
            Environment="ETCD_SSL_DIR=/etc/kubernetes/ssl"
            EnvironmentFile=-/run/flannel/etcd-endpoints.opts
            ExecStartPre=/usr/bin/systemctl is-active cfn-etcd-environment.service
            ExecStartPre=/bin/sh -ec "echo FLANNELD_ETCD_ENDPOINTS=${ETCD_ENDPOINTS} >/run/flannel/etcd-endpoints.opts"
            {{- if .AssetsEncryptionEnabled }}
            ExecStartPre=/opt/bin/decrypt-assets
            {{- end}}
            ExecStartPre=/usr/bin/etcdctl \
            --ca-file=/etc/kubernetes/ssl/etcd-trusted-ca.pem \
            --cert-file=/etc/kubernetes/ssl/etcd-client.pem \
            --key-file=/etc/kubernetes/ssl/etcd-client-key.pem \
            --endpoints="${ETCD_ENDPOINTS}" \
            set /coreos.com/network/config '{"Network" : "{{.PodCIDR}}", "Backend" : {"Type" : "vxlan"}}'
            TimeoutStartSec=120

{{if .FlannelImage.RktPullDocker}}
        - name: 20-flannel-custom-image.conf
          content: |
            [Unit]
            PartOf=flanneld.service
            Before=docker.service

            [Service]
            Environment="FLANNEL_IMAGE={{.FlannelImage.RktRepo}}"
            Environment="RKT_RUN_ARGS={{.FlannelImage.Options}}"

    - name: flannel-docker-opts.service
      drop-ins:
        - name: 10-flannel-docker-options.conf
          content: |
            [Unit]
            PartOf=flanneld.service
            Before=docker.service

            [Service]
            Environment="FLANNEL_IMAGE={{.FlannelImage.RktRepo}}"
            Environment="RKT_RUN_ARGS={{.FlannelImage.Options}} --uuid-file-save=/var/lib/coreos/flannel-wrapper2.uuid"
{{end}}
    - name: kubelet.service
      command: start
      runtime: true
      content: |
        [Unit]
        Wants=flanneld.service cfn-etcd-environment.service
        After=cfn-etcd-environment.service
        [Service]
        # EnvironmentFile=/etc/environment allows the reading of COREOS_PRIVATE_IPV4
        EnvironmentFile=/etc/environment
        EnvironmentFile=-/etc/etcd-environment
        Environment=KUBELET_IMAGE_TAG={{.K8sVer}}
        Environment=KUBELET_IMAGE_URL={{ .HyperkubeImage.RktRepoWithoutTag }}
        Environment="RKT_RUN_ARGS=--volume dns,kind=host,source=/etc/resolv.conf {{.HyperkubeImage.Options}}\
        --set-env=ETCD_CA_CERT_FILE=/etc/kubernetes/ssl/etcd-trusted-ca.pem \
        --set-env=ETCD_CERT_FILE=/etc/kubernetes/ssl/etcd-client.pem \
        --set-env=ETCD_KEY_FILE=/etc/kubernetes/ssl/etcd-client-key.pem \
        --mount volume=dns,target=/etc/resolv.conf \
        {{ if eq .ContainerRuntime "rkt" -}}
        --volume rkt,kind=host,source=/opt/bin/host-rkt \
        --mount volume=rkt,target=/usr/bin/rkt \
        --volume var-lib-rkt,kind=host,source=/var/lib/rkt \
        --mount volume=var-lib-rkt,target=/var/lib/rkt \
        --volume stage,kind=host,source=/tmp \
        --mount volume=stage,target=/tmp \
        {{ end -}}
        --volume var-lib-cni,kind=host,source=/var/lib/cni \
        --mount volume=var-lib-cni,target=/var/lib/cni \
        --volume var-log,kind=host,source=/var/log \
        --mount volume=var-log,target=/var/log{{ if .UseCalico }} \
        --volume cni-bin,kind=host,source=/opt/cni/bin \
        --mount volume=cni-bin,target=/opt/cni/bin{{ end }} \
        --volume etc-kubernetes,kind=host,source=/etc/kubernetes \
        --mount volume=etc-kubernetes,target=/etc/kubernetes"
        ExecStartPre=/usr/bin/systemctl is-active flanneld.service
        ExecStartPre=/usr/bin/systemctl is-active cfn-etcd-environment.service
        ExecStartPre=/usr/bin/mkdir -p /var/lib/cni
        ExecStartPre=/usr/bin/mkdir -p /var/log/containers
        ExecStartPre=/usr/bin/mkdir -p /opt/cni/bin
        ExecStartPre=/usr/bin/mkdir -p /etc/kubernetes/manifests
        ExecStartPre=/usr/bin/etcdctl \
                       --ca-file /etc/kubernetes/ssl/etcd-trusted-ca.pem \
                       --key-file /etc/kubernetes/ssl/etcd-client-key.pem \
                       --cert-file /etc/kubernetes/ssl/etcd-client.pem \
                       --endpoints "${ETCD_ENDPOINTS}" \
                       cluster-health

        ExecStartPre=/bin/sh -ec "find /etc/kubernetes/manifests /srv/kubernetes/manifests  -maxdepth 1 -type f | xargs --no-run-if-empty sed -i 's|#ETCD_ENDPOINTS#|${ETCD_ENDPOINTS}|'"
        {{if .UseCalico -}}
        ExecStartPre=/bin/sh -ec "find /etc/kubernetes/cni/net.d/ -maxdepth 1 -type f | xargs --no-run-if-empty sed -i 's|#ETCD_ENDPOINTS#|${ETCD_ENDPOINTS}|'"
        ExecStartPre=/usr/bin/docker run --rm -e SLEEP=false -e KUBERNETES_SERVICE_HOST= -e KUBERNETES_SERVICE_PORT= -v /opt/cni/bin:/host/opt/cni/bin {{ .CalicoCniImage.RepoWithTag }} /install-cni.sh
        {{end -}}
        ExecStart=/usr/lib/coreos/kubelet-wrapper \
        --kubeconfig=/etc/kubernetes/kubeconfig/controller.yaml \
        --require-kubeconfig \
        --cni-conf-dir=/etc/kubernetes/cni/net.d \
        {{/* Work-around until https://github.com/kubernetes/kubernetes/issues/43967 is fixed via https://github.com/kubernetes/kubernetes/pull/43995 */ -}}
        --cni-bin-dir=/opt/cni/bin \
        --network-plugin={{.K8sNetworkPlugin}} \
        --container-runtime={{.ContainerRuntime}} \
        --rkt-path=/usr/bin/rkt \
        --rkt-stage1-image=coreos.com/rkt/stage1-coreos \
        --node-labels node-role.kubernetes.io/master{{if .NodeLabels.Enabled}},{{.NodeLabels.String}} \
        {{end}} \
        --register-with-taints=node.alpha.kubernetes.io/role=master:NoSchedule \
        --allow-privileged=true \
        --pod-manifest-path=/etc/kubernetes/manifests \
        {{ if .KubeDns.NodeLocalResolver }}--cluster-dns=${COREOS_PRIVATE_IPV4} \
        {{ else }}--cluster-dns={{.DNSServiceIP}} \
        {{ end }}--cluster-domain=cluster.local \
        --cloud-provider=aws \
        {{if .Experimental.Admission.Priority.Enabled}}
        --feature-gates=PodPriority=true \
        {{end}}\
        $KUBELET_OPTS
        Restart=always
        RestartSec=10

        [Install]
        WantedBy=multi-user.target

{{ if eq .ContainerRuntime "rkt" }}
    - name: rkt-api.service
      enable: true
      content: |
        [Unit]
        Before=kubelet.service
        [Service]
        ExecStart=/usr/bin/rkt api-service
        Restart=always
        RestartSec=10
        [Install]
        RequiredBy=kubelet.service

    - name: load-rkt-stage1.service
      enable: true
      content: |
        [Unit]
        Description=Load rkt stage1 images
        Documentation=http://github.com/coreos/rkt
        Requires=network-online.target
        After=network-online.target
        Before=rkt-api.service
        [Service]
        Type=oneshot
        RemainAfterExit=yes
        ExecStart=/usr/bin/rkt fetch /usr/lib/rkt/stage1-images/stage1-coreos.aci /usr/lib/rkt/stage1-images/stage1-fly.aci  --insecure-options=image
        [Install]
        RequiredBy=rkt-api.service
{{ end }}

    - name: install-kube-system.service
      command: start
      runtime: true
      content: |
        [Unit]
        Wants=kubelet.service docker.service

        [Service]
        Type=oneshot
        StartLimitInterval=0
        RemainAfterExit=true
        ExecStartPre=/usr/bin/bash -c "until /usr/bin/systemctl is-active kubelet.service; do echo waiting until kubelet starts; sleep 10; done"
        ExecStartPre=/usr/bin/bash -c "until /usr/bin/systemctl is-active docker.service; do echo waiting until docker starts; sleep 10; done"
        ExecStartPre=/usr/bin/bash -c "until /usr/bin/curl -s -f http://127.0.0.1:8080/version; do echo waiting until apiserver starts; sleep 10; done"
        ExecStart=/opt/bin/retry 3 /opt/bin/install-kube-system

    - name: apply-kube-aws-plugins.service
      command: start
      runtime: true
      content: |
        [Unit]
        Requires=install-kube-system.service
        After=install-kube-system.service

        [Service]
        Type=oneshot
        StartLimitInterval=0
        RemainAfterExit=true
        ExecStart=/opt/bin/retry 3 /opt/bin/apply-kube-aws-plugins

{{ if $.ElasticFileSystemID }}
    - name: rpc-statd.service
      command: start
      enable: true
    - name: efs.service
      command: start
      content: |
        [Unit]
        After=network-online.target
        Before=kubelet.service
        [Service]
        Type=oneshot
        ExecStartPre=-/usr/bin/mkdir -p /efs
        ExecStart=/bin/sh -c 'grep -qs /efs /proc/mounts || /usr/bin/mount -t nfs4 -o nfsvers=4.1,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2 $(/usr/bin/curl -s http://169.254.169.254/latest/meta-data/placement/availability-zone).{{ $.ElasticFileSystemID }}.efs.{{ $.Region }}.amazonaws.com:/ /efs'
        ExecStop=/usr/bin/umount /efs
        RemainAfterExit=yes
        [Install]
        WantedBy=kubelet.service
{{ end }}
{{if .WaitSignal.Enabled}}
    - name: cfn-signal.service
      command: start
      content: |
        [Unit]
        Wants=kubelet.service docker.service install-kube-system.service apply-kube-aws-plugins.service
        After=kubelet.service install-kube-system.service apply-kube-aws-plugins.service

        [Service]
        Type=simple
        Restart=on-failure
        RestartSec=60
        StartLimitInterval=640
        StartLimitBurst=10
        ExecStartPre=/usr/bin/systemctl is-active install-kube-system.service
        ExecStartPre=/usr/bin/systemctl is-active apply-kube-aws-plugins.service
        ExecStartPre=/usr/bin/bash -c "while sleep 1; do if /usr/bin/curl -s -m 20 -f  http://127.0.0.1:8080/healthz > /dev/null &&  /usr/bin/curl -s -m 20 -f  http://127.0.0.1:10252/healthz > /dev/null && /usr/bin/curl -s -m 20 -f  http://127.0.0.1:10251/healthz > /dev/null &&  /usr/bin/curl --insecure -s -m 20 -f  https://127.0.0.1:10250/healthz > /dev/null && /usr/bin/curl -s -m 20 -f http://127.0.0.1:10256/healthz > /dev/null; then break ; fi;  done"
        {{ if .UseCalico }}
        ExecStartPre=/usr/bin/bash -c "until /usr/bin/docker run --net=host --pid=host --rm {{ .CalicoCtlImage.RepoWithTag }} node status > /dev/null; do sleep 3; done && echo Calico running"
        {{ end }}
        {{if .Experimental.AuditLog.Enabled -}}
        ExecStartPre=/opt/bin/check-worker-communication
        {{end -}}
        ExecStart=/opt/bin/cfn-signal
{{end}}
{{if .Experimental.AwsNodeLabels.Enabled }}
    - name: kube-node-label.service
      enable: true
      command: start
      runtime: true
      content: |
        [Unit]
        Description=Label this kubernetes node with additional AWS parameters
        Wants=kubelet.service
        After=kubelet.service
        Before=cfn-signal.service

        [Service]
        Type=oneshot
        ExecStop=/bin/true
        RemainAfterExit=true
        ExecStart=/opt/bin/kube-node-label
{{end}}

{{if .Experimental.EphemeralImageStorage.Enabled}}
    - name: format-ephemeral.service
      command: start
      content: |
        [Unit]
        Description=Formats the ephemeral drive
        ConditionFirstBoot=yes
        After=dev-{{.Experimental.EphemeralImageStorage.Disk}}.device
        Requires=dev-{{.Experimental.EphemeralImageStorage.Disk}}.device
        [Service]
        Type=oneshot
        RemainAfterExit=yes
        ExecStart=/usr/sbin/wipefs -f /dev/{{.Experimental.EphemeralImageStorage.Disk}}
        ExecStart=/usr/sbin/mkfs.{{.Experimental.EphemeralImageStorage.Filesystem}} -f /dev/{{.Experimental.EphemeralImageStorage.Disk}}
    - name: var-lib-docker.mount
      command: start
      content: |
        [Unit]
        Description=Mount ephemeral to /var/lib/docker
        Requires=format-ephemeral.service
        After=format-ephemeral.service
        [Mount]
        What=/dev/{{.Experimental.EphemeralImageStorage.Disk}}
{{if eq .ContainerRuntime "docker"}}
        Where=/var/lib/docker
{{else if eq .ContainerRuntime "rkt"}}
        Where=/var/lib/rkt
{{end}}
        Type={{.Experimental.EphemeralImageStorage.Filesystem}}
{{end}}
{{ if .SharedPersistentVolume }}
    - name: load-efs-pv.service
      command: start
      content: |
        [Unit]
        Description=Load efs persistent volume mount
        Wants=kubelet.service
        After=kubelet.service
        Before=cfn-signal.service
        [Service]
        Type=simple
        RemainAfterExit=true
        RestartSec=10
        Restart=on-failure
        ExecStartPre=/opt/bin/set-efs-pv
        ExecStart=/opt/bin/load-efs-pv
{{end}}

{{if .SSHAuthorizedKeys}}
ssh_authorized_keys:
  {{range $sshkey := .SSHAuthorizedKeys}}
  - {{$sshkey}}
  {{end}}
{{end}}

{{if .Region.IsChina}}
    - name: pause-amd64.service
      enable: true
      command: start
      runtime: true
      content: |
        [Unit]
        Description=Pull and tag a mirror image for pause-amd64
        Wants=docker.service
        After=docker.service

        [Service]
        Restart=on-failure
        RemainAfterExit=true
        ExecStartPre=/usr/bin/systemctl is-active docker.service
        ExecStartPre=/usr/bin/docker pull {{.PauseImage.RepoWithTag}}
        ExecStart=/usr/bin/docker tag {{.PauseImage.RepoWithTag}} gcr.io/google_containers/pause-amd64:3.0
        ExecStop=/bin/true
        [Install]
        WantedBy=install-kube-system.service
{{end}}
write_files:
  - path: /etc/ssh/sshd_config
    permissions: 0600
    owner: root:root
    content: |
      UsePrivilegeSeparation sandbox
      Subsystem sftp internal-sftp
      ClientAliveInterval 180
      UseDNS no
      UsePAM yes
      PrintLastLog no # handled by PAM
      PrintMotd no # handled by PAM
      PasswordAuthentication no
      ChallengeResponseAuthentication no
{{- if .Controller.CustomFiles}}
  {{- range $w := .Controller.CustomFiles}}
  - path: {{$w.Path}}
    permissions: {{$w.PermissionsString}}
    encoding: gzip+base64
    content: {{$w.GzippedBase64Content}}
  {{- end }}
{{- end }}
  - path: /etc/modules-load.d/ip_vs.conf
    content: |
      ip_vs
      ip_vs_rr
      ip_vs_wrr
      ip_vs_sh
      nf_conntrack_ipv4
{{if and (.AmazonSsmAgent.Enabled) (ne .AmazonSsmAgent.DownloadUrl "")}}
  - path: "/opt/ssm/bin/install-ssm-agent.sh"
    permissions: 0700
    content: |
      #!/bin/bash
      set -e

      TARGET_DIR=/opt/ssm
      if [[ -f "${TARGET_DIR}"/bin/amazon-ssm-agent ]]; then
        exit 0
      fi

      TMP_DIR=$(mktemp -d)
      trap "rm -rf ${TMP_DIR}" EXIT

      TAR_FILE=ssm.linux-amd64.tar.gz
      CHECKSUM_FILE="${TAR_FILE}.sha1"

      echo -n "{{ .AmazonSsmAgent.Sha1Sum }} ${TMP_DIR}/${TAR_FILE}" > "${TMP_DIR}/${CHECKSUM_FILE}"

      curl --silent -L -o "${TMP_DIR}/${TAR_FILE}" "{{ .AmazonSsmAgent.DownloadUrl }}"

      sha1sum --quiet -c "${TMP_DIR}/${CHECKSUM_FILE}"

      tar zfx "${TMP_DIR}"/"${TAR_FILE}" -C "${TMP_DIR}"
      chown -R root:root "${TMP_DIR}"/ssm

      CONFIG_DIR=/etc/amazon/ssm
      mkdir -p "${CONFIG_DIR}"
      mv -f "${TMP_DIR}"/ssm/amazon-ssm-agent.json "${CONFIG_DIR}"/amazon-ssm-agent.json
      mv -f "${TMP_DIR}"/ssm/seelog_unix.xml "${CONFIG_DIR}"/seelog.xml

      mv -f "${TMP_DIR}"/ssm/* "${TARGET_DIR}"/bin/

{{end}}
{{if .Experimental.DisableSecurityGroupIngress}}
  - path: /etc/kubernetes/additional-configs/cloud.config
    owner: root:root
    permissions: 0644
    content: |
      [global]
      DisableSecurityGroupIngress = true
{{end}}

  - path: /opt/bin/apply-kube-aws-plugins
    permissions: 0700
    owner: root:root
    content: |
      #!/bin/bash -vxe

      kubectl() {
          /usr/bin/docker run --rm --net=host \
            -v /etc/resolv.conf:/etc/resolv.conf \
            -v {{.KubernetesManifestPlugin.Directory}}:{{.KubernetesManifestPlugin.Directory}} \
            {{.HyperkubeImage.RepoWithTag}} /hyperkube kubectl "$@"
      }

      helm() {
          /usr/bin/docker run --rm --net=host \
            -v /etc/resolv.conf:/etc/resolv.conf \
            -v {{.HelmReleasePlugin.Directory}}:{{.HelmReleasePlugin.Directory}} \
            {{.HelmImage.RepoWithTag}} helm "$@"
      }

      while read m || [[ -n $m ]]; do
        kubectl apply -f $m
      done <{{.KubernetesManifestPlugin.ManifestListFile.Path}}

      while read r || [[ -n $r ]]; do
        release_name=$(jq .name $r)
        chart_name=$(jq .chart.name $r)
        chart_version=$(jq .chart.version $r)
        values_file=$(jq .values.file $r)
        if helm status $release_name; then
          helm upgrade $release_name $chart_name --version $chart_version -f $values_file
        else
          helm install $release_name $chart_name --version $chart_version -f $values_file
        fi
      done <{{.HelmReleasePlugin.ReleaseListFile.Path}}

{{if .Experimental.AwsEnvironment.Enabled}}
  - path: /opt/bin/set-aws-environment
    owner: root:root
    permissions: 0700
    content: |
      #!/bin/bash -e

      rkt run \
        --volume=dns,kind=host,source=/etc/resolv.conf,readOnly=true \
        --mount volume=dns,target=/etc/resolv.conf \
        --volume=awsenv,kind=host,source=/etc/aws-environment,readOnly=false \
        --mount volume=awsenv,target=/etc/aws-environment \
        --uuid-file-save=/var/run/coreos/set-aws-environment.uuid \
        --net=host \
        --trust-keys-from-https \
        {{.AWSCliImage.Options}}{{.AWSCliImage.RktRepo}} --exec=/bin/bash -- \
          -ec \
          'instance_id=$(curl http://169.254.169.254/latest/meta-data/instance-id)
           stack_name=$(
             aws ec2 describe-tags --region {{.Region}} --filters \
               "Name=resource-id,Values=$instance_id" \
               "Name=key,Values=aws:cloudformation:stack-name" \
               --output json \
             | jq -r ".Tags[].Value"
           )
           cfn-init -v -c "aws-environment" --region {{.Region}} --resource {{.Controller.LogicalName}} --stack $stack_name
          '

      rkt rm --uuid-file=/var/run/coreos/set-aws-environment.uuid || :
{{end}}

  {{if .Experimental.AwsNodeLabels.Enabled -}}
  - path: /opt/bin/kube-node-label
    permissions: 0700
    owner: root:root
    content: |
      #!/bin/bash -e
      set -ue

      INSTANCE_ID="$(/usr/bin/curl -s http://169.254.169.254/latest/meta-data/instance-id)"
      SECURITY_GROUPS="$(/usr/bin/curl -s http://169.254.169.254/latest/meta-data/security-groups | tr '\n' ',')"
      AUTOSCALINGGROUP="$(/usr/bin/docker run --rm --net=host \
                {{.AWSCliImage.RepoWithTag}} aws \
                autoscaling describe-auto-scaling-instances \
                --instance-ids ${INSTANCE_ID} --region {{.Region}} \
                --query 'AutoScalingInstances[].AutoScalingGroupName' --output text)"
      LAUNCHCONFIGURATION="$(/usr/bin/docker run --rm --net=host \
                {{.AWSCliImage.RepoWithTag}} \
                aws autoscaling describe-auto-scaling-groups \
                --auto-scaling-group-name $AUTOSCALINGGROUP --region {{.Region}} \
                --query 'AutoScalingGroups[].LaunchConfigurationName' --output text)"

       until /usr/bin/curl -s -f http://127.0.0.1:8080/version; do echo waiting until apiserver starts; sleep 1; done

       /usr/bin/curl \
         --retry 5 \
         --request PATCH \
         -H 'Content-Type: application/strategic-merge-patch+json' \
         -d '{
              "metadata": {
                "labels": {
                  "kube-aws.coreos.com/autoscalinggroup": "'${AUTOSCALINGGROUP}'",
                   "kube-aws.coreos.com/launchconfiguration": "'${LAUNCHCONFIGURATION}'"
                 },
                 "annotations": {
                  "kube-aws.coreos.com/securitygroups": "'${SECURITY_GROUPS}'"
                }
              }
            }' \
         http://localhost:8080/api/v1/nodes/$(hostname)
  {{end -}}

{{ if .SharedPersistentVolume }}
  - path: /opt/bin/set-efs-pv
    owner: root:root
    permissions: 0700
    content: |
      #!/bin/bash -e

      rkt run \
        --volume=dns,kind=host,source=/etc/resolv.conf,readOnly=true \
        --mount volume=dns,target=/etc/resolv.conf \
        --volume=awsenv,kind=host,source=/etc/kubernetes,readOnly=false \
        --mount volume=awsenv,target=/etc/kubernetes \
        --uuid-file-save=/var/run/coreos/set-efs-pv.uuid \
        --net=host \
        --trust-keys-from-https \
        {{.AWSCliImage.Options}}{{.AWSCliImage.RktRepo}} --exec=/bin/bash -- \
          -ec \
          'instance_id=$(curl http://169.254.169.254/latest/meta-data/instance-id)
           stack_name=$(
             aws ec2 describe-tags --region {{.Region}} --filters \
               "Name=resource-id,Values=$instance_id" \
               "Name=key,Values=aws:cloudformation:stack-name" \
               --output json \
             | jq -r ".Tags[].Value"
           )
           cfn-init -v -c "load-efs-pv" --region {{.Region}} --resource {{.Controller.LogicalName}} --stack $stack_name
          '

      rkt rm --uuid-file=/var/run/coreos/set-efs-pv.uuid || :
{{end}}
  - path: /opt/bin/cfn-signal
    owner: root:root
    permissions: 0700
    content: |
      #!/bin/bash -e

      rkt run \
        --volume=dns,kind=host,source=/etc/resolv.conf,readOnly=true \
        --mount volume=dns,target=/etc/resolv.conf \
        --volume=awsenv,kind=host,source=/var/run/coreos,readOnly=false \
        --mount volume=awsenv,target=/var/run/coreos \
        --uuid-file-save=/var/run/coreos/cfn-signal.uuid \
        --net=host \
        --trust-keys-from-https \
        {{.AWSCliImage.Options}}{{.AWSCliImage.RktRepo}} --exec=/bin/bash -- \
          -ec \
          'instance_id=$(curl http://169.254.169.254/latest/meta-data/instance-id)
           stack_name=$(
             aws ec2 describe-tags --region {{.Region}} --filters \
               "Name=resource-id,Values=$instance_id" \
               "Name=key,Values=aws:cloudformation:stack-name" \
               --output json \
             | jq -r ".Tags[].Value"
           )
           cfn-signal -e 0 --region {{.Region}} --resource {{.Controller.LogicalName}} --stack $stack_name
          '

      rkt rm --uuid-file=/var/run/coreos/cfn-signal.uuid || :

  - path: /opt/bin/cfn-etcd-environment
    owner: root:root
    permissions: 0700
    content: |
      #!/bin/bash -e

      rkt run \
        --volume=dns,kind=host,source=/etc/resolv.conf,readOnly=true \
        --mount volume=dns,target=/etc/resolv.conf \
        --volume=awsenv,kind=host,source=/var/run/coreos,readOnly=false \
        --mount volume=awsenv,target=/var/run/coreos \
        --uuid-file-save=/var/run/coreos/cfn-etcd-environment.uuid \
        --net=host \
        --trust-keys-from-https \
        {{.AWSCliImage.Options}}{{.AWSCliImage.RktRepo}} --exec=/bin/bash -- \
          -ec \
          'instance_id=$(curl http://169.254.169.254/latest/meta-data/instance-id)
           stack_name=$(
             aws ec2 describe-tags --region {{.Region}} --filters \
               "Name=resource-id,Values=$instance_id" \
               "Name=key,Values=aws:cloudformation:stack-name" \
               --output json \
             | jq -r ".Tags[].Value"
           )
           cfn-init -v -c "etcd-client" --region {{.Region}} --resource {{.Controller.LogicalName}} --stack $stack_name
          '

      rkt rm --uuid-file=/var/run/coreos/cfn-etcd-environment.uuid || :

  - path: /etc/default/kubelet
    permissions: 0755
    owner: root:root
    content: |
      KUBELET_OPTS="{{.Experimental.KubeletOpts}}"

  - path: /opt/bin/install-kube-system
    permissions: 0700
    owner: root:root
    content: |
      #!/bin/bash -e

      vols="-v /srv/kubernetes:/srv/kubernetes"

      kubectl() {
          # --request-timeout=1s is intended to instruct kubectl to give up discovering unresponsive apiservice(s) in certain periods
          # so that temporal freakiness/unresponsity of specific apiservice until apiserver/controller-manager fully starts doesn't
          # affect the whole controller bootstrap process.
          /usr/bin/docker run -i --rm $vols --net=host {{.HyperkubeImage.RepoWithTag}} /hyperkube kubectl --request-timeout=1s "$@"
      }

      ks() {
        kubectl --namespace kube-system "$@"
      }

      # Try to batch as many files as possible to reduce the total amount of delay due to wilderness in the API aggregation
      # See https://github.com/kubernetes-incubator/kube-aws/issues/1039
      applyall() {
        kubectl apply -f $(echo "$@" | tr ' ' ',')
      }

      while ! kubectl get ns kube-system; do
        echo Waiting until kube-system created.
        sleep 3
      done

      {{if .Experimental.KIAMSupport.Enabled }}
      kiam_tls_dir=/etc/kubernetes/ssl/kiam
      vols="${vols} -v $kiam_tls_dir:$kiam_tls_dir"

      kubectl create secret generic kiam-server-tls -n kube-system \
        --from-file=$kiam_tls_dir/ca.pem \
        --from-file=$kiam_tls_dir/server.pem \
        --from-file=$kiam_tls_dir/server-key.pem --dry-run -o yaml | kubectl apply -n kube-system -f -
      kubectl create secret generic kiam-agent-tls -n kube-system \
        --from-file=$kiam_tls_dir/ca.pem \
        --from-file=$kiam_tls_dir/agent.pem \
        --from-file=$kiam_tls_dir/agent-key.pem --dry-run -o yaml | kubectl apply -n kube-system -f -

      mfdir=/srv/kubernetes/manifests
      applyall "${mfdir}/kube-system-ns.yaml" "${mfdir}/kiam-all.yaml"
      {{ end }}

      # See https://github.com/kubernetes-incubator/kube-aws/issues/1039#issuecomment-348978375
      if ks get apiservice v1beta1.metrics.k8s.io && ! ps ax | grep '[h]yperkube proxy'; then
        echo "apiserver is up but kube-proxy isn't up. We have likely encountered #1039."
        echo "Temporary deleting the v1beta1.metrics.k8s.io apiservice as a work-around for #1039"
        ks delete apiservice v1beta1.metrics.k8s.io

        echo Waiting until controller-manager stabilizes and it creates a kube-proxy pod.
        until ps ax | grep '[h]yperkube proxy'; do
            echo Sleeping 3 seconds.
            sleep 3
        done
        echo kube-proxy stared. apiserver should be responsive again.
      fi

      mfdir=/srv/kubernetes/manifests
      rbac=/srv/kubernetes/rbac

      {{ if .UseCalico }}
      /bin/bash /opt/bin/populate-tls-calico-etcd
      applyall "${mfdir}/calico.yaml"
      {{ end }}

      {{ if .Addons.MetricsServer.Enabled -}}
      applyall \
        "${mfdir}/metrics-server-sa.yaml" \
        "${mfdir}/metrics-server-de.yaml" \
        "${mfdir}/metrics-server-svc.yaml" \
        "${rbac}/cluster-roles/metrics-server.yaml" \
        "${rbac}/cluster-role-bindings/metrics-server.yaml" \
        "${rbac}/role-bindings/metrics-server.yaml" \
        "${mfdir}/metrics-server-apisvc.yaml"
      {{- end }}

      {{ if .Experimental.NodeDrainer.Enabled }}
      applyall "${mfdir}"/{kube-node-drainer-ds,kube-node-drainer-asg-status-updater-de}".yaml"
      {{ end }}

      # Secrets
      applyall "${mfdir}/kubernetes-dashboard-se.yaml"

      # Configmaps
      applyall "${mfdir}"/{kube-dns,kube-proxy,heapster-config}"-cm.yaml"

      # Service Accounts
      applyall "${mfdir}"/{kube-dns,heapster,kube-proxy,kubernetes-dashboard}"-sa.yaml"

      # Install tiller by default
      applyall "${mfdir}/tiller.yaml"

{{ if .KubeDns.NodeLocalResolver }}
      # DNS Masq Fix
      applyall "${mfdir}/dnsmasq-node-ds.yaml"
{{ end }}

      # Deployments
      applyall "${mfdir}"/{kube-dns,kube-dns-autoscaler,kubernetes-dashboard,{{ if .Addons.ClusterAutoscaler.Enabled }}cluster-autoscaler,{{ end }}heapster{{ if .KubeResourcesAutosave.Enabled }},kube-resources-autosave{{ end }}}"-de.yaml"

      # Daemonsets
      applyall "${mfdir}"/kube-proxy"-ds.yaml"

      # Services
      applyall "${mfdir}"/{kube-dns,heapster,kubernetes-dashboard}"-svc.yaml"

      {{- if .Addons.Rescheduler.Enabled }}
      applyall "${mfdir}/kube-rescheduler-de.yaml"
      {{- end }}

      mfdir=/srv/kubernetes/rbac

      # Cluster roles and bindings
      applyall "${mfdir}/cluster-roles/node-extensions.yaml"

      applyall "${mfdir}/cluster-role-bindings"/{kube-admin,system-worker,node,node-proxier,node-extensions,heapster}".yaml"

      {{ if .KubernetesDashboard.AdminPrivileges }}
      applyall "${mfdir}/cluster-role-bindings/kubernetes-dashboard-admin.yaml"
      {{- end }}

      # Roles and bindings
      applyall "${mfdir}/roles"/{pod-nanny,kubernetes-dashboard}".yaml"

      applyall "${mfdir}/role-bindings"/{heapster-nanny,kubernetes-dashboard}".yaml"

      {{ if .Experimental.TLSBootstrap.Enabled }}
      applyall "${mfdir}/cluster-roles"/{node-bootstrapper,kubelet-certificate-bootstrap}".yaml"

      applyall "${mfdir}/cluster-role-bindings"/{node-bootstrapper,kubelet-certificate-bootstrap}".yaml"
      {{ end }}

      {{if .Experimental.Kube2IamSupport.Enabled }}
        mfdir=/srv/kubernetes/manifests
        applyall "${mfdir}/kube2iam-rbac.yaml"
        applyall "${mfdir}/kube2iam-ds.yaml";
      {{ end }}

  - path: /etc/kubernetes/cni/docker_opts_cni.env
    content: |
      DOCKER_OPT_BIP=""
      DOCKER_OPT_IPMASQ=""

  - path: /opt/bin/host-rkt
    permissions: 0755
    owner: root:root
    content: |
      #!/bin/sh
      # This is bind mounted into the kubelet rootfs and all rkt shell-outs go
      # through this rkt wrapper. It essentially enters the host mount namespace
      # (which it is already in) only for the purpose of breaking out of the chroot
      # before calling rkt. It makes things like rkt gc work and avoids bind mounting
      # in certain rkt filesystem dependancies into the kubelet rootfs. This can
      # eventually be obviated when the write-api stuff gets upstream and rkt gc is
      # through the api-server. Related issue:
      # https://github.com/coreos/rkt/issues/2878
      exec nsenter -m -u -i -n -p -t 1 -- /usr/bin/rkt "$@"

{{ if .UseCalico }}
  - path: /srv/kubernetes/manifests/calico.yaml
    content: |
      kind: ConfigMap
      apiVersion: v1
      metadata:
        name: calico-config
        namespace: kube-system
      data:
        etcd_endpoints: "#ETCD_ENDPOINTS#"
        cni_network_config: |-
          {
              "name": "calico",
              "type": "flannel",
              "delegate": {
                  "type": "calico",
                  "etcd_endpoints": "__ETCD_ENDPOINTS__",
                  "etcd_key_file": "__ETCD_KEY_FILE__",
                  "etcd_cert_file": "__ETCD_CERT_FILE__",
                  "etcd_ca_cert_file": "__ETCD_CA_CERT_FILE__",
                  "log_level": "info",
                  "policy": {
                      "type": "k8s",
                      "k8s_api_root": "https://__KUBERNETES_SERVICE_HOST__:__KUBERNETES_SERVICE_PORT__",
                      "k8s_auth_token": "__SERVICEACCOUNT_TOKEN__"
                  },
                  "kubernetes": {
                      "kubeconfig": "__KUBECONFIG_FILEPATH__"
                  }
              }
          }

        etcd_ca: "/calico-secrets/etcd-ca"
        etcd_cert: "/calico-secrets/etcd-cert"
        etcd_key: "/calico-secrets/etcd-key"

      ---

      apiVersion: v1
      kind: Secret
      type: Opaque
      metadata:
        name: calico-etcd-secrets
        namespace: kube-system
      data:
        etcd-key: "$ETCDKEY"
        etcd-cert: "$ETCDCERT"
        etcd-ca: "$ETCDCA"

      ---

      kind: DaemonSet
      apiVersion: extensions/v1beta1
      metadata:
        name: calico-node
        namespace: kube-system
        labels:
          k8s-app: calico-node
      spec:
        selector:
          matchLabels:
            k8s-app: calico-node
        updateStrategy:
          type: RollingUpdate
        template:
          metadata:
            labels:
              k8s-app: calico-node
            annotations:
              scheduler.alpha.kubernetes.io/critical-pod: ''
          spec:
            {{if .Experimental.Admission.Priority.Enabled -}}
            priorityClassName: system-node-critical
            {{ end -}}
            tolerations:
            - operator: Exists
              effect: NoSchedule
            - operator: Exists
              effect: NoExecute
            - operator: Exists
              key: CriticalAddonsOnly
            hostNetwork: true
            containers:
              - name: calico-node
                image: {{ .CalicoNodeImage.RepoWithTag }}
                env:
                  - name: ETCD_ENDPOINTS
                    valueFrom:
                      configMapKeyRef:
                        name: calico-config
                        key: etcd_endpoints
                  - name: CALICO_NETWORKING_BACKEND
                    value: "none"
                  - name: CLUSTER_TYPE
                    value: "kubeaws,canal"
                  - name: CALICO_DISABLE_FILE_LOGGING
                    value: "true"
                  - name: NO_DEFAULT_POOLS
                    value: "true"
                  - name: ETCD_CA_CERT_FILE
                    valueFrom:
                      configMapKeyRef:
                        name: calico-config
                        key: etcd_ca
                  - name: ETCD_KEY_FILE
                    valueFrom:
                      configMapKeyRef:
                        name: calico-config
                        key: etcd_key
                  - name: ETCD_CERT_FILE
                    valueFrom:
                      configMapKeyRef:
                        name: calico-config
                        key: etcd_cert
                securityContext:
                  privileged: true
                volumeMounts:
                  - mountPath: /lib/modules
                    name: lib-modules
                    readOnly: true
                  - mountPath: /var/run/calico
                    name: var-run-calico
                    readOnly: false
                  - mountPath: /calico-secrets
                    name: etcd-certs
                  - mountPath: /etc/resolv.conf
                    name: dns
                    readOnly: true
            volumes:
              - name: lib-modules
                hostPath:
                  path: /lib/modules
              - name: var-run-calico
                hostPath:
                  path: /var/run/calico
              - name: cni-bin-dir
                hostPath:
                  path: /opt/cni/bin
              - name: cni-net-dir
                hostPath:
                  path: /etc/kubernetes/cni/net.d
              - name: etcd-certs
                secret:
                  secretName: calico-etcd-secrets
              - name: dns
                hostPath:
                  path: /etc/resolv.conf

      ---

      apiVersion: extensions/v1beta1
      kind: Deployment
      metadata:
        name: calico-kube-controllers
        namespace: kube-system
        labels:
          k8s-app: calico-policy
        annotations:
          scheduler.alpha.kubernetes.io/critical-pod: ''

      spec:
        replicas: 1
        template:
          metadata:
            name: calico-kube-controllers
            namespace: kube-system
            labels:
              k8s-app: calico-policy
          spec:
            {{if .Experimental.Admission.Priority.Enabled -}}
            priorityClassName: system-node-critical
            {{ end -}}
            tolerations:
            - key: "node.alpha.kubernetes.io/role"
              operator: "Equal"
              value: "master"
              effect: "NoSchedule"
            - key: "CriticalAddonsOnly"
              operator: "Exists"
            hostNetwork: true
            containers:
              - name: calico-kube-controllers
                image: {{ .CalicoKubeControllersImage.RepoWithTag }}
                env:
                  - name: ETCD_ENDPOINTS
                    valueFrom:
                      configMapKeyRef:
                        name: calico-config
                        key: etcd_endpoints
                  - name: ETCD_CA_CERT_FILE
                    valueFrom:
                      configMapKeyRef:
                        name: calico-config
                        key: etcd_ca
                  - name: ETCD_KEY_FILE
                    valueFrom:
                      configMapKeyRef:
                        name: calico-config
                        key: etcd_key
                  - name: ETCD_CERT_FILE
                    valueFrom:
                      configMapKeyRef:
                        name: calico-config
                        key: etcd_cert
                  - name: K8S_API
                    value: "https://kubernetes.default:443"
                  - name: CONFIGURE_ETC_HOSTS
                    value: "true"
                volumeMounts:
                  - mountPath: /calico-secrets
                    name: etcd-certs
            volumes:
              - name: etcd-certs
                secret:
                  secretName: calico-etcd-secrets

  - path: /opt/bin/populate-tls-calico-etcd
    owner: root:root
    permissions: 0700
    content: |
      #!/bin/bash -e

      etcd_ca=$(cat /etc/kubernetes/ssl/etcd-trusted-ca.pem | base64 | tr -d '\n')
      etcd_key=$(cat /etc/kubernetes/ssl/etcd-client-key.pem | base64 | tr -d '\n')
      etcd_cert=$(cat /etc/kubernetes/ssl/etcd-client.pem | base64 | tr -d '\n')

      sed -i -e "s#\$ETCDCA#$etcd_ca#g" /srv/kubernetes/manifests/calico.yaml
      sed -i -e "s#\$ETCDCERT#$etcd_cert#g" /srv/kubernetes/manifests/calico.yaml
      sed -i -e "s#\$ETCDKEY#$etcd_key#g" /srv/kubernetes/manifests/calico.yaml

{{ end }}
{{ if .KubeResourcesAutosave.Enabled }}
  - path: /srv/kubernetes/manifests/kube-resources-autosave-de.yaml
    content: |
      ---
      apiVersion: extensions/v1beta1
      kind: Deployment
      metadata:
        name: kube-resources-autosave
        namespace: kube-system
        labels:
          k8s-app: kube-resources-autosave-policy
      spec:
        replicas: 1
        template:
          metadata:
            {{if .Experimental.KIAMSupport.Enabled -}}
            annotations:
              iam.amazonaws.com/role: "{{$.ClusterName}}-IAMRoleResourcesAutoSave"
            {{ end -}}
            name: kube-resources-autosave
            namespace: kube-system
            labels:
              k8s-app: kube-resources-autosave-policy
          spec:
            {{if .Experimental.Admission.Priority.Enabled -}}
            priorityClassName: system-node-critical
            {{ end -}}
            containers:
            - name: kube-resources-autosave-dumper
              image: {{.HyperkubeImage.RepoWithTag}}
              command: ["/bin/sh", "-c" ]
              args:
                - |
                    set -x ;
                    DUMP_DIR_COMPLETE=/kube-resources-autosave/complete ;
                    aws configure set s3.signature_version s3v4 ;
                    mkdir -p ${DUMP_DIR_COMPLETE} ;
                    while true; do
                      TIMESTAMP=$(date +%Y-%m-%d_%H-%M-%S)
                      DUMP_DIR=/kube-resources-autosave/tmp/${TIMESTAMP} ;
                      mkdir -p ${DUMP_DIR} ;
                      RESOURCES_OUT_NAMESPACE="namespaces persistentvolumes nodes storageclasses clusterrolebindings clusterroles";
                      for r in ${RESOURCES_OUT_NAMESPACE};do
                        echo " Searching for resources: ${r}" ;
                        /kubectl get --export -o=json ${r} | \
                        jq '.items |= ([ .[] |
                            del(.status,
                            .metadata.uid,
                            .metadata.selfLink,
                            .metadata.resourceVersion,
                            .metadata.creationTimestamp,
                            .metadata.generation,
                            .spec.claimRef
                          )])' > ${DUMP_DIR}/${r}.json ;
                      done ;
                      RESOURCES_IN_NAMESPACE="componentstatuses configmaps daemonsets deployments endpoints events horizontalpodautoscalers
                      ingresses jobs limitranges networkpolicies  persistentvolumeclaims pods podsecuritypolicies podtemplates replicasets
                      replicationcontrollers resourcequotas secrets serviceaccounts services statefulsets customresourcedefinitions
                      poddisruptionbudgets roles rolebindings";
                      for ns in $(jq -r '.items[].metadata.name' < ${DUMP_DIR}/namespaces.json);do
                        echo "Searching in namespace: ${ns}" ;
                        mkdir -p ${DUMP_DIR}/${ns} ;
                        for r in ${RESOURCES_IN_NAMESPACE};do
                          echo " Searching for resources: ${r}" ;
                          /kubectl --namespace=${ns} get --export -o=json ${r} | \
                          jq '.items |= ([ .[] |
                            select(.type!="kubernetes.io/service-account-token") |
                            del(
                              .spec.clusterIP,
                              .metadata.uid,
                              .metadata.selfLink,
                              .metadata.resourceVersion,
                              .metadata.creationTimestamp,
                              .metadata.generation,
                              .metadata.annotations."pv.kubernetes.io/bind-completed",
                              .status
                            )])' > ${DUMP_DIR}/${ns}/${r}.json && touch /probe-token ;
                        done ;
                      done ;
                    mv ${DUMP_DIR} ${DUMP_DIR_COMPLETE}/${TIMESTAMP} ;
                    rm -r -f ${DUMP_DIR} ;
                    sleep 24h ;
                    done
              livenessProbe:
                exec:
                  command: ["/bin/sh", "-c",  "AGE=$(( $(date +%s) - $(stat -c%Y /probe-token) < 25*60*60 ));  [ $AGE -gt 0 ]" ]
                initialDelaySeconds: 240
                periodSeconds: 10
              volumeMounts:
              - name: dump-dir
                mountPath: /kube-resources-autosave
                readOnly: false
            - name: kube-resources-autosave-pusher
              image: {{.AWSCliImage.RepoWithTag}}
              command: ["/bin/sh", "-c" ]
              args:
                - |
                    set -x ;
                    DUMP_DIR_COMPLETE=/kube-resources-autosave/complete ;
                    while true; do
                      for FILE in ${DUMP_DIR_COMPLETE}/* ; do
                        aws s3 mv ${FILE} s3://{{ .KubeResourcesAutosave.S3Path }}/$(basename ${FILE}) --recursive && rm -r -f ${FILE} && touch /probe-token ;
                      done ;
                      sleep 1m ;
                    done
              livenessProbe:
                exec:
                  command: ["/bin/sh", "-c",  "AGE=$(( $(date +%s) - $(stat -c%Y /probe-token) < 25*60*60 ));  [ $AGE -gt 0 ]" ]
                initialDelaySeconds: 240
                periodSeconds: 10
              volumeMounts:
              - name: dump-dir
                mountPath: /kube-resources-autosave
                readOnly: false
            volumes:
            - name: dump-dir
              emptyDir: {}
{{ end }}

{{if .AssetsEncryptionEnabled }}
  - path: /opt/bin/decrypt-assets
    owner: root:root
    permissions: 0700
    content: |
      #!/bin/bash -e

      rkt run \
        --volume=kube,kind=host,source=/etc/kubernetes,readOnly=false \
        --mount=volume=kube,target=/etc/kubernetes \
        --uuid-file-save=/var/run/coreos/decrypt-assets.uuid \
        --volume=dns,kind=host,source=/etc/resolv.conf,readOnly=true --mount volume=dns,target=/etc/resolv.conf \
        --net=host \
        --trust-keys-from-https \
        {{.AWSCliImage.Options}}{{.AWSCliImage.RktRepo}} --exec=/bin/bash -- \
          -ec \
          'echo decrypting assets
           shopt -s nullglob
           for encKey in /etc/kubernetes/{ssl,{{ if or (.AssetsConfig.HasAuthTokens) ( and .Experimental.TLSBootstrap.Enabled .AssetsConfig.HasTLSBootstrapToken) }}auth{{end}}}/{,kiam/}*.enc; do
             if [ ! -f $encKey ]; then
               echo skipping non-existent file: $encKey 1>&2
               continue
             fi
             echo decrypting $encKey
             f=$(mktemp $encKey.XXXXXXXX)
             /usr/bin/aws \
               --region {{.Region}} kms decrypt \
               --ciphertext-blob fileb://$encKey \
               --output text \
               --query Plaintext \
             | base64 -d > $f
             mv -f $f ${encKey%.enc}
           done;

           {{- if or (.AssetsConfig.HasAuthTokens) ( and .Experimental.TLSBootstrap.Enabled .AssetsConfig.HasTLSBootstrapToken) }}
           authDir=/etc/kubernetes/auth
           echo generating $authDir/tokens.csv
           echo > $authDir/tokens.csv

           {{- if .AssetsConfig.HasTLSBootstrapToken }}
           echo $(cat $authDir/kubelet-tls-bootstrap-token.tmp),kubelet-bootstrap,10001,system:kubelet-bootstrap >> $authDir/tokens.csv
           {{- end }}
           {{- if .AssetsConfig.HasAuthTokens }}
           cat $authDir/tokens.csv.tmp >> $authDir/tokens.csv
           {{- end }}
           {{- end }}

           echo done.'

      rkt rm --uuid-file=/var/run/coreos/decrypt-assets.uuid || :
{{ end }}

{{if .Experimental.NodeDrainer.Enabled}}
  - path: /srv/kubernetes/manifests/kube-node-drainer-asg-status-updater-de.yaml
    content: |
        kind: Deployment
        apiVersion: extensions/v1beta1
        metadata:
          name: kube-node-drainer-asg-status-updater
          namespace: kube-system
          labels:
            k8s-app: kube-node-drainer-asg-status-updater
        spec:
          replicas: 1
          template:
            metadata:
              labels:
                k8s-app: kube-node-drainer-asg-status-updater
              annotations:
                scheduler.alpha.kubernetes.io/critical-pod: ''
                {{if ne .Experimental.NodeDrainer.IAMRole.ARN.Arn "" -}}
                iam.amazonaws.com/role: {{ .Experimental.NodeDrainer.IAMRole.ARN.Arn }}
                {{ end }}
            spec:
              {{if .Experimental.Admission.Priority.Enabled -}}
              priorityClassName: system-node-critical
              {{ end -}}
              initContainers:
                - name: hyperkube
                  image: {{.HyperkubeImage.RepoWithTag}}
                  command:
                  - /bin/cp
                  - -f
                  - /hyperkube
                  - /workdir/hyperkube
                  volumeMounts:
                  - mountPath: /workdir
                    name: workdir
              containers:
                - name: main
                  image: {{.AWSCliImage.RepoWithTag}}
                  env:
                  - name: NODE_NAME
                    valueFrom:
                      fieldRef:
                        fieldPath: spec.nodeName
                  command:
                  - /bin/sh
                  - -xec
                  - |
                    metadata() { curl -s -S -f http://169.254.169.254/2016-09-02/"$1"; }
                    asg()      { aws --region="${REGION}" autoscaling "$@"; }

                    # Hyperkube binary is not statically linked, so we need to use
                    # the musl interpreter to be able to run it in this image
                    # See: https://github.com/kubernetes-incubator/kube-aws/pull/674#discussion_r118889687
                    kubectl() { /lib/ld-musl-x86_64.so.1 /opt/bin/hyperkube kubectl "$@"; }

                    REGION=$(metadata dynamic/instance-identity/document | jq -r .region)
                    [ -n "${REGION}" ]

                    # Not customizable, for now
                    POLL_INTERVAL=10

                    # Keeps a comma-separated list of instances that need to be drained. Sets '-'
                    # to force the ConfigMap to be updated in the first iteration.
                    instances_to_drain='-'

                    # Instance termination detection loop
                    while sleep ${POLL_INTERVAL}; do

                      # Fetch the list of instances being terminated by their respective ASGs
                      updated_instances_to_drain=$(asg describe-auto-scaling-groups | jq -r '[.AutoScalingGroups[] | select((.Tags[].Key | contains("kube-aws:")) and (.Tags[].Key | contains("kubernetes.io/cluster/{{.ClusterName}}"))) | .Instances[] | select(.LifecycleState == "Terminating:Wait") | .InstanceId] | sort | join(",")')

                      # Have things changed since last iteration?
                      if [ "${updated_instances_to_drain}" == "${instances_to_drain}" ]; then
                        continue
                      fi
                      instances_to_drain="${updated_instances_to_drain}"

                      # Update ConfigMap to reflect current ASG state
                      echo "{\"apiVersion\": \"v1\", \"kind\": \"ConfigMap\", \"metadata\": {\"name\": \"kube-node-drainer-status\"}, \"data\": {\"asg\": \"${instances_to_drain}\"}}" | kubectl -n kube-system apply -f -
                    done
                  volumeMounts:
                  - mountPath: /opt/bin
                    name: workdir
              volumes:
                - name: workdir
                  emptyDir: {}

  - path: /srv/kubernetes/manifests/kube-node-drainer-ds.yaml
    content: |
        kind: DaemonSet
        apiVersion: extensions/v1beta1
        metadata:
          name: kube-node-drainer-ds
          namespace: kube-system
          labels:
            k8s-app: kube-node-drainer-ds
        spec:
          updateStrategy:
            type: RollingUpdate
          template:
            metadata:
              labels:
                k8s-app: kube-node-drainer-ds
              annotations:
                scheduler.alpha.kubernetes.io/critical-pod: ''
            spec:
              {{if .Experimental.Admission.Priority.Enabled -}}
              priorityClassName: system-node-critical
              {{ end -}}
              tolerations:
              - operator: Exists
                effect: NoSchedule
              - operator: Exists
                effect: NoExecute
              - operator: Exists
                key: CriticalAddonsOnly
              initContainers:
                - name: hyperkube
                  image: {{.HyperkubeImage.RepoWithTag}}
                  command:
                  - /bin/cp
                  - -f
                  - /hyperkube
                  - /workdir/hyperkube
                  volumeMounts:
                  - mountPath: /workdir
                    name: workdir
              containers:
                - name: main
                  image: {{.AWSCliImage.RepoWithTag}}
                  env:
                  - name: NODE_NAME
                    valueFrom:
                      fieldRef:
                        fieldPath: spec.nodeName
                  command:
                  - /bin/sh
                  - -xec
                  - |
                    metadata() { curl -s -S -f http://169.254.169.254/2016-09-02/"$1"; }
                    asg()      { aws --region="${REGION}" autoscaling "$@"; }

                    # Hyperkube binary is not statically linked, so we need to use
                    # the musl interpreter to be able to run it in this image
                    # See: https://github.com/kubernetes-incubator/kube-aws/pull/674#discussion_r118889687
                    kubectl() { /lib/ld-musl-x86_64.so.1 /opt/bin/hyperkube kubectl "$@"; }

                    INSTANCE_ID=$(metadata meta-data/instance-id)
                    REGION=$(metadata dynamic/instance-identity/document | jq -r .region)
                    [ -n "${REGION}" ]

                    # Not customizable, for now
                    POLL_INTERVAL=10

                    # Used to identify the source which requested the instance termination
                    termination_source=''

                    # Instance termination detection loop
                    while sleep ${POLL_INTERVAL}; do

                      # Spot instance termination check
                      http_status=$(curl -o /dev/null -w '%{http_code}' -sL http://169.254.169.254/latest/meta-data/spot/termination-time)
                      if [ "${http_status}" -eq 200 ]; then
                        termination_source=spot
                        break
                      fi

                      # Termination ConfigMap check
                      if [ -e /etc/kube-node-drainer/asg ] && grep -q "${INSTANCE_ID}" /etc/kube-node-drainer/asg; then
                        termination_source=asg
                        break
                      fi
                    done

                    # Node draining loop
                    while true; do
                      echo Node is terminating, draining it...

                      if ! kubectl drain --ignore-daemonsets=true --delete-local-data=true --force=true --timeout=60s "${NODE_NAME}"; then
                        echo Not all pods on this host can be evicted, will try again
                        continue
                      fi
                      echo All evictable pods are gone

                      if [ "${termination_source}" == asg ]; then
                        echo Notifying AutoScalingGroup that instance ${INSTANCE_ID} can be shutdown
                        ASG_NAME=$(asg describe-auto-scaling-instances --instance-ids "${INSTANCE_ID}" | jq -r '.AutoScalingInstances[].AutoScalingGroupName')
                        HOOK_NAME=$(asg describe-lifecycle-hooks --auto-scaling-group-name "${ASG_NAME}" | jq -r '.LifecycleHooks[].LifecycleHookName' | grep -i nodedrainer)
                        asg complete-lifecycle-action --lifecycle-action-result CONTINUE --instance-id "${INSTANCE_ID}" --lifecycle-hook-name "${HOOK_NAME}" --auto-scaling-group-name "${ASG_NAME}"
                      fi

                      # Expect instance will be shut down in 5 minutes
                      sleep 300
                    done
                  volumeMounts:
                  - mountPath: /opt/bin
                    name: workdir
                  - mountPath: /etc/kube-node-drainer
                    name: kube-node-drainer-status
                    readOnly: true
              volumes:
              - name: workdir
                emptyDir: {}
              - name: kube-node-drainer-status
                projected:
                  sources:
                  - configMap:
                      name: kube-node-drainer-status
                      optional: true
{{end}}

  # TODO: remove the following binding once the TLS Bootstrapping feature is enabled by default, see:
  # https://github.com/kubernetes-incubator/kube-aws/pull/618#discussion_r115162048
  # https://kubernetes.io/docs/admin/authorization/rbac/#core-component-roles

  # Makes kube-worker user behave like a regular member of system:nodes group,
  # needed when TLS bootstrapping is disabled
  - path: /srv/kubernetes/rbac/cluster-role-bindings/node.yaml
    content: |
        kind: ClusterRoleBinding
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: kube-aws:node
        subjects:
          - kind: User
            name: kube-worker
        roleRef:
          kind: ClusterRole
          name: system:node
          apiGroup: rbac.authorization.k8s.io

  # We need to give nodes a few extra permissions so that both the node
  # draining and node labeling with AWS metadata work as expected
  - path: /srv/kubernetes/rbac/cluster-roles/node-extensions.yaml
    content: |
        kind: ClusterRole
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
            name: kube-aws:node-extensions
        rules:
          - apiGroups: ["extensions"]
            resources:
            - daemonsets
            verbs:
            - get
          # Can be removed if node authorizer is enabled
          - apiGroups: [""]
            resources:
            - nodes
            verbs:
            - patch
            - update
          - apiGroups: ["extensions"]
            resources:
            - replicasets
            verbs:
            - get
          - apiGroups: ["batch"]
            resources:
            - jobs
            verbs:
            - get
          - apiGroups: [""]
            resources:
            - replicationcontrollers
            verbs:
            - get
          - apiGroups: [""]
            resources:
            - pods/eviction
            verbs:
            - create
          - nonResourceURLs: ["*"]
            verbs: ["*"]

  # Grants super-user permissions to the kube-admin user
  - path: /srv/kubernetes/rbac/cluster-role-bindings/kube-admin.yaml
    content: |
        kind: ClusterRoleBinding
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: kube-aws:admin
        subjects:
          - kind: User
            name: kube-admin
        roleRef:
          kind: ClusterRole
          name: cluster-admin
          apiGroup: rbac.authorization.k8s.io

  # Also allows ` + "`kube-worker`" + ` user to perform actions needed by the
  # ` + "`kube-proxy`" + ` component.
  - path: /srv/kubernetes/rbac/cluster-role-bindings/node-proxier.yaml
    content: |
        kind: ClusterRoleBinding
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: kube-aws:node-proxier
        subjects:
          - kind: User
            name: kube-worker
          - kind: ServiceAccount
            name: kube-proxy
            namespace: kube-system
          # Not needed after migrating to DaemonSet-based kube-proxy
          - kind: Group
            name: system:nodes
        roleRef:
          kind: ClusterRole
          name: system:node-proxier
          apiGroup: rbac.authorization.k8s.io

  # Allows add-ons running with the default service account in kube-sytem to have super-user access
  - path: /srv/kubernetes/rbac/cluster-role-bindings/system-worker.yaml
    content: |
        kind: ClusterRoleBinding
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: kube-aws:system-worker
        subjects:
          - kind: ServiceAccount
            namespace: kube-system
            name: default
        roleRef:
          kind: ClusterRole
          name: cluster-admin
          apiGroup: rbac.authorization.k8s.io

  # TODO: remove the following binding once the TLS Bootstrapping feature is enabled by default, see:
  # https://github.com/kubernetes-incubator/kube-aws/pull/618#discussion_r115162048
  # https://kubernetes.io/docs/admin/authorization/rbac/#core-component-roles

  # Associates the add-on role ` + "`kube-aws:node-extensions`" + ` to all nodes, so that
  # extra kube-aws features (like node draining) work as expected
  - path: /srv/kubernetes/rbac/cluster-role-bindings/node-extensions.yaml
    content: |
        kind: ClusterRoleBinding
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: kube-aws:node-extensions
        subjects:
          - kind: User
            name: kube-worker
          - kind: Group
            name: system:nodes
        roleRef:
          kind: ClusterRole
          name: kube-aws:node-extensions
          apiGroup: rbac.authorization.k8s.io

  # Allow heapster access to the built in cluster role via its service account
  - path: /srv/kubernetes/rbac/cluster-role-bindings/heapster.yaml
    content: |
        kind: ClusterRoleBinding
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: heapster
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: ClusterRole
          name: system:heapster
        subjects:
        - kind: ServiceAccount
          name: heapster
          namespace: kube-system

  # metrics-server
  - path: /srv/kubernetes/rbac/cluster-role-bindings/metrics-server.yaml
    content: |
        apiVersion: rbac.authorization.k8s.io/v1beta1
        kind: ClusterRoleBinding
        metadata:
          name: metrics-server:system:auth-delegator
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: ClusterRole
          name: system:auth-delegator
        subjects:
        - kind: ServiceAccount
          name: metrics-server
          namespace: kube-system
        ---
        apiVersion: rbac.authorization.k8s.io/v1
        kind: ClusterRoleBinding
        metadata:
          name: system:metrics-server
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: ClusterRole
          name: system:metrics-server
        subjects:
        - kind: ServiceAccount
          name: metrics-server
          namespace: kube-system

  - path: /srv/kubernetes/rbac/role-bindings/metrics-server.yaml
    content: |
        apiVersion: rbac.authorization.k8s.io/v1beta1
        kind: RoleBinding
        metadata:
          name: metrics-server-auth-reader
          namespace: kube-system
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: Role
          name: extension-apiserver-authentication-reader
        subjects:
        - kind: ServiceAccount
          name: metrics-server
          namespace: kube-system

  - path: /srv/kubernetes/rbac/cluster-roles/metrics-server.yaml
    content: |
        apiVersion: rbac.authorization.k8s.io/v1
        kind: ClusterRole
        metadata:
          name: system:metrics-server
        rules:
        - apiGroups:
          - ""
          resources:
          - pods
          - nodes
          - namespaces
          verbs:
          - get
          - list
          - watch
        - apiGroups:
          - "extensions"
          resources:
          - deployments
          verbs:
          - get
          - list
          - watch

  # Heapster's pod_nanny monitors the heapster deployment & its pod(s), and scales
  # the resources of the deployment if necessary.
  - path: /srv/kubernetes/rbac/roles/pod-nanny.yaml
    content: |
        apiVersion: rbac.authorization.k8s.io/v1
        kind: Role
        metadata:
          name: system:pod-nanny
          namespace: kube-system
          labels:
            kubernetes.io/cluster-service: "true"
            addonmanager.kubernetes.io/mode: Reconcile
        rules:
        - apiGroups:
          - ""
          resources:
          - pods
          verbs:
          - get
        - apiGroups:
          - "extensions"
          resources:
          - deployments
          verbs:
          - get
          - update

  # Allow heapster nanny access to the pod nanny role via its service account (same pod as heapster)
  - path: /srv/kubernetes/rbac/role-bindings/heapster-nanny.yaml
    content: |
        kind: RoleBinding
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: heapster-nanny
          namespace: kube-system
          labels:
            kubernetes.io/cluster-service: "true"
            addonmanager.kubernetes.io/mode: Reconcile
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: Role
          name: system:pod-nanny
        subjects:
        - kind: ServiceAccount
          name: heapster
          namespace: kube-system

{{ if .Experimental.TLSBootstrap.Enabled }}
  # A ClusterRole which instructs the CSR approver to approve a user requesting
  # node client credentials.
  - path: /srv/kubernetes/rbac/cluster-roles/kubelet-certificate-bootstrap.yaml
    content: |
        kind: ClusterRole
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: kube-aws:kubelet-certificate-bootstrap
        rules:
        - apiGroups:
          - certificates.k8s.io
          resources:
          - certificatesigningrequests/nodeclient
          verbs:
          - create

  # Approve all CSRs for the group "system:kubelet-bootstrap-token"
  - path: /srv/kubernetes/rbac/cluster-role-bindings/kubelet-certificate-bootstrap.yaml
    content: |
        kind: ClusterRoleBinding
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: kube-aws:kubelet-certificate-bootstrap
        subjects:
        - kind: Group
          name: system:kubelet-bootstrap
          apiGroup: rbac.authorization.k8s.io
        roleRef:
          kind: ClusterRole
          name: kube-aws:kubelet-certificate-bootstrap
          apiGroup: rbac.authorization.k8s.io

  # Only allows certificate signing requests to be performed with the bootstrap token
  - path: /srv/kubernetes/rbac/cluster-roles/node-bootstrapper.yaml
    content: |
        kind: ClusterRole
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: kube-aws:node-bootstrapper
        rules:
          - apiGroups:
              - certificates.k8s.io
            resources:
              - certificatesigningrequests
            verbs:
            - create
            - get
            - list
            - watch

  - path: /srv/kubernetes/rbac/cluster-role-bindings/node-bootstrapper.yaml
    content: |
        kind: ClusterRoleBinding
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: kube-aws:node-bootstrapper
        subjects:
          - kind: Group
            namespace: '*'
            name: system:kubelet-bootstrap
        roleRef:
          kind: ClusterRole
          name: kube-aws:node-bootstrapper
          apiGroup: rbac.authorization.k8s.io
{{ end }}

  #kubernetes dashboard
  - path: /srv/kubernetes/rbac/roles/kubernetes-dashboard.yaml
    content: |
        kind: Role
        apiVersion: rbac.authorization.k8s.io/v1
        metadata:
          name: kubernetes-dashboard-minimal
          namespace: kube-system
        rules:
        - apiGroups: [""]
          resources: ["secrets"]
          verbs: ["create"]
        - apiGroups: [""]
          resources: ["secrets"]
          resourceNames: ["kubernetes-dashboard-key-holder", "kubernetes-dashboard-certs"]
          verbs: ["get", "update", "delete"]
        - apiGroups: [""]
          resources: ["configmaps"]
          resourceNames: ["kubernetes-dashboard-settings"]
          verbs: ["get", "update"]
        - apiGroups: [""]
          resources: ["services"]
          resourceNames: ["heapster"]
          verbs: ["proxy"]

  - path: /srv/kubernetes/rbac/role-bindings/kubernetes-dashboard.yaml
    content: |
        apiVersion: rbac.authorization.k8s.io/v1
        kind: RoleBinding
        metadata:
          name: kubernetes-dashboard-minimal
          namespace: kube-system
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: Role
          name: kubernetes-dashboard-minimal
        subjects:
        - kind: ServiceAccount
          name: kubernetes-dashboard
          namespace: kube-system

{{ if .KubernetesDashboard.AdminPrivileges }}
  - path: /srv/kubernetes/rbac/cluster-role-bindings/kubernetes-dashboard-admin.yaml
    content: |
        apiVersion: rbac.authorization.k8s.io/v1
        kind: ClusterRoleBinding
        metadata:
          name: kubernetes-dashboard
          labels:
            k8s-app: kubernetes-dashboard
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: ClusterRole
          name: cluster-admin
        subjects:
        - kind: ServiceAccount
          name: kubernetes-dashboard
          namespace: kube-system
{{ end }}
  - path: /srv/kubernetes/manifests/kube-proxy-cm.yaml
    content: |
      kind: ConfigMap
      apiVersion: v1
      metadata:
        name: kube-proxy-config
        namespace: kube-system
      data:
        kube-proxy-config.yaml: |
          apiVersion: {{if ge .K8sVer "v1.9"}}kubeproxy.config.k8s.io{{else}}componentconfig{{end}}/v1alpha1
          kind: KubeProxyConfiguration
          bindAddress: 0.0.0.0
          clientConnection:
            kubeconfig: /etc/kubernetes/kubeconfig/kube-proxy.yaml
          clusterCIDR: {{.PodCIDR}}
          {{if .KubeProxy.IPVSMode.Enabled -}}
          featureGates: "SupportIPVSProxyMode=true"
          mode: ipvs
          ipvs:
            scheduler: {{.KubeProxy.IPVSMode.Scheduler}}
            syncPeriod: {{.KubeProxy.IPVSMode.SyncPeriod}}
            minSyncPeriod: {{.KubeProxy.IPVSMode.MinSyncPeriod}}
          {{end}}

  - path: /srv/kubernetes/manifests/kube-proxy-ds.yaml
    content: |
        apiVersion: extensions/v1beta1
        kind: DaemonSet
        metadata:
          name: kube-proxy
          namespace: kube-system
          labels:
            k8s-app: kube-proxy
          annotations:
            rkt.alpha.kubernetes.io/stage1-name-override: coreos.com/rkt/stage1-fly
        spec:
          updateStrategy:
            type: RollingUpdate
          template:
            metadata:
              labels:
                k8s-app: kube-proxy
              annotations:
                scheduler.alpha.kubernetes.io/critical-pod: ''
            spec:
              {{if .Experimental.Admission.Priority.Enabled -}}
              priorityClassName: system-node-critical
              {{ end -}}
              serviceAccountName: kube-proxy
              tolerations:
              - operator: Exists
                effect: NoSchedule
              - operator: Exists
                effect: NoExecute
              - operator: Exists
                key: CriticalAddonsOnly
              hostNetwork: true
              containers:
              - name: kube-proxy
                image: {{.HyperkubeImage.RepoWithTag}}
                command:
                - /hyperkube
                - proxy
                - --config=/etc/kubernetes/kube-proxy/kube-proxy-config.yaml
                securityContext:
                  privileged: true
                volumeMounts:
                {{if .KubeProxy.IPVSMode.Enabled -}}
                - mountPath: /lib/modules
                  name: lib-modules
                  readOnly: true
                {{end -}}
                - mountPath: /etc/kubernetes/kubeconfig
                  name: kubeconfig
                  readOnly: true
                - mountPath: /etc/kubernetes/kube-proxy
                  name: kube-proxy-config
                  readOnly: true
              volumes:
              {{if .KubeProxy.IPVSMode.Enabled -}}
              - name: lib-modules
                hostPath:
                  path: /lib/modules
              {{end -}}
              - name: kubeconfig
                hostPath:
                  path: /etc/kubernetes/kubeconfig
              - name: kube-proxy-config
                configMap:
                  name: kube-proxy-config

  - path: /etc/kubernetes/manifests/kube-apiserver.yaml
    content: |
      apiVersion: v1
      kind: Pod
      metadata:
        name: kube-apiserver
        namespace: kube-system
        labels:
          k8s-app: kube-apiserver
      spec:
        hostNetwork: true
        containers:
        - name: kube-apiserver
          image: {{.HyperkubeImage.RepoWithTag}}
          command:
          - /hyperkube
          - apiserver
          - --apiserver-count={{if .MinControllerCount}}{{ .MinControllerCount }}{{else}}{{ .ControllerCount }}{{end}}
          - --bind-address=0.0.0.0
          - --etcd-servers=#ETCD_ENDPOINTS#
          - --etcd-cafile=/etc/kubernetes/ssl/etcd-trusted-ca.pem
          - --etcd-certfile=/etc/kubernetes/ssl/etcd-client.pem
          - --etcd-keyfile=/etc/kubernetes/ssl/etcd-client-key.pem
          - --allow-privileged=true
          - --service-cluster-ip-range={{.ServiceCIDR}}
          - --secure-port=443
          {{if .Etcd.Version.Is3}}
          - --storage-backend=etcd3
          {{else}}
          - --storage-backend=etcd2
          {{end}}
          - --kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP
          {{if or (.AssetsConfig.HasAuthTokens) ( and .Experimental.TLSBootstrap.Enabled .AssetsConfig.HasTLSBootstrapToken)}}
          - --token-auth-file=/etc/kubernetes/auth/tokens.csv
          {{ end }}
          {{if .Experimental.AuditLog.Enabled}}
          - --audit-log-maxage={{.Experimental.AuditLog.MaxAge}}
          - --audit-log-path={{.Experimental.AuditLog.LogPath}}
          - --audit-log-maxbackup=1
          - --audit-policy-file=/etc/kubernetes/apiserver/audit-policy.yaml
          {{ end }}
          - --authorization-mode={{if .Experimental.NodeAuthorizer.Enabled}}Node,{{end}}RBAC
          {{if .Experimental.Authentication.Webhook.Enabled}}
          - --authentication-token-webhook-config-file=/etc/kubernetes/webhooks/authentication.yaml
          - --authentication-token-webhook-cache-ttl={{ .Experimental.Authentication.Webhook.CacheTTL }}
          {{ end }}
          - --advertise-address=$private_ipv4
          - --admission-control=NamespaceLifecycle,LimitRanger,ServiceAccount,PersistentVolumeLabel,DefaultStorageClass{{if .Experimental.Admission.PodSecurityPolicy.Enabled}},PodSecurityPolicy{{ end }}{{if .Experimental.Admission.AlwaysPullImages.Enabled}},AlwaysPullImages{{ end }}{{if .Experimental.NodeAuthorizer.Enabled}},NodeRestriction{{end}},ResourceQuota{{if .Experimental.Admission.DenyEscalatingExec.Enabled}},DenyEscalatingExec{{end}}{{if .Experimental.Admission.Initializers.Enabled}},Initializers{{end}}{{if .Experimental.Admission.Priority.Enabled}},Priority{{end}},DefaultTolerationSeconds{{if .Experimental.Admission.MutatingAdmissionWebhook.Enabled}},MutatingAdmissionWebhook{{end}}{{if .Experimental.Admission.ValidatingAdmissionWebhook.Enabled}},ValidatingAdmissionWebhook{{end}}
          - --anonymous-auth=false
          {{if .Experimental.Oidc.Enabled}}
          - --oidc-issuer-url={{.Experimental.Oidc.IssuerUrl}}
          - --oidc-client-id={{.Experimental.Oidc.ClientId}}
          {{if .Experimental.Oidc.UsernameClaim}}
          - --oidc-username-claim={{.Experimental.Oidc.UsernameClaim}}
          {{ end -}}
          {{if .Experimental.Oidc.GroupsClaim}}
          - --oidc-groups-claim={{.Experimental.Oidc.GroupsClaim}}
          {{ end -}}
          {{ end -}}
          - --cert-dir=/etc/kubernetes/ssl
          - --tls-cert-file=/etc/kubernetes/ssl/apiserver.pem
          - --tls-private-key-file=/etc/kubernetes/ssl/apiserver-key.pem
          - --client-ca-file=/etc/kubernetes/ssl/ca.pem
          - --service-account-key-file=/etc/kubernetes/ssl/apiserver-key.pem
          - --runtime-config=extensions/v1beta1/networkpolicies=true,batch/v2alpha1=true{{if .Experimental.Admission.PodSecurityPolicy.Enabled}},extensions/v1beta1/podsecuritypolicy=true{{ end }}{{if .Experimental.Admission.Initializers.Enabled}},admissionregistration.k8s.io/v1alpha1{{end}}{{if .Experimental.Admission.Priority.Enabled}},scheduling.k8s.io/v1alpha1=true{{end}}
         {{if .Experimental.Admission.Priority.Enabled}}
          - --feature-gates=PodPriority=true
         {{end}}
          - --cloud-provider=aws
          {{range $f := .APIServerFlags}}
          - --{{$f.Name}}={{$f.Value}}
          {{ end -}}
          livenessProbe:
            httpGet:
              host: 127.0.0.1
              port: 8080
              path: /healthz
            initialDelaySeconds: 15
            timeoutSeconds: 15
          ports:
          - containerPort: 443
            hostPort: 443
            name: https
          - containerPort: 8080
            hostPort: 8080
            name: local
          volumeMounts:
          - mountPath: /etc/kubernetes/ssl
            name: ssl-certs-kubernetes
            readOnly: true
          - mountPath: /etc/ssl/certs
            name: ssl-certs-host
            readOnly: true
          {{if or (.AssetsConfig.HasAuthTokens) ( and .Experimental.TLSBootstrap.Enabled .AssetsConfig.HasTLSBootstrapToken)}}
          - mountPath: /etc/kubernetes/auth
            name: auth-kubernetes
            readOnly: true
          {{end}}
          {{if .Experimental.Authentication.Webhook.Enabled}}
          - mountPath: /etc/kubernetes/webhooks
            name: kubernetes-webhooks
            readOnly: true
          {{end}}
          {{if .Experimental.AuditLog.Enabled}}
          - mountPath: /var/log
            name: var-log
            readOnly: false
          - mountPath: /etc/kubernetes/apiserver
            name: apiserver
            readOnly: true
          {{end}}
          {{range $v := .APIServerVolumes}}
          - mountPath: {{quote $v.Path}}
            name: {{quote $v.Name}}
            readOnly: {{$v.ReadOnly}}
          {{end}}
        volumes:
        - hostPath:
            path: /etc/kubernetes/ssl
          name: ssl-certs-kubernetes
        - hostPath:
            path: /usr/share/ca-certificates
          name: ssl-certs-host
        {{if or (.AssetsConfig.HasAuthTokens) ( and .Experimental.TLSBootstrap.Enabled .AssetsConfig.HasTLSBootstrapToken)}}
        - hostPath:
            path: /etc/kubernetes/auth
          name: auth-kubernetes
        {{end}}
        {{if .Experimental.Authentication.Webhook.Enabled}}
        - hostPath:
            path: /etc/kubernetes/webhooks
          name: kubernetes-webhooks
        {{end}}
        {{if .Experimental.AuditLog.Enabled}}
        - hostPath:
            path: /var/log
          name: var-log
        - hostPath:
            path: /etc/kubernetes/apiserver
          name: apiserver
        {{end}}
        {{range $v := .APIServerVolumes}}
        - hostPath:
            path: {{quote $v.Path}}
          name: {{quote $v.Name}}
        {{end}}

  - path: /etc/kubernetes/manifests/kube-controller-manager.yaml
    content: |
      apiVersion: v1
      kind: Pod
      metadata:
        name: kube-controller-manager
        namespace: kube-system
        labels:
          k8s-app: kube-controller-manager
      spec:
        containers:
        - name: kube-controller-manager
          image: {{.HyperkubeImage.RepoWithTag}}
          command:
          - /hyperkube
          - controller-manager
          - --master=http://127.0.0.1:8080
          - --leader-elect=true
          - --service-account-private-key-file=/etc/kubernetes/ssl/apiserver-key.pem
          {{ if .Experimental.TLSBootstrap.Enabled }}
          - --insecure-experimental-approve-all-kubelet-csrs-for-group=system:kubelet-bootstrap
          - --cluster-signing-cert-file=/etc/kubernetes/ssl/worker-ca.pem
          - --cluster-signing-key-file=/etc/kubernetes/ssl/worker-ca-key.pem
          {{ end }}
          - --root-ca-file=/etc/kubernetes/ssl/ca.pem
          - --cloud-provider=aws
          {{if .Experimental.NodeMonitorGracePeriod}}
          - --node-monitor-grace-period={{ .Experimental.NodeMonitorGracePeriod }}
          {{end}}
          {{if .Experimental.DisableSecurityGroupIngress}}
          - --cloud-config=/etc/kubernetes/additional-configs/cloud.config
          {{end}}
          resources:
            requests:
              cpu: 200m
          livenessProbe:
            httpGet:
              host: 127.0.0.1
              path: /healthz
              port: 10252
            initialDelaySeconds: 15
            timeoutSeconds: 15
          volumeMounts:
          {{if .Experimental.DisableSecurityGroupIngress}}
          - mountPath: /etc/kubernetes/additional-configs
            name: additional-configs
            readOnly: true
          {{end}}
          - mountPath: /etc/kubernetes/ssl
            name: ssl-certs-kubernetes
            readOnly: true
          - mountPath: /etc/ssl/certs
            name: ssl-certs-host
            readOnly: true
        hostNetwork: true
        volumes:
        {{if .Experimental.DisableSecurityGroupIngress}}
        - hostPath:
            path: /etc/kubernetes/additional-configs
          name: additional-configs
        {{end}}
        - hostPath:
            path: /etc/kubernetes/ssl
          name: ssl-certs-kubernetes
        - hostPath:
            path: /usr/share/ca-certificates
          name: ssl-certs-host

  - path: /etc/kubernetes/manifests/kube-scheduler.yaml
    content: |
      apiVersion: v1
      kind: Pod
      metadata:
        name: kube-scheduler
        namespace: kube-system
        labels:
          k8s-app: kube-scheduler
      spec:
        hostNetwork: true
        containers:
        - name: kube-scheduler
          image: {{.HyperkubeImage.RepoWithTag}}
          command:
          - /hyperkube
          - scheduler
          - --master=http://127.0.0.1:8080
          - --leader-elect=true
         {{if .Experimental.Admission.Priority.Enabled}}
          - --feature-gates=PodPriority=true
         {{end}}
          resources:
            requests:
              cpu: 100m
          livenessProbe:
            httpGet:
              host: 127.0.0.1
              path: /healthz
              port: 10251
            initialDelaySeconds: 15
            timeoutSeconds: 15

  {{- if .Addons.Rescheduler.Enabled }}
  - path: /srv/kubernetes/manifests/kube-rescheduler-de.yaml
    content: |
      apiVersion: extensions/v1beta1
      kind: Deployment
      metadata:
        name: kube-rescheduler
        namespace: kube-system
        labels:
          k8s-app: kube-rescheduler
          kubernetes.io/cluster-service: "true"
          kubernetes.io/name: "Rescheduler"
      spec:
        # ` + "`replicas`" + ` should always be the default of 1, rescheduler crashes otherwise
        template:
          metadata:
            labels:
              k8s-app: kube-rescheduler
            annotations:
              scheduler.alpha.kubernetes.io/critical-pod: ''
          spec:
            {{if .Experimental.Admission.Priority.Enabled -}}
            priorityClassName: system-node-critical
            {{ end -}}
            tolerations:
            - key: "CriticalAddonsOnly"
              operator: "Exists"
            hostNetwork: true
            containers:
            - name: kube-rescheduler
              image: {{ .KubeReschedulerImage.RepoWithTag }}
              resources:
                requests:
                  cpu: 10m
                  memory: 100Mi
  {{- end }}

  - path: /srv/kubernetes/manifests/kube-proxy-sa.yaml
    content: |
        apiVersion: v1
        kind: ServiceAccount
        metadata:
          name: kube-proxy
          namespace: kube-system
          labels:
            kubernetes.io/cluster-service: "true"

  - path: /srv/kubernetes/manifests/kube-dns-sa.yaml
    content: |
        apiVersion: v1
        kind: ServiceAccount
        metadata:
          name: kube-dns
          namespace: kube-system
          labels:
            kubernetes.io/cluster-service: "true"

  - path: /srv/kubernetes/manifests/kube-dns-cm.yaml
    content: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: kube-dns
          namespace: kube-system

  - path: /srv/kubernetes/manifests/heapster-config-cm.yaml
    content: |
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: heapster-config
          namespace: kube-system
          labels:
            kubernetes.io/cluster-service: "true"
            addonmanager.kubernetes.io/mode: EnsureExists
        data:
          NannyConfiguration: |-
            apiVersion: nannyconfig/v1alpha1
            kind: NannyConfiguration

  - path: /srv/kubernetes/manifests/kube-dns-autoscaler-de.yaml
    content: |
        apiVersion: extensions/v1beta1
        kind: Deployment
        metadata:
          name: kube-dns-autoscaler
          namespace: kube-system
          labels:
            k8s-app: kube-dns-autoscaler
            kubernetes.io/cluster-service: "true"
        spec:
          template:
            metadata:
              labels:
                k8s-app: kube-dns-autoscaler
              annotations:
                scheduler.alpha.kubernetes.io/critical-pod: ''
            spec:
              {{if .Experimental.Admission.Priority.Enabled -}}
              priorityClassName: system-node-critical
              {{ end -}}
              tolerations:
              - key: "CriticalAddonsOnly"
                operator: "Exists"
              containers:
              - name: autoscaler
                image: {{ .ClusterProportionalAutoscalerImage.RepoWithTag }}
                resources:
                    requests:
                        cpu: "20m"
                        memory: "10Mi"
                command:
                  - /cluster-proportional-autoscaler
                  - --namespace=kube-system
                  - --configmap=kube-dns-autoscaler
                  - --target=Deployment/kube-dns
                  - --default-params={"linear":{"coresPerReplica":{{ .KubeDns.Autoscaler.CoresPerReplica }},"nodesPerReplica":{{ .KubeDns.Autoscaler.NodesPerReplica }},"min":{{ .KubeDns.Autoscaler.Min}}}}
                  - --logtostderr=true
                  - --v=2

{{ if .KubeDns.NodeLocalResolver }}
  - path: /srv/kubernetes/manifests/dnsmasq-node-ds.yaml
    content: |
        apiVersion: extensions/v1beta1
        kind: DaemonSet
        metadata:
          name: dnsmasq-node
          namespace: kube-system
          labels:
            k8s-app: dnsmasq-node
        spec:
          updateStrategy:
            type: RollingUpdate
          template:
            metadata:
              labels:
                k8s-app: dnsmasq-node
              annotations:
                scheduler.alpha.kubernetes.io/critical-pod: ''
            spec:
              {{if .Experimental.Admission.Priority.Enabled -}}
              priorityClassName: system-node-critical
              {{ end -}}
              tolerations:
              - operator: Exists
                effect: NoSchedule
              - operator: Exists
                effect: NoExecute
              - operator: Exists
                key: CriticalAddonsOnly
              volumes:
              - name: kube-dns-config
                configMap:
                  name: kube-dns
                  optional: true
              containers:
              - name: dnsmasq
                image: {{ .KubeDnsMasqImage.RepoWithTag }}
                livenessProbe:
                  httpGet:
                    path: /healthcheck/dnsmasq
                    port: 10054
                    scheme: HTTP
                  initialDelaySeconds: 60
                  timeoutSeconds: 5
                  successThreshold: 1
                  failureThreshold: 5
                args:
                - -v=2
                - -logtostderr
                - -configDir=/etc/k8s/dns/dnsmasq-nanny
                - -restartDnsmasq=true
                - --
                - -k
                - --cache-size=1000
                - --server=/cluster.local/{{.DNSServiceIP}}
                - --server=/in-addr.arpa/{{.DNSServiceIP}}
                - --server=/ip6.arpa/{{.DNSServiceIP}}
                - --log-facility=-
                ports:
                - containerPort: 53
                  name: dns
                  protocol: UDP
                - containerPort: 53
                  name: dns-tcp
                  protocol: TCP
                # see: https://github.com/kubernetes/kubernetes/issues/29055 for details
                resources:
                  requests:
                    cpu: 150m
                    memory: 20Mi
                volumeMounts:
                - name: kube-dns-config
                  mountPath: /etc/k8s/dns/dnsmasq-nanny
              - name: sidecar
                image: {{ .DnsMasqMetricsImage.RepoWithTag }}
                livenessProbe:
                  httpGet:
                    path: /metrics
                    port: 10054
                    scheme: HTTP
                  initialDelaySeconds: 60
                  timeoutSeconds: 5
                  successThreshold: 1
                  failureThreshold: 5
                args:
                - --v=2
                - --logtostderr
                - --probe=dnsmasq,127.0.0.1:53,ec2.amazonaws.com,5,A
                ports:
                - containerPort: 10054
                  name: metrics
                  protocol: TCP
                resources:
                  requests:
                    memory: 20Mi
              hostNetwork: true
              dnsPolicy: Default
              automountServiceAccountToken: false
{{ end }}

  - path: /srv/kubernetes/manifests/kube-dns-de.yaml
    content: |
        apiVersion: extensions/v1beta1
        kind: Deployment
        metadata:
          name: kube-dns
          namespace: kube-system
          labels:
            k8s-app: kube-dns
            kubernetes.io/cluster-service: "true"
        spec:
          # replicas: not specified here:
          # 1. In order to make Addon Manager do not reconcile this replicas parameter.
          # 2. Default is 1.
          # 3. Will be tuned in real time if DNS horizontal auto-scaling is turned on.
          strategy:
            rollingUpdate:
              maxSurge: 10%
              maxUnavailable: 0
          selector:
            matchLabels:
              k8s-app: kube-dns
          template:
            metadata:
              labels:
                k8s-app: kube-dns
              annotations:
                scheduler.alpha.kubernetes.io/critical-pod: ''
            spec:
              {{if .Experimental.Admission.Priority.Enabled -}}
              priorityClassName: system-node-critical
              {{ end -}}
              volumes:
              - name: kube-dns-config
                configMap:
                  name: kube-dns
                  optional: true
              {{ if .KubeDns.DeployToControllers -}}
              affinity:
                nodeAffinity:
                  requiredDuringSchedulingIgnoredDuringExecution:
                    nodeSelectorTerms:
                    - matchExpressions:
                      - key: node-role.kubernetes.io/master
                        operator: In
                        values:
                        - ""
              tolerations:
               - key: "CriticalAddonsOnly"
                 operator: "Exists"
               - key: "node.alpha.kubernetes.io/role"
                 operator: "Equal"
                 value: "master"
                 effect: "NoSchedule"
              {{ else -}}
              tolerations:
              - key: "CriticalAddonsOnly"
                operator: "Exists"
              {{ end -}}
              containers:
              - name: kubedns
                image: {{ .KubeDnsImage.RepoWithTag }}
                resources:
                  limits:
                    memory: 170Mi
                  requests:
                    cpu: 100m
                    memory: 70Mi
                livenessProbe:
                  httpGet:
                    path: /healthcheck/kubedns
                    port: 10054
                    scheme: HTTP
                  initialDelaySeconds: 60
                  timeoutSeconds: 5
                  successThreshold: 1
                  failureThreshold: 5
                readinessProbe:
                  httpGet:
                    path: /readiness
                    port: 8081
                    scheme: HTTP
                  initialDelaySeconds: 3
                  timeoutSeconds: 5
                args:
                - --domain=cluster.local.
                - --dns-port=10053
                - --config-dir=/kube-dns-config
                # This should be set to v=2 only after the new image (cut from 1.5) has
                # been released, otherwise we will flood the logs.
                - --v=2
                env:
                - name: PROMETHEUS_PORT
                  value: "10055"
                ports:
                - containerPort: 10053
                  name: dns-local
                  protocol: UDP
                - containerPort: 10053
                  name: dns-tcp-local
                  protocol: TCP
                - containerPort: 10055
                  name: metrics
                  protocol: TCP
                volumeMounts:
                - name: kube-dns-config
                  mountPath: /kube-dns-config
              - name: dnsmasq
                image: {{ .KubeDnsMasqImage.RepoWithTag }}
                livenessProbe:
                  httpGet:
                    path: /healthcheck/dnsmasq
                    port: 10054
                    scheme: HTTP
                  initialDelaySeconds: 60
                  timeoutSeconds: 5
                  successThreshold: 1
                  failureThreshold: 5
                args:
                - -v=2
                - -logtostderr
                - -configDir=/etc/k8s/dns/dnsmasq-nanny
                - -restartDnsmasq=true
                - --
                - -k
                - --cache-size=1000
                - --log-facility=-
                - --server=/cluster.local/127.0.0.1#10053
                - --server=/in-addr.arpa/127.0.0.1#10053
                - --server=/ip6.arpa/127.0.0.1#10053
                ports:
                - containerPort: 53
                  name: dns
                  protocol: UDP
                - containerPort: 53
                  name: dns-tcp
                  protocol: TCP
                # see: https://github.com/kubernetes/kubernetes/issues/29055 for details
                resources:
                  requests:
                    cpu: 150m
                    memory: 20Mi
                volumeMounts:
                - name: kube-dns-config
                  mountPath: /etc/k8s/dns/dnsmasq-nanny
              - name: sidecar
                image: {{ .DnsMasqMetricsImage.RepoWithTag }}
                livenessProbe:
                  httpGet:
                    path: /metrics
                    port: 10054
                    scheme: HTTP
                  initialDelaySeconds: 60
                  timeoutSeconds: 5
                  successThreshold: 1
                  failureThreshold: 5
                args:
                - --v=2
                - --logtostderr
                - --probe=kubedns,127.0.0.1:10053,kubernetes.default.svc.cluster.local,5,A
                - --probe=dnsmasq,127.0.0.1:53,kubernetes.default.svc.cluster.local,5,A
                ports:
                - containerPort: 10054
                  name: metrics
                  protocol: TCP
                resources:
                  requests:
                    memory: 20Mi
                    cpu: 10m
              dnsPolicy: Default
              serviceAccountName: kube-dns

  - path: /srv/kubernetes/manifests/kube-dns-svc.yaml
    content: |
        apiVersion: v1
        kind: Service
        metadata:
          name: kube-dns
          namespace: kube-system
          labels:
            k8s-app: kube-dns
            kubernetes.io/cluster-service: "true"
            kubernetes.io/name: "KubeDNS"
        spec:
          selector:
            k8s-app: kube-dns
          clusterIP: {{.DNSServiceIP}}
          ports:
          - name: dns
            port: 53
            protocol: UDP
          - name: dns-tcp
            port: 53
            protocol: TCP

  - path: /srv/kubernetes/manifests/heapster-sa.yaml
    content: |
        apiVersion: v1
        kind: ServiceAccount
        metadata:
          name: heapster
          namespace: kube-system
          labels:
            kubernetes.io/cluster-service: "true"

  - path: /srv/kubernetes/manifests/heapster-de.yaml
    content: |
        apiVersion: extensions/v1beta1
        kind: Deployment
        metadata:
          name: heapster
          namespace: kube-system
          labels:
            k8s-app: heapster
            kubernetes.io/cluster-service: "true"
            version: v1.5.0
        spec:
          replicas: 1
          selector:
            matchLabels:
              k8s-app: heapster
              version: v1.5.0
          template:
            metadata:
              labels:
                k8s-app: heapster
                version: v1.5.0
              annotations:
                scheduler.alpha.kubernetes.io/critical-pod: ''
            spec:
              {{if .Experimental.Admission.Priority.Enabled -}}
              priorityClassName: system-node-critical
              {{ end -}}
              tolerations:
              - key: "CriticalAddonsOnly"
                operator: "Exists"
              serviceAccountName: heapster
              containers:
                - image: {{ .HeapsterImage.RepoWithTag }}
                  name: heapster
                  livenessProbe:
                    httpGet:
                      path: /healthz
                      port: 8082
                      scheme: HTTP
                    initialDelaySeconds: 180
                    timeoutSeconds: 5
                  resources:
                    limits:
                      cpu: 80m
                      memory: 200Mi
                    requests:
                      cpu: 80m
                      memory: 200Mi
                  command:
                    - /heapster
                    - --source=kubernetes.summary_api:''
                - image: {{ .AddonResizerImage.RepoWithTag }}
                  name: heapster-nanny
                  resources:
                    limits:
                      cpu: 50m
                      memory: 90Mi
                    requests:
                      cpu: 50m
                      memory: 90Mi
                  env:
                    - name: MY_POD_NAME
                      valueFrom:
                        fieldRef:
                          fieldPath: metadata.name
                    - name: MY_POD_NAMESPACE
                      valueFrom:
                        fieldRef:
                          fieldPath: metadata.namespace
                  volumeMounts:
                  - name: heapster-config-volume
                    mountPath: /etc/config
                  command:
                    - /pod_nanny
                    - --config-dir=/etc/config
                    - --cpu=80m
                    - --extra-cpu=4m
                    - --memory=200Mi
                    - --extra-memory=4Mi
                    - --threshold=5
                    - --deployment=heapster
                    - --container=heapster
                    - --poll-period=300000
                    - --estimator=exponential
              volumes:
                - name: heapster-config-volume
                  configMap:
                    name: heapster-config

  - path: /srv/kubernetes/manifests/metrics-server-sa.yaml
    content: |
        apiVersion: v1
        kind: ServiceAccount
        metadata:
          name: metrics-server
          namespace: kube-system
          labels:

  - path: /srv/kubernetes/manifests/metrics-server-de.yaml
    content: |
        apiVersion: extensions/v1beta1
        kind: Deployment
        metadata:
          name: metrics-server
          namespace: kube-system
          labels:
            k8s-app: metrics-server
          annotations:
            scheduler.alpha.kubernetes.io/critical-pod: ''
        spec:
          selector:
            matchLabels:
              k8s-app: metrics-server
          template:
            metadata:
              name: metrics-server
              labels:
                k8s-app: metrics-server
            spec:
              {{if .Experimental.Admission.Priority.Enabled -}}
              priorityClassName: system-node-critical
              {{ end -}}
              serviceAccountName: metrics-server
              containers:
              - name: metrics-server
                image: {{ .MetricsServerImage.RepoWithTag }}
                imagePullPolicy: Always
                command:
                - /metrics-server
                - --source=kubernetes.summary_api:''
                - --requestheader-client-ca-file=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
                - --requestheader-username-headers=X-Remote-User
                - --requestheader-group-headers=X-Remote-Group
                - --requestheader-extra-headers-prefix=X-Remote-Extra
                resources:
                  limits:
                    cpu: 80m
                    memory: 200Mi
                  requests:
                    cpu: 80m
                    memory: 200Mi

  - path: /srv/kubernetes/manifests/metrics-server-apisvc.yaml
    content: |
        apiVersion: apiregistration.k8s.io/v1beta1
        kind: APIService
        metadata:
          name: v1beta1.metrics.k8s.io
        spec:
          service:
            name: metrics-server
            namespace: kube-system
          group: metrics.k8s.io
          version: v1beta1
          insecureSkipTLSVerify: true
          groupPriorityMinimum: 100
          versionPriority: 100

  - path: /srv/kubernetes/manifests/metrics-server-svc.yaml
    content: |
        apiVersion: v1
        kind: Service
        metadata:
          name: metrics-server
          namespace: kube-system
          labels:
            kubernetes.io/name: "Metrics-server"
        spec:
          selector:
            k8s-app: metrics-server
          ports:
          - port: 443
            protocol: TCP
            targetPort: 443

  {{if .Addons.ClusterAutoscaler.Enabled}}
  - path: /srv/kubernetes/manifests/cluster-autoscaler-de.yaml
    content: |
        apiVersion: extensions/v1beta1
        kind: Deployment
        metadata:
          name: cluster-autoscaler
          namespace: kube-system
          labels:
            app: cluster-autoscaler
        spec:
          replicas: 1
          selector:
            matchLabels:
              app: cluster-autoscaler
          template:
            metadata:
              labels:
                app: cluster-autoscaler
              annotations:
                scheduler.alpha.kubernetes.io/critical-pod: ''
            spec:
              {{if .Experimental.Admission.Priority.Enabled -}}
              priorityClassName: system-node-critical
              {{ end -}}
              affinity:
                nodeAffinity:
                  requiredDuringSchedulingIgnoredDuringExecution:
                    nodeSelectorTerms:
                    - matchExpressions:
                      - key: "kube-aws.coreos.com/cluster-autoscaler-supported"
                        operator: "In"
                        values:
                        - "true"
              tolerations:
              - key: "node.alpha.kubernetes.io/role"
                operator: "Equal"
                value: "master"
                effect: "NoSchedule"
              - key: "CriticalAddonsOnly"
                operator: "Exists"
              containers:
                - image: {{ .ClusterAutoscalerImage.RepoWithTag }}
                  name: cluster-autoscaler
                  resources:
                    limits:
                      cpu: 100m
                      memory: 300Mi
                    requests:
                      cpu: 100m
                      memory: 300Mi
                  command:
                    - ./cluster-autoscaler
                    - --v=4
                    - --stderrthreshold=info
                    - --cloud-provider=aws
                    - --skip-nodes-with-local-storage=false
                    - --expander=least-waste
                    - --node-group-auto-discovery=asg:tag=k8s.io/cluster-autoscaler/enabled,kubernetes.io/cluster/{{.ClusterName}}
                  env:
                    - name: AWS_REGION
                      value: {{.Region}}
                  imagePullPolicy: "Always"
  {{end}}

  - path: /srv/kubernetes/manifests/heapster-svc.yaml
    content: |
        kind: Service
        apiVersion: v1
        metadata:
          name: heapster
          namespace: kube-system
          labels:
            kubernetes.io/cluster-service: "true"
            kubernetes.io/name: "Heapster"
            k8s-app: heapster
        spec:
          ports:
            - port: 80
              targetPort: 8082
          selector:
            k8s-app: heapster

  - path: /srv/kubernetes/manifests/kubernetes-dashboard-se.yaml
    content: |
        apiVersion: v1
        kind: Secret
        metadata:
          labels:
            k8s-app: kubernetes-dashboard
          name: kubernetes-dashboard-certs
          namespace: kube-system
        type: Opaque

  - path: /srv/kubernetes/manifests/kubernetes-dashboard-sa.yaml
    content: |
        apiVersion: v1
        kind: ServiceAccount
        metadata:
          labels:
            k8s-app: kubernetes-dashboard
          name: kubernetes-dashboard
          namespace: kube-system

  - path: /srv/kubernetes/manifests/kubernetes-dashboard-de.yaml
    content: |
        kind: Deployment
        apiVersion: apps/v1beta2
        metadata:
          labels:
            k8s-app: kubernetes-dashboard
          name: kubernetes-dashboard
          namespace: kube-system
        spec:
          replicas: 1
          revisionHistoryLimit: 10
          selector:
            matchLabels:
              k8s-app: kubernetes-dashboard
          template:
            metadata:
              labels:
                k8s-app: kubernetes-dashboard
            spec:
              {{if .Experimental.Admission.Priority.Enabled -}}
              priorityClassName: system-node-critical
              {{ end -}}
              containers:
              - name: kubernetes-dashboard
                image: {{ .KubernetesDashboardImage.RepoWithTag }}
                ports:
                {{ if .KubernetesDashboard.InsecureLogin }}
                - containerPort: 9090
                {{ else }}
                - containerPort: 8443
                {{ end }}
                  protocol: TCP
                args:
                  {{ if .KubernetesDashboard.InsecureLogin }}
                  - --enable-insecure-login
                  - --insecure-port=9090
                  {{ else }}
                  - --auto-generate-certificates
                  {{ end }}
                resources:
                  limits:
                    cpu: 100m
                    memory: 100Mi
                  requests:
                    cpu: 100m
                    memory: 100Mi
                volumeMounts:
                - name: kubernetes-dashboard-certs
                  mountPath: /certs
                - mountPath: /tmp
                  name: tmp-volume
                livenessProbe:
                  httpGet:
                    {{ if .KubernetesDashboard.InsecureLogin }}
                    scheme: HTTP
                    {{ else }}
                    scheme: HTTPS
                    {{ end }}
                    path: /
                    {{ if .KubernetesDashboard.InsecureLogin }}
                    port: 9090
                    {{ else }}
                    port: 8443
                    {{ end }}
                  initialDelaySeconds: 30
                  timeoutSeconds: 30
              volumes:
              - name: kubernetes-dashboard-certs
                secret:
                  secretName: kubernetes-dashboard-certs
              - name: tmp-volume
                emptyDir: {}
              serviceAccountName: kubernetes-dashboard
              tolerations:
              - key: "node.alpha.kubernetes.io/role"
                operator: "Equal"
                value: "master"
                effect: "NoSchedule"
              - key: "CriticalAddonsOnly"
                operator: "Exists"

  - path: /srv/kubernetes/manifests/kubernetes-dashboard-svc.yaml
    content: |
        kind: Service
        apiVersion: v1
        metadata:
          labels:
            k8s-app: kubernetes-dashboard
          name: kubernetes-dashboard
          namespace: kube-system
        spec:
          ports:
            {{ if .KubernetesDashboard.InsecureLogin }}
            - port: 9090
              targetPort: 9090
            {{ else }}
            - port: 443
              targetPort: 8443
            {{ end }}
          selector:
            k8s-app: kubernetes-dashboard

  - path: /srv/kubernetes/manifests/tiller.yaml
    content: |
        apiVersion: extensions/v1beta1
        kind: Deployment
        metadata:
          creationTimestamp: null
          labels:
            app: helm
            name: tiller
          name: tiller-deploy
          namespace: kube-system
        spec:
          strategy: {}
          template:
            metadata:
              creationTimestamp: null
              labels:
                app: helm
                name: tiller
              # Addition to the default tiller deployment for prioritizing tiller over other non-critical pods with rescheduler
              annotations:
                scheduler.alpha.kubernetes.io/critical-pod: ''
            spec:
              {{if .Experimental.Admission.Priority.Enabled -}}
              priorityClassName: system-node-critical
              {{ end -}}
              tolerations:
              # Additions to the default tiller deployment for allowing to schedule tiller onto controller nodes
              # so that helm can be used to install pods running only on controller nodes
              - key: "node.alpha.kubernetes.io/role"
                operator: "Equal"
                value: "master"
                effect: "NoSchedule"
              - key: "CriticalAddonsOnly"
                operator: "Exists"
              containers:
              - env:
                - name: TILLER_NAMESPACE
                  value: kube-system
                image: {{.TillerImage.RepoWithTag}}
                imagePullPolicy: IfNotPresent
                livenessProbe:
                  httpGet:
                    path: /liveness
                    port: 44135
                  initialDelaySeconds: 1
                  timeoutSeconds: 1
                name: tiller
                ports:
                - containerPort: 44134
                  name: tiller
                readinessProbe:
                  httpGet:
                    path: /readiness
                    port: 44135
                  initialDelaySeconds: 1
                  timeoutSeconds: 1
                resources: {}
              nodeSelector:
                beta.kubernetes.io/os: linux
        status: {}
        ---
        apiVersion: v1
        kind: Service
        metadata:
          creationTimestamp: null
          labels:
            app: helm
            name: tiller
          name: tiller-deploy
          namespace: kube-system
        spec:
          ports:
          - name: tiller
            port: 44134
            targetPort: tiller
          selector:
            app: helm
            name: tiller
          type: ClusterIP
        status:
          loadBalancer: {}

  - path: {{.KubernetesManifestPlugin.ManifestListFile.Path}}
    encoding: gzip+base64
    content: {{.KubernetesManifestPlugin.ManifestListFile.Content.ToGzip.ToBase64}}

{{ range $m := .KubernetesManifestPlugin.Manifests }}
{{ $f := $m.ManifestFile }}
  - path: {{$f.Path}}
    encoding: gzip+base64
    content: {{$f.Content.ToGzip.ToBase64}}
{{ end }}

  - path: {{.HelmReleasePlugin.ReleaseListFile.Path}}
    encoding: gzip+base64
    content: {{.HelmReleasePlugin.ReleaseListFile.Content.ToGzip.ToBase64}}


{{ range $r := .HelmReleasePlugin.Releases }}
{{ $f := $r.ReleaseFile }}
  - path: {{$f.Path}}
    encoding: gzip+base64
    content: {{$f.Content.ToGzip.ToBase64}}
{{ $f := $r.ValuesFile }}
  - path: {{$f.Path}}
    encoding: gzip+base64
    content: {{$f.Content.ToGzip.ToBase64}}
{{ end }}

{{ if .AssetsConfig.HasTLSBootstrapToken }}
  - path: /etc/kubernetes/auth/kubelet-tls-bootstrap-token.tmp{{if .AssetsEncryptionEnabled}}.enc{{end}}
    encoding: gzip+base64
    content: {{.AssetsConfig.TLSBootstrapToken}}
{{ end }}

{{ if .AssetsConfig.HasAuthTokens }}
  - path: /etc/kubernetes/auth/tokens.csv.tmp{{if .AssetsEncryptionEnabled}}.enc{{end}}
    encoding: gzip+base64
    content: {{.AssetsConfig.AuthTokens}}
{{ end }}

{{ if .ManageCertificates }}
  - path: /etc/kubernetes/ssl/ca.pem
    encoding: gzip+base64
    content: {{.AssetsConfig.CACert}}

{{ if .Experimental.TLSBootstrap.Enabled }}
  - path: /etc/kubernetes/ssl/worker-ca-key.pem.enc
    encoding: gzip+base64
    content: {{.AssetsConfig.WorkerCAKey}}

  - path: /etc/kubernetes/ssl/worker-ca.pem
    encoding: gzip+base64
    content: {{.AssetsConfig.WorkerCACert}}
{{ end }}

  - path: /etc/kubernetes/ssl/apiserver.pem
    encoding: gzip+base64
    content: {{.AssetsConfig.APIServerCert}}

  - path: /etc/kubernetes/ssl/apiserver-key.pem{{if .AssetsEncryptionEnabled}}.enc{{end}}
    encoding: gzip+base64
    content: {{.AssetsConfig.APIServerKey}}

  - path: /etc/kubernetes/ssl/etcd-client.pem
    encoding: gzip+base64
    content: {{.AssetsConfig.EtcdClientCert}}

  - path: /etc/kubernetes/ssl/etcd-client-key.pem{{if .AssetsEncryptionEnabled}}.enc{{end}}
    encoding: gzip+base64
    content: {{.AssetsConfig.EtcdClientKey}}

  - path: /etc/kubernetes/ssl/etcd-trusted-ca.pem
    encoding: gzip+base64
    content: {{.AssetsConfig.EtcdTrustedCA}}
{{ end }}

  # File needed on every node (used by the kube-proxy DaemonSet), including controllers
  - path: /etc/kubernetes/kubeconfig/kube-proxy.yaml
    content: |
        apiVersion: v1
        kind: Config
        clusters:
        - name: default
          cluster:
            server: http://localhost:8080
        users:
        - name: default
          user:
            tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
        contexts:
        - context:
            cluster: default
            user: default
          name: default
        current-context: default

  - path: /etc/kubernetes/kubeconfig/controller.yaml
    content: |
        apiVersion: v1
        kind: Config
        clusters:
        - name: local
          cluster:
            server: http://localhost:8080
        users:
        - name: kubelet
        contexts:
        - context:
            cluster: local
            user: kubelet
          name: kubelet-context
        current-context: kubelet-context

{{ if not .UseCalico }}
  - path: /etc/kubernetes/cni/net.d/10-flannel.conf
    content: |
        {
            "name": "podnet",
            "type": "flannel",
            "delegate": {
                "isDefaultGateway": true
            }
        }

{{ else }}

  - path: /etc/kubernetes/cni/net.d/10-calico.conf
    content: |
      {
        "name": "calico",
        "type": "flannel",
        "delegate": {
          "type": "calico",
          "etcd_endpoints": "#ETCD_ENDPOINTS#",
          "etcd_key_file": "/etc/kubernetes/ssl/etcd-client-key.pem",
          "etcd_cert_file": "/etc/kubernetes/ssl/etcd-client.pem",
          "etcd_ca_cert_file": "/etc/kubernetes/ssl/etcd-trusted-ca.pem",
          "log_level": "info",
          "policy": {
            "type": "k8s",
            "k8s_api_root": "http://127.0.0.1:8080/api/v1/"
          }
        }
      }

{{ end }}

# AdvancedAuditing is enabled by default since K8S v1.8.
# With AdvancedAuditing, you have to provide a audit policy file.
# Otherwise no audit logs are recorded at all.
{{if .Experimental.AuditLog.Enabled -}}
  # Refer to the audit profile used by GCE
  # https://github.com/kubernetes/kubernetes/blob/v1.8.3/cluster/gce/gci/configure-helper.sh#L517
  - path: /etc/kubernetes/apiserver/audit-policy.yaml
    owner: root:root
    permissions: 0600
    content: |
      apiVersion: audit.k8s.io/v1beta1
      kind: Policy
      rules:
        # The following requests were manually identified as high-volume and low-risk,
        # so drop them.
        - level: None
          users: ["system:kube-proxy"]
          verbs: ["watch"]
          resources:
            - group: "" # core
              resources: ["endpoints", "services", "services/status"]
        - level: None
          # Ingress controller reads ` + "`configmaps/ingress-uid`" + ` through the unsecured port.
          # TODO(#46983): Change this to the ingress controller service account.
          users: ["system:unsecured"]
          namespaces: ["kube-system"]
          verbs: ["get"]
          resources:
            - group: "" # core
              resources: ["configmaps"]
        - level: None
          users: ["kubelet"] # legacy kubelet identity
          verbs: ["get"]
          resources:
            - group: "" # core
              resources: ["nodes", "nodes/status"]
        - level: None
          userGroups: ["system:nodes"]
          verbs: ["get"]
          resources:
            - group: "" # core
              resources: ["nodes", "nodes/status"]
        - level: None
          users:
            - system:kube-controller-manager
            - system:kube-scheduler
            - system:serviceaccount:kube-system:endpoint-controller
          verbs: ["get", "update"]
          namespaces: ["kube-system"]
          resources:
            - group: "" # core
              resources: ["endpoints"]
        - level: None
          users: ["system:apiserver"]
          verbs: ["get"]
          resources:
            - group: "" # core
              resources: ["namespaces", "namespaces/status", "namespaces/finalize"]
        # Don't log HPA fetching metrics.
        - level: None
          users:
            - system:kube-controller-manager
          verbs: ["get", "list"]
          resources:
            - group: "metrics.k8s.io"
        # Don't log these read-only URLs.
        - level: None
          nonResourceURLs:
            - /healthz*
            - /version
            - /swagger*
        # Don't log events requests.
        - level: None
          resources:
            - group: "" # core
              resources: ["events"]
        # Secrets, ConfigMaps, and TokenReviews can contain sensitive & binary data,
        # so only log at the Metadata level.
        - level: Metadata
          resources:
            - group: "" # core
              resources: ["secrets", "configmaps"]
            - group: authentication.k8s.io
              resources: ["tokenreviews"]
          omitStages:
            - "RequestReceived"
        # Get responses can be large; skip them.
        - level: Request
          verbs: ["get", "list", "watch"]
          resources:
            - group: "" # core
            - group: "admissionregistration.k8s.io"
            - group: "apiextensions.k8s.io"
            - group: "apiregistration.k8s.io"
            - group: "apps"
            - group: "authentication.k8s.io"
            - group: "authorization.k8s.io"
            - group: "autoscaling"
            - group: "batch"
            - group: "certificates.k8s.io"
            - group: "extensions"
            - group: "metrics.k8s.io"
            - group: "networking.k8s.io"
            - group: "policy"
            - group: "rbac.authorization.k8s.io"
            - group: "settings.k8s.io"
            - group: "storage.k8s.io"
          omitStages:
            - "RequestReceived"
        # Default level for known APIs
        - level: RequestResponse
          resources:
            - group: "" # core
            - group: "admissionregistration.k8s.io"
            - group: "apiextensions.k8s.io"
            - group: "apiregistration.k8s.io"
            - group: "apps"
            - group: "authentication.k8s.io"
            - group: "authorization.k8s.io"
            - group: "autoscaling"
            - group: "batch"
            - group: "certificates.k8s.io"
            - group: "extensions"
            - group: "metrics.k8s.io"
            - group: "networking.k8s.io"
            - group: "policy"
            - group: "rbac.authorization.k8s.io"
            - group: "settings.k8s.io"
            - group: "storage.k8s.io"
          omitStages:
            - "RequestReceived"
        # Default level for all other requests.
        - level: Metadata
          omitStages:
            - "RequestReceived"
{{ end -}}

{{if .Experimental.Authentication.Webhook.Enabled}}
  - path: /etc/kubernetes/webhooks/authentication.yaml
    encoding: base64
    content: {{ .Experimental.Authentication.Webhook.Config }}
{{ end }}

{{ if .SharedPersistentVolume }}
  - path: /opt/bin/load-efs-pv
    owner: root:root
    permissions: 0700
    content: |
      #!/bin/bash -e

      docker run --rm --net=host \
        -v /etc/kubernetes:/etc/kubernetes \
        -v /etc/resolv.conf:/etc/resolv.conf \
        {{ .HyperkubeImage.RepoWithTag }} /bin/bash \
          -vxec \
          'echo "Starting Loading EFS Persistent Volume"
           /kubectl create -f /etc/kubernetes/efs-pv.yaml
           echo "Finished Loading EFS Persistent Volume"'

{{ end }}
{{if .Experimental.Kube2IamSupport.Enabled }}
  - path: /srv/kubernetes/manifests/kube2iam-ds.yaml
    content: |
      apiVersion: extensions/v1beta1
      kind: DaemonSet
      metadata:
        name: kube2iam
        namespace: kube-system
        labels:
          app: kube2iam
        annotations:
          scheduler.alpha.kubernetes.io/critical-pod: ''
      spec:
        updateStrategy:
          type: RollingUpdate
        template:
          metadata:
            labels:
              name: kube2iam
          spec:
            {{if .Experimental.Admission.Priority.Enabled -}}
            priorityClassName: system-node-critical
            {{ end -}}
            serviceAccountName: kube2iam
            hostNetwork: true
            tolerations:
            - operator: Exists
              effect: NoSchedule
            - operator: Exists
              effect: NoExecute
            - operator: Exists
              key: CriticalAddonsOnly
            containers:
              - image: {{.Kube2IAMImage.RepoWithTag}}
                name: kube2iam
                args:
                  - "--app-port=8282"
                  - "--auto-discover-base-arn"
                  - "--auto-discover-default-role"
                  - "--iptables=true"
                  - "--host-ip=$(HOST_IP)"
           {{- if .UseCalico }}
                  - "--host-interface=cali+"
           {{else}}
                  - "--host-interface=cni0"
           {{- end }}
                env:
                  - name: HOST_IP
                    valueFrom:
                      fieldRef:
                        fieldPath: status.podIP
                ports:
                  - containerPort: 8282
                    hostPort: 8282
                    name: http
                resources:
                  limits:
                    cpu: 10m
                    memory: 32Mi
                  requests:
                    cpu: 10m
                    memory: 32Mi
                securityContext:
                  privileged: true
  - path: /srv/kubernetes/manifests/kube2iam-rbac.yaml
    content: |
      apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: kube2iam
        namespace: kube-system
      ---

      apiVersion: rbac.authorization.k8s.io/v1
      kind: ClusterRole
      metadata:
        annotations:
          rbac.authorization.kubernetes.io/autoupdate: "true"
        labels:
          kubernetes.io/bootstrapping: kube2iam
        name: kube2iam
      rules:
      - apiGroups:
        - ""
        resources:
        - pods
        - namespaces
        verbs:
        - get
        - list
        - watch
      ---

      apiVersion: rbac.authorization.k8s.io/v1
      kind: ClusterRoleBinding
      metadata:
        name: kube2iam
      roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: ClusterRole
        name: kube2iam
      subjects:
      - kind: ServiceAccount
        name: kube2iam
        namespace: kube-system
{{end}}

{{if .Experimental.KIAMSupport.Enabled }}
  - path: /etc/kubernetes/ssl/kiam/ca.pem
    encoding: gzip+base64
    content: {{.AssetsConfig.KIAMCACert}}

  - path: /etc/kubernetes/ssl/kiam/server-key.pem{{if .AssetsEncryptionEnabled}}.enc{{end}}
    encoding: gzip+base64
    content: {{.AssetsConfig.KIAMServerKey}}

  - path: /etc/kubernetes/ssl/kiam/server.pem
    encoding: gzip+base64
    content: {{.AssetsConfig.KIAMServerCert}}

  - path: /etc/kubernetes/ssl/kiam/agent-key.pem{{if .AssetsEncryptionEnabled}}.enc{{end}}
    encoding: gzip+base64
    content: {{.AssetsConfig.KIAMAgentKey}}

  - path: /etc/kubernetes/ssl/kiam/agent.pem
    encoding: gzip+base64
    content: {{.AssetsConfig.KIAMAgentCert}}

  # Without the namespace annotation the pod will be unable to assume any roles
  - path: /srv/kubernetes/manifests/kube-system-ns.yaml
    content: |
      apiVersion: v1
      kind: Namespace
      metadata:
        name: kube-system
        annotations:
          iam.amazonaws.com/permitted: ".*"
  - path: /srv/kubernetes/manifests/kiam-all.yaml
    content: |
      apiVersion: extensions/v1beta1
      kind: DaemonSet
      metadata:
        namespace: kube-system
        name: kiam-server
      spec:
        updateStrategy:
          type: RollingUpdate
        template:
          metadata:
            annotations:
              prometheus.io/scrape: "true"
              prometheus.io/port: "9620"
            labels:
              app: kiam
              role: server
          spec:
            {{if .Experimental.Admission.Priority.Enabled -}}
            priorityClassName: system-node-critical
            {{ end -}}
            tolerations:
            - operator: Exists
              effect: NoSchedule
            - operator: Exists
              effect: NoExecute
            - operator: Exists
              key: CriticalAddonsOnly
            serviceAccountName: kiam-server
            nodeSelector:
              node-role.kubernetes.io/master: ""
            volumes:
              - name: ssl-certs
                hostPath:
                  path: /usr/share/ca-certificates
              - name: tls
                secret:
                  secretName: kiam-server-tls
            containers:
              - name: kiam
                image: quay.io/uswitch/kiam:v2.5
                imagePullPolicy: Always
                command:
                  - /server
                args:
                  - --json-log
                  - --bind=0.0.0.0:443
                  - --cert=/etc/kiam/tls/server.pem
                  - --key=/etc/kiam/tls/server-key.pem
                  - --ca=/etc/kiam/tls/ca.pem
                  - --role-base-arn-autodetect
                  - --sync=1m
                  - --prometheus-listen-addr=0.0.0.0:9620
                  - --prometheus-sync-interval=5s
                volumeMounts:
                  - mountPath: /etc/ssl/certs
                    name: ssl-certs
                  - mountPath: /etc/kiam/tls
                    name: tls
                livenessProbe:
                  exec:
                    command:
                    - /health
                    - --cert=/etc/kiam/tls/server.pem
                    - --key=/etc/kiam/tls/server-key.pem
                    - --ca=/etc/kiam/tls/ca.pem
                    - --server-address=localhost:443
                    - --server-address-refresh=2s
                    - --timeout=5s
                  initialDelaySeconds: 10
                  periodSeconds: 10
                  timeoutSeconds: 10
                readinessProbe:
                  exec:
                    command:
                    - /health
                    - --cert=/etc/kiam/tls/server.pem
                    - --key=/etc/kiam/tls/server-key.pem
                    - --ca=/etc/kiam/tls/ca.pem
                    - --server-address=localhost:443
                    - --server-address-refresh=2s
                    - --timeout=5s
                  initialDelaySeconds: 3
                  periodSeconds: 10
                  timeoutSeconds: 10
      ---
      apiVersion: v1
      kind: Service
      metadata:
        name: kiam-server
        namespace: kube-system
      spec:
        clusterIP: None
        selector:
          app: kiam
          role: server
        ports:
        - name: grpc
          port: 443
          targetPort: 443
          protocol: TCP
      ---
      ---
      kind: ServiceAccount
      apiVersion: v1
      metadata:
        name: kiam-server
        namespace: kube-system
      ---
      apiVersion: rbac.authorization.k8s.io/v1beta1
      kind: ClusterRole
      metadata:
        name: kiam-read
      rules:
      - apiGroups:
        - ""
        resources:
        - namespaces
        - pods
        verbs:
        - watch
        - get
        - list
      ---
      apiVersion: rbac.authorization.k8s.io/v1beta1
      kind: ClusterRoleBinding
      metadata:
        name: kiam-server
      roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: ClusterRole
        name: kiam-read
      subjects:
      - kind: ServiceAccount
        name: kiam-server
        namespace: kube-system
      ---
      apiVersion: extensions/v1beta1
      kind: DaemonSet
      metadata:
        namespace: kube-system
        name: kiam-agent
      spec:
        template:
          metadata:
            annotations:
              prometheus.io/scrape: "true"
              prometheus.io/port: "9620"
            labels:
              app: kiam
              role: agent
          spec:
            {{if .Experimental.Admission.Priority.Enabled -}}
            priorityClassName: system-node-critical
            {{ end -}}
            tolerations:
            - operator: Exists
              effect: NoSchedule
            - operator: Exists
              effect: NoExecute
            - operator: Exists
              key: CriticalAddonsOnly
            hostNetwork: true
            dnsPolicy: ClusterFirstWithHostNet
            nodeSelector:
              kubernetes.io/role: node
            volumes:
              - name: ssl-certs
                hostPath:
                  path: /usr/share/ca-certificates
              - name: tls
                secret:
                  secretName: kiam-agent-tls
              - name: xtables
                hostPath:
                  path: /run/xtables.lock
            containers:
              - name: kiam
                securityContext:
                  privileged: true
                image: {{.KIAMImage.RepoWithTag}}
                command:
                  - /agent
                args:
                  - --iptables
           {{- if .UseCalico }}
                  - "--host-interface=cali+"
           {{else}}
                  - "--host-interface=cni0"
           {{- end }}
                  - --json-log
                  - --port=8181
                  - --cert=/etc/kiam/tls/agent.pem
                  - --key=/etc/kiam/tls/agent-key.pem
                  - --ca=/etc/kiam/tls/ca.pem
                  - --server-address=kiam-server:443
                  - --prometheus-listen-addr=0.0.0.0:9620
                  - --prometheus-sync-interval=5s
                env:
                  - name: HOST_IP
                    valueFrom:
                      fieldRef:
                        fieldPath: status.podIP
                volumeMounts:
                  - mountPath: /etc/ssl/certs
                    name: ssl-certs
                  - mountPath: /etc/kiam/tls
                    name: tls
                  - mountPath: /var/run/xtables.lock
                    name: xtables
                livenessProbe:
                  httpGet:
                    path: /ping
                    port: 8181
                  initialDelaySeconds: 3
                  periodSeconds: 3
{{end}}

  - path: /opt/bin/retry
    owner: root:root
    permissions: 0755
    content: |
      #!/bin/bash
      max_attempts="$1"; shift
      cmd="$@"
      attempt_num=1
      attempt_interval_sec=3

      until $cmd
      do
          if (( attempt_num == max_attempts ))
          then
              echo "Attempt $attempt_num failed and there are no more attempts left!"
              return 1
          else
              echo "Attempt $attempt_num failed! Trying again in $attempt_interval_sec seconds..."
              ((attempt_num++))
              sleep $attempt_interval_sec;
          fi
      done

  {{if .Experimental.AuditLog.Enabled -}}
  # Check worker communication by searching audit logs
  - path: /opt/bin/check-worker-communication
    permissions: 0700
    owner: root:root
    content: |
      #!/bin/bash -e
      set -ue

      AUDIT_LOG_PATH="{{.Experimental.AuditLog.LogPath}}"

      kubectl() {
        /usr/bin/docker run --rm --net=host -v /srv/kubernetes:/srv/kubernetes {{.HyperkubeImage.RepoWithTag}} /hyperkube kubectl "$@"
      }

      queryserver() {
        echo "Checking to see if workers are communicating with API server."
        auditlogs | grep -v 127.0.0.1 | grep kubelet | jq -c 'select(.responseStatus.code == 200)' --exit-status
      }

      auditlogs() {
        if [ "$AUDIT_LOG_PATH" == "/dev/stdout" ]; then
          # Let ` + "`docker logs`" + ` gather logs for periods slightly longer than the delay between ` + "`queryserver`" + ` calls
          # so that we won't drop any lines.
          docker logs --since 11s ${DOCKERIMAGE} |& cat
        else
          cat "$AUDIT_LOG_PATH"
        fi
      }

      #This checks whether there are any nodes other than controllers. If there is, this indicates it is a cluster update, if there is not, is a fresh cluster.
      #If it is an update, we check whether the workers can communicate with the API server before reporting success.

      kubectl get nodes 2>/dev/null
      RC=$?
      if [[ $RC -gt 0 ]]; then
        echo "No nodes present, assuming API server not yet ready, cannot verify cluster is up."
        exit 1
      fi

      NUM_WORKERS=$(kubectl get nodes -l node-role.kubernetes.io/master!="" 2>/dev/null)
      if [[ -z ${NUM_WORKERS} ]]; then
        echo "Fresh cluster, not checking for existing workers."
        exit 0
      else
        #We first retrieve the name of the image which is running the api-server, and then search its logs for any mentions of kubelet with a 200 response.

        DOCKERIMAGE=$(docker ps | grep -i k8s_kube-apiserver_kube-apiserver | awk '{print $1}')
        until queryserver; do echo "Worker communication failed, retrying." && sleep 10; done
        echo "Communication with workers has been established."
      fi
  {{end -}}

{{ end }}`)
