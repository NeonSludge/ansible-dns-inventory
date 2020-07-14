# ansible-dns-inventory

A [dynamic inventory](https://docs.ansible.com/ansible/latest/user_guide/intro_dynamic_inventory.html) script for Ansible that discovers hosts and groups via a DNS zone transfer and organizes them into a tree.

This utility uses a DNS server as a single-source-of-truth for your Ansible inventories. It extracts host attributes from corresponding DNS TXT records and builds a tree out of them that then gets exported into a JSON representation, ready for use by Ansible. A tree is often a very convenient way of organizing your inventory because it allows for a predictable variable merging/flattening order.

This dynamic inventory started as a Bash script and has been used for a couple of years in environments ranging from tens to hundreds of hosts. I am publishing this Golang version in hopes that someone else finds it useful.

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
| Key  | Description                                                                                                                                                 |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| OS   | Operating system identifier.                                                                                                                                |
| ENV  | Environment identifier.                                                                                                                                     |
| ROLE | Host role identifier(s). Can be a comma-delimited list.                                                                                                     |
| SRV  | Host service identifier(s). This will be split further using the `txt.keys.separator` to produce a hierarchy of groups. Can also be a comma-delimited list. |

Key names and separators are customizable via `ansible-dns-inventory`'s config file.
If a host has several TXT records, the first one wins. So if you have other stuff you would like to put in there, make sure that the first TXT record returned by your DNS server for a given host is always exclusively meant for `ansible-dns-inventory`.

### Config file

`ansible-dns-inventory` uses a YAML configuration file. It looks for an `ansible-dns-inventory.yaml` file inside these directories (in this specific order):

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

In general, if you have a single TXT record for a `HOST` and this record has all 4 attributes set then this `HOST` will end up in this hierarchy of groups:

```
@all:
  |--@all_<ROLE>:
  |  |--@all_<ROLE>_<SRV[1]>:
  |  |  |--<HOST>
  |--@all_host:
  |  |--@all_host_<OS>:
  |  |  |--<HOST>
  |--@<ENV>:
  |  |--@<ENV>_<ROLE>:
  |  |  |--@<ENV>_<ROLE>_<SRV[1]>:
  |  |  |  |--@<ENV>_<ROLE>_<SRV[1]>_<SRV[2]>:
  |  |  |  |  |--@<ENV>_<ROLE>_<SRV[1]>_<SRV[2]>_..._<SRV[n]>:
  |  |  |  |  |  |--<HOST>
  |  |--@<ENV>_host:
  |  |  |--@<ENV>_host_<OS>:
  |  |  |  |--<HOST>
```

Let's say you have these records in your DNS server:

| Host                | TXT record                                            |
| ------------------- | ----------------------------------------------------- |
| `app01.infra.local` | `OS=linux;ENV=dev;ROLE=app;SRV=tomcat_backend_auth`   |
| `app02.infra.local` | `OS=linux;ENV=dev;ROLE=app;SRV=tomcat_backend_auth`   |
| `app03.infra.local` | `OS=linux;ENV=dev;ROLE=app;SRV=tomcat_backend_media`  |

These will produce the following Ansible inventory tree:

```
@all:
  |--@all_app:
  |  |--@all_app_tomcat:
  |  |  |--app01.infra.local
  |  |  |--app02.infra.local
  |  |  |--app03.infra.local
  |--@all_host:
  |  |--@all_host_linux:
  |  |  |--app01.infra.local
  |  |  |--app02.infra.local
  |  |  |--app03.infra.local
  |--@dev:
  |  |--@dev_app:
  |  |  |--@dev_app_tomcat:
  |  |  |  |--@dev_app_tomcat_backend:
  |  |  |  |  |--@dev_app_tomcat_backend_auth:
  |  |  |  |  |  |--app01.infra.local
  |  |  |  |  |  |--app02.infra.local
  |  |  |  |  |--@dev_app_tomcat_backend_media:
  |  |  |  |  |  |--app03.infra.local
  |  |--@dev_host:
  |  |  |--@dev_host_linux:
  |  |  |  |--app01.infra.local
  |  |  |  |--app02.infra.local
  |  |  |  |--app03.infra.local
  |--@ungrouped:
```
