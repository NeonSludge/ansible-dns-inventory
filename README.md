# ansible-dns-inventory

A [dynamic inventory](https://docs.ansible.com/ansible/latest/user_guide/intro_dynamic_inventory.html) script for Ansible that discovers hosts and groups via a DNS zone transfer and organizes them into a tree.

This utility uses a DNS server as a single-source-of-truth for your Ansible inventories. It extracts host attributes from corresponding DNS TXT records and builds a tree out of them that then gets exported into a JSON representation, ready for use by Ansible.

For this to work you must ensure that:

1. Your DNS server allows zone transfers (AXFR) to the host that is going to be running `ansible-dns-inventory` (Ansible control node).
2. Every host that should be managed by Ansible has a properly formatted DNS TXT record.
3. You have created a configuration file for `ansible-dns-inventory`.

### TXT record format
For a host to appear in `ansible-dns-inventory`'s output its DNS TXT record should contain several attributes formatted as a set of key/value pairs.

#### Example of a TXT record
```
OS=linux;ENV=dev;ROLE=app;SRV=tomcat_backend_auth
```

#### Host attributes (default key names)
| Key  | Description                                                |
| ---- | ---------------------------------------------------------- |
| OS   | Operating system identifier.                               |
| ENV  | Environment identifier.                                    |
| ROLE | Host role identifier(s). Can be a comma-delimited list.    |
| SRV  | Host service identifier(s). Can be a comma-delimited list. |

Key names and separators are customizable via `ansible-dns-inventory`'s config file.
If a host has several TXT records, the last one wins. So if you have other stuff you would like to put in there, make sure that the last TXT record is always exclusively meant for `ansible-dns-inventory`.

### Config file

`ansible-dns-inventory` uses an YAML configuration file. It looks for an `ansible-dns-inventory.yaml` file inside these directories (in this specific order):

* `.` (current working directory)
* `~/.ansible/`
* `/etc/ansible/`

There is a [template](config/ansible-dns-inventory.yaml) in this repository that has descriptions and default values for all available parameters.

#### Example of a config file
```
dns:
  server: "10.100.100.1:53"
  timeout: "120s"
  zones:
    - server.local.
    - infra.local.
txt:
  kv:
    separator: "|"
  keys:
    env: "PRJ"

```

### Inventory structure

