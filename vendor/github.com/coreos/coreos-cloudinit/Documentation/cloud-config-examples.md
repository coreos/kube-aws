# Cloud-Config Examples

---

**NOTE**: coreos-cloudinit is no longer under active development and has been superseded by [Ignition][ignition]. For more information about the recommended tools for provisioning Container Linux, refer to the [provisioning documentation][provisioning].

[ignition]: https://github.com/coreos/ignition
[provisioning]: https://github.com/coreos/docs/blob/master/os/provisioning.md

---

This document contains commonly useful cloud-config examples, ranging from
common tasks like configuring flannel to useful tricks like setting the
timezone.

## etcd

### Configuring TLS encryption

Cloud-config has a parameter that will place the contents of a file on disk. We're going to use this to add our drop-in unit to configure etcd as well as the certificate files.

```cloud-config
#cloud-config

coreos:
  units:
    - name: etcd2.service
      drop-ins:
        - name: 30-certificates.conf
          content: |
            [Service]
            # Client Env Vars
            Environment=ETCD_CA_FILE=/path/to/CA.pem
            Environment=ETCD_CERT_FILE=/path/to/server.crt
            Environment=ETCD_KEY_FILE=/path/to/server.key
            # Peer Env Vars
            Environment=ETCD_PEER_CA_FILE=/path/to/CA.pem
            Environment=ETCD_PEER_CERT_FILE=/path/to/peers.crt
            Environment=ETCD_PEER_KEY_FILE=/path/to/peers.key
      command: start

write_files:
  - path: /path/to/CA.pem
    permissions: 0644
    content: |
      -----BEGIN CERTIFICATE-----
      MIIFNDCCAx6gAwIBAgIBATALBgkqhkiG9w0BAQUwLTEMMAoGA1UEBhMDVVNBMRAw
      ...snip...
      EtHaxYQRy72yZrte6Ypw57xPRB8sw1DIYjr821Lw05DrLuBYcbyclg==
      -----END CERTIFICATE-----
  - path: /path/to/server.crt
    permissions: 0644
    content: |
      -----BEGIN CERTIFICATE-----
      MIIFWTCCA0OgAwIBAgIBAjALBgkqhkiG9w0BAQUwLTEMMAoGA1UEBhMDVVNBMRAw
      DgYDVQQKEwdldGNkLWNhMQswCQYDVQQLEwJDQTAeFw0xNDA1MjEyMTQ0MjhaFw0y
      ...snip...
      rdmtCVLOyo2wz/UTzvo7UpuxRrnizBHpytE4u0KgifGp1OOKY+1Lx8XSH7jJIaZB
      a3m12FMs3AsSt7mzyZk+bH2WjZLrlUXyrvprI40=
      -----END CERTIFICATE-----
  - path: /path/to/server.key
    permissions: 0644
    content: |
      -----BEGIN RSA PRIVATE KEY-----
      Proc-Type: 4,ENCRYPTED
      DEK-Info: DES-EDE3-CBC,069abc493cd8bda6

      TBX9mCqvzNMWZN6YQKR2cFxYISFreNk5Q938s5YClnCWz3B6KfwCZtjMlbdqAakj
      ...snip...
      mgVh2LBerGMbsdsTQ268sDvHKTdD9MDAunZlQIgO2zotARY02MLV/Q5erASYdCxk
      -----END RSA PRIVATE KEY-----
  - path: /path/to/peers.crt
    permissions: 0644
    content: |
      -----BEGIN CERTIFICATE-----
      VQQLEwJDQTAeFw0xNDA1MjEyMTQ0MjhaFw0yMIIFWTCCA0OgAwIBAgIBAjALBgkq
      DgYDVQQKEwdldGNkLWNhMQswCQYDhkiG9w0BAQUwLTEMMAoGA1UEBhMDVVNBMRAw
      ...snip...
      BHpytE4u0KgifGp1OOKY+1Lx8XSH7jJIaZBrdmtCVLOyo2wz/UTzvo7UpuxRrniz
      St7mza3m12FMs3AsyZk+bH2WjZLrlUXyrvprI90=
      -----END CERTIFICATE-----
  - path: /path/to/peers.key
    permissions: 0644
    content: |
      -----BEGIN RSA PRIVATE KEY-----
      Proc-Type: 4,ENCRYPTED
      DEK-Info: DES-EDE3-CBC,069abc493cd8bda6

      SFreNk5Q938s5YTBX9mCqvzNMWZN6YQKR2cFxYIClnCWz3B6KfwCZtjMlbdqAakj
      ...snip...
      DvHKTdD9MDAunZlQIgO2zotmgVh2LBerGMbsdsTQ268sARY02MLV/Q5erASYdCxk
      -----END RSA PRIVATE KEY-----
```

## Docker

### Enabling the remote API

To enable the remote API on every Container Linux machine in a cluster, we need to provide the new socket file, and Docker's socket activation support will automatically start using the socket

```cloud-config
#cloud-config

coreos:
  units:
    - name: docker-tcp.socket
      command: start
      enable: true
      content: |
        [Unit]
        Description=Docker Socket for the API

        [Socket]
        ListenStream=2375
        BindIPv6Only=both
        Service=docker.service

        [Install]
        WantedBy=sockets.target
```

To keep access to the port local, replace the `ListenStream` configuration above with:

```cloud-config
        [Socket]
        ListenStream=127.0.0.1:2375

```

### Enable the remote API with TLS authentication

Docker TLS configuration consists of three parts: keys creation, configuring new systemd socket unit and systemd drop-in configuration.

```cloud-config
#cloud-config

write_files:
    - path: /etc/docker/ca.pem
      permissions: 0644
      content: |
        -----BEGIN CERTIFICATE-----
        MIIFNDCCAx6gAwIBAgIBATALBgkqhkiG9w0BAQswLTEMMAoGA1UEBhMDVVNBMRAw
        DgYDVQQKEwdldGNkLWNhMQswCQYDVQQLEwJDQTAeFw0xNTA5MDIxMDExMDhaFw0y
        NTA5MDIxMDExMThaMC0xDDAKBgNVBAYTA1VTQTEQMA4GA1UEChMHZXRjZC1jYTEL
        ... ... ...
    - path: /etc/docker/server.pem
      permissions: 0644
      content: |
        -----BEGIN CERTIFICATE-----
        MIIFajCCA1SgAwIBAgIBBTALBgkqhkiG9w0BAQswLTEMMAoGA1UEBhMDVVNBMRAw
        DgYDVQQKEwdldGNkLWNhMQswCQYDVQQLEwJDQTAeFw0xNTA5MDIxMDM3MDFaFw0y
        NTA5MDIxMDM3MDNaMEQxDDAKBgNVBAYTA1VTQTEQMA4GA1UEChMHZXRjZC1jYTEQ
        ... ... ...
    - path: /etc/docker/server-key.pem
      permissions: 0600
      content: |
        -----BEGIN RSA PRIVATE KEY-----
        MIIJKAIBAAKCAgEA23Q4yELhNEywScrHl6+MUtbonCu59LIjpxDMAGxAHvWhWpEY
        P5vfas8KgxxNyR+U8VpIjEXvwnhwCx/CSCJc3/VtU9v011Ir0WtTrNDocb90fIr3
        YeRWq744UJpBeDHPV9opf8xFE7F74zWeTVMwtiMPKcQDzZ7XoNyJMxg1wmiMbdCj
        ... ... ...
coreos:
  units:
    - name: docker-tls-tcp.socket
      command: start
      enable: true
      content: |
        [Unit]
        Description=Docker Secured Socket for the API

        [Socket]
        ListenStream=2376
        BindIPv6Only=both
        Service=docker.service

        [Install]
        WantedBy=sockets.target
    - name: docker.service
      drop-ins:
        - name: 10-tls-verify.conf
          content: |
            [Service]
            Environment="DOCKER_OPTS=--tlsverify --tlscacert=/etc/docker/ca.pem --tlscert=/etc/docker/server.pem --tlskey=/etc/docker/server-key.pem"
```

### Enabling the Docker debug flag

```cloud-config
#cloud-config

coreos:
  units:
    - name: docker.service
      drop-ins:
        - name: 10-debug.conf
          content: |
            [Service]
            Environment=DOCKER_OPTS=--debug
      command: restart
```

### Use an HTTP proxy

```cloud-config
#cloud-config

coreos:
  units:
    - name: docker.service
      drop-ins:
        - name: 20-http-proxy.conf
          content: |
            [Service]
            Environment="HTTP_PROXY=http://proxy.example.com:8080"
      command: restart
```

### Increase ulimits

```cloud-config
#cloud-config

coreos:
  units:
    - name: docker.service
      drop-ins:
        - name: 30-increase-ulimit.conf
          content: |
            [Service]
            LimitMEMLOCK=infinity
      command: restart
```

## Flannel

### Setting flannel's configuration

Flannel looks up its configuration in etcd, so it can be useful to pre-populate this value before flannel starts. Here is an example of doing this with a minimum flannel configuration:

```yaml
#cloud-config
coreos:
  units:
    - name: flanneld.service
      drop-ins:
        - name: 50-network-config.conf
          content: |
            [Service]
            ExecStartPre=/usr/bin/etcdctl set /coreos.com/network/config '{ "Network": "10.1.0.0/16" }'
```

## Time

### Setting the timezone

This cloud config adds a systemd unit to set the timezone to `America/New_York`. You can find a list of all supported timezones [here](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).

```cloud-config
#cloud-config
coreos:
  units:
    - name: settimezone.service
      command: start
      content: |
        [Unit]
        Description=Set the time zone

        [Service]
        ExecStart=/usr/bin/timedatectl set-timezone America/New_York
        RemainAfterExit=yes
        Type=oneshot
```

### Setting an NTP source

```cloud-config
#cloud-config
write_files:
  - path: /etc/systemd/timesyncd.conf
    content: |
      [Time]
      NTP=0.pool.example.com 1.pool.example.com
```

### Using ntpd instead of timesyncd

```cloud-config
#cloud-config
coreos:
  units:
    - name: systemd-timesyncd.service
      command: stop
      mask: true
    - name: ntpd.service
      command: start
      enable: true
```

### Configuring `ntpd`

```cloud-config
#cloud-config
write_files:
  - path: /etc/ntp.conf
    content: |
      server 0.pool.example.com
      server 1.pool.example.com

      # - Allow only time queries, at a limited rate.
      # - Allow all local queries (IPv4, IPv6)
      restrict default nomodify nopeer noquery limited kod
      restrict 127.0.0.1
      restrict [::1]
```

## Configuring sshd

In this example we will disable logins for the root user, only allow login for the core user and disable password based authentication.

```cloud-config
#cloud-config

write_files:
  - path: /etc/ssh/sshd_config
    permissions: 0600
    owner: root:root
    content: |
      # Use most defaults for sshd configuration.
      UsePrivilegeSeparation sandbox
      Subsystem sftp internal-sftp

      PermitRootLogin no
      AllowUsers core
      PasswordAuthentication no
      ChallengeResponseAuthentication no
```

### Changing the port

```cloud-config
#cloud-config

coreos:
  units:
  - name: sshd.socket
    command: restart
    runtime: true
    content: |
      [Socket]
      ListenStream=2222
      FreeBind=true
      Accept=yes
```

### Override socket-activated SSH

Occasionally when systemd gets into a broken state, socket activation doesn't work, which can make a system inaccessible if ssh is the only option. This can be avoided configuring a permanently active SSH daemon that forks for each incoming connection.

```cloud-config
#cloud-config

coreos:
  units:
  - name: sshd.socket
    command: stop
    mask: true

  - name: sshd.service
    command: start
    content: |
      [Unit]
      Description=OpenSSH server daemon

      [Service]
      Type=forking
      PIDFile=/var/run/sshd.pid
      ExecStart=/usr/sbin/sshd
      ExecReload=/bin/kill -HUP $MAINPID
      KillMode=process
      Restart=on-failure
      RestartSec=30s

      [Install]
      WantedBy=multi-user.target

write_files:
  - path: "/var/run/sshd.pid"
    permissions: "0644"
    owner: "root"
```

## ISCSI

To configure and start iSCSI automatically after a machine is provisioned, credentials need to be written to disk and the iSCSI service started.

```cloud-config
#cloud-config
coreos:
    units:
        - name: "iscsid.service"
          command: "start"
write_files:
  - path: "/etc/iscsi/iscsid.conf"
    permissions: "0644"
    owner: "root"
    content: |
      isns.address = host_ip
      isns.port = host_port
      node.session.auth.username = my_username
      node.session.auth.password = my_secret_password
      discovery.sendtargets.auth.username = my_username
      discovery.sendtargets.auth.password = my_secret_password
```

## Networking

### Static networking

Setting up static networking in your cloud-config can be done by writing out the network unit. Be sure to modify the `[Match]` section with the name of your desired interface, and replace the IPs:

```cloud-config
#cloud-config

coreos:
  units:
    - name: 00-eth0.network
      runtime: true
      content: |
        [Match]
        Name=eth0

        [Network]
        DNS=1.2.3.4
        Address=10.0.0.101/24
        Gateway=10.0.0.1
```

### networkd and bond0 with cloud-init

By default, the kernel creates a `bond0` network device as soon as the `bonding` module is loaded by coreos-cloudinit. The device is created with default bonding options, such as "round-robin" mode. This leads to confusing behavior with `systemd-networkd` since networkd does not alter options of an existing network device.

You have three options:

- Use Ignition to configure your network
- Name your bond something other than `bond0`, or
- Prevent the kernel from automatically creating `bond0`.

To defer creating `bond0`, add to your cloud-config before any other network configuration:

```cloud-config
#cloud-config

write_files:
  - path: /etc/modprobe.d/bonding.conf
    content: |
      # Prevent kernel from automatically creating bond0 when the module is loaded.
      # This allows systemd-networkd to create and apply options to bond0.
      options bonding max_bonds=0
  - path: /etc/systemd/network/10-eth.network
    permissions: 0644
    owner: root
    content: |
      [Match]
      Name=eth*

      [Network]
      Bond=bond0
  - path: /etc/systemd/network/20-bond.netdev
    permissions: 0644
    owner: root
    content: |
      [NetDev]
      Name=bond0
      Kind=bond

      [Bond]
      Mode=0 # defaults to balance-rr
      MIIMonitorSec=1
  - path: /etc/systemd/network/30-bond-dhcp.network
    permissions: 0644
    owner: root
    content: |
      [Match]
      Name=bond0

      [Network]
      DHCP=ipv4
coreos:
  units:
    - name: down-interfaces.service
      command: start
      content: |
        [Service]
        Type=oneshot
        ExecStart=/usr/bin/ip link set eth0 down
        ExecStart=/usr/bin/ip addr flush dev eth0
        ExecStart=/usr/bin/ip link set eth1 down
        ExecStart=/usr/bin/ip addr flush dev eth1
    - name: systemd-networkd.service
      command: restart
```

### networkd and DHCP behavior with cloud-init

By default, even if you've already set a static IP address and you have a working DHCP server in your network, systemd-networkd will nevertheless assign IP address using DHCP. If you would like to remove this address, you have to use the following cloud-config example:

```cloud-config
#cloud-config

coreos:
  units:
    - name: systemd-networkd.service
      command: stop
    - name: 00-eth0.network
      runtime: true
      content: |
        [Match]
        Name=eth0

        [Network]
        DNS=1.2.3.4
        Address=10.0.0.101/24
        Gateway=10.0.0.1
    - name: down-interfaces.service
      command: start
      content: |
        [Service]
        Type=oneshot
        ExecStart=/usr/bin/ip link set eth0 down
        ExecStart=/usr/bin/ip addr flush dev eth0
    - name: systemd-networkd.service
      command: restart
```

### Configure static routes

```cloud-config
#cloud-config

coreos:
  units:
    - name: 10-static.network
      content: |
        [Route]
        Gateway=192.168.122.1
        Destination=172.16.0.0/24
```

## Loading kernel modules

These files are processed early during the boot sequence. This means that updating modules-load.d via cloud-config will only take effect on the next boot unless the systemd-modules-load service is also restarted:

```cloud-config
#cloud-config

write_files:
  - path: /etc/modules-load.d/nf.conf
    content: nf_conntrack

coreos:
  units:
    - name: systemd-modules-load.service
      command: restart
```

### Loading kernel modules with options

This example cloud-config excerpt loads the `dummy` network interface module with an option specifying the number of interfaces the module should create when loaded `(numdummies=5)`:

```cloud-config
#cloud-config

write_files:
  - path: /etc/modprobe.d/dummy.conf
    content: options dummy numdummies=5
  - path: /etc/modules-load.d/dummy.conf
    content: dummy

coreos:
  units:
    - name: systemd-modules-load.service
      command: restart
```
