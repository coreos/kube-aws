# Deprecated Cloud-Config Features

---

**NOTE**: coreos-cloudinit is no longer under active development and has been superseded by [Ignition][ignition]. For more information about the recommended tools for provisioning Container Linux, refer to the [provisioning documentation][provisioning].

[ignition]: https://github.com/coreos/ignition
[provisioning]: https://github.com/coreos/docs/blob/master/os/provisioning.md

---

## Retrieving SSH Authorized Keys

### From a GitHub User

Using the `coreos-ssh-import-github` field, we can import public SSH keys from a GitHub user to use as authorized keys to a server.

```yaml
#cloud-config

users:
  - name: elroy
    coreos-ssh-import-github: elroy
```

### From an HTTP Endpoint

We can also pull public SSH keys from any HTTP endpoint which matches [GitHub's API response format](https://developer.github.com/v3/users/keys/#list-public-keys-for-a-user).
For example, if you have an installation of GitHub Enterprise, you can provide a complete URL with an authentication token:

```yaml
#cloud-config

users:
  - name: elroy
    coreos-ssh-import-url: https://github-enterprise.example.com/api/v3/users/elroy/keys?access_token=<TOKEN>
```

You can also specify any URL whose response matches the JSON format for public keys:

```yaml
#cloud-config

users:
  - name: elroy
    coreos-ssh-import-url: https://example.com/public-keys
```
