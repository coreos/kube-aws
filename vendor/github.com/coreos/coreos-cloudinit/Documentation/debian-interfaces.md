# Debian Interfaces #

---

**NOTE**: coreos-cloudinit is no longer under active development and has been superseded by [Ignition][ignition]. For more information about the recommended tools for provisioning Container Linux, refer to the [provisioning documentation][provisioning].

[ignition]: https://github.com/coreos/ignition
[provisioning]: https://github.com/coreos/docs/blob/master/os/provisioning.md

---

**WARNING**: This option is EXPERIMENTAL and may change or be removed at any point.

There is basic support for converting from a Debian network configuration to networkd unit files. The -convert-netconf=debian option is used to activate this feature.

# convert-netconf #

Default: ""

Read the network config provided in cloud-drive and translate it from the specified format into networkd unit files (requires the -from-configdrive flag). Currently only supports "debian" which provides support for a small subset of the [Debian network configuration](https://wiki.debian.org/NetworkConfiguration). These options include:

- interface config methods
	- static
		- address/netmask
		- gateway
		- hwaddress
		- dns-nameservers
	- dhcp
		- hwaddress
	- manual
	- loopback
- vlan_raw_device
- bond-slaves
