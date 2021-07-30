# ansible-dns-inventory

[![Go Report Card](https://goreportcard.com/badge/github.com/NeonSludge/ansible-dns-inventory)](https://goreportcard.com/report/github.com/NeonSludge/ansible-dns-inventory)

A [dynamic inventory](https://docs.ansible.com/ansible/latest/user_guide/intro_dynamic_inventory.html) script for Ansible that discovers hosts and groups via a DNS request and organizes them into a tree.

This utility uses a DNS server as a single-source-of-truth for your Ansible inventories. It extracts host attributes from corresponding DNS TXT records and builds a tree out of them that then gets exported into a JSON representation, ready for use by Ansible. A tree is often a very convenient way of organizing your inventory because it allows for a predictable variable merging/flattening order.

This dynamic inventory started as a Bash script and has been used for a couple of years in environments ranging from tens to hundreds of hosts. I am publishing this Golang version in hopes that someone else finds it useful.

For this to work you must ensure that:

1. Your DNS server allows zone transfers (AXFR) to the host that is going to be running `ansible-dns-inventory` (Ansible control node) OR you're using the no-transfer mode (the `dns.notransfer.enabled` parameter in the configuration).
2. Every host that should be managed by Ansible has one or more properly formatted DNS TXT records OR there is a set of TXT records belonging to a special host (the `dns.notransfer.host` parameter) AND you're using the no-transfer mode.
3. You have created a configuration file for `ansible-dns-inventory`.

## Usage
```
Usage of dns-inventory:
  -attrs
        export host attributes
  -format string
        select export format, if available (default "yaml")
  -groups
        export groups
  -host string
        produce a JSON dictionary of host variables for Ansible
  -hosts
        export hosts
  -list
        produce a JSON inventory for Ansible
  -tree
        export raw inventory tree
  -version
        display ansible-dns-inventory version and build info
```

## TXT record format
There are two ways to add a host to the inventory:

1. Create a DNS TXT record for this host and format it properly, specifying host attributes as a set of key/value pairs. One host can have an unlimited number of TXT records: all of them will be parsed by `ansible-dns-inventory`.
2. Enable the no-transfer mode, add a TXT record for the special host (`ansible-dns-inventory.your.domain` by default) and format it properly, referencing the host you want to add to your inventory and specifying its attributes as a set of key/value pairs. Again, one host can have any number of records here.

Here is an example of using both of these ways:

### Example of a TXT record (regular mode)
| Host                  | TXT record                                                                       |
| --------------------- | -------------------------------------------------------------------------------- |
| `app01.infra.local`   | `OS=linux;ENV=dev;ROLE=app;SRV=tomcat_backend_auth;VARS=key1=value1,key2=value2` |

### Example of a TXT record (no-transfer mode)
| Host                                | TXT record                                                                                         |
| ----------------------------------- | -------------------------------------------------------------------------------------------------- |
| `ansible-dns-inventory.infra.local` | `app01.infra.local:OS=linux;ENV=dev;ROLE=app;SRV=tomcat_backend_auth;VARS=key1=value1,key2=value2` |

The separator between the hostname and the attribute string in the no-transfer mode is customizable (the `dns.notransfer.separator` parameter).

### Host attributes (default key names)
| Key  | Description                                                                                                                                                 |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| OS   | Operating system identifier. Required.                                                                                                                      |
| ENV  | Environment identifier. Required.                                                                                                                           |
| ROLE | Host role identifier(s). Required. Can be a comma-delimited list.                                                                                           |
| SRV  | Host service identifier(s). This will be split further using the `txt.keys.separator` to produce a hierarchy of groups. Required. Can also be a comma-delimited list. |
| VARS | Optional host variables.                                                                                                                                    |

Key names and separators are customizable via `ansible-dns-inventory`'s config file.
Key values are validated and can only contain numbers and letters of the Latin alphabet, except for the service identifier(s) which can also contain the `txt.keys.separator` symbol.

### Host variables
`ansible-dns-inventory` supports passing additional host variables to Ansible via the `VARS` attribute. This feature is disabled by default, you can enable it by setting the `txt.vars.enabled` parameter to `true`.
This is meant to be used in cases where storing some Ansible host variables directly in TXT records could be a good idea. For example, you might want to put variables like `ansible_user` there.

WARNING: This feature adds an additional DNS request for every host in your inventory so be careful when using it with large inventories.
The no-transfer mode may particularly suffer a perfomance hit if host variables are used.

## Config file

`ansible-dns-inventory` can use a YAML configuration file, a set of environment variables or both as its configuration source.

It will try to load the file specified in the `ADI_CONFIG_FILE` environment variable if it is defined.
If this variable is not defined or has an empty value, it looks for an `ansible-dns-inventory.yaml` file inside these directories (in this specific order):

* `.` (current working directory)
* `~/.ansible/`
* `/etc/ansible/`

`ansible-dns-inventory` will panic if a configuration file was found but there was a problem reading it.
If no configuration file was found, it will fall back to using default values and environment variables.

Every parameter can also be overriden by a corresponding environment variable.
There is a [template](config/ansible-dns-inventory.yaml) in this repository that lists descriptions, environment variable names and default values for all available parameters.

### Example of a config file
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

## Inventory structure

In general, if you have a single TXT record for a `HOST` and this record has all 4 required attributes set then this `HOST` will end up in this hierarchy of groups:

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

## Export mode

`ansible-dns-inventory` can also export the inventory in several formats. This makes it possible to use your inventory in some third-party software.
An example of this use case would be using this output as a dictionary in a [Logstash translate filter](https://www.elastic.co/guide/en/logstash/current/plugins-filters-translate.html#plugins-filters-translate-dictionary_path) to populate a `groups` field during log processing to be able to filter events coming from a specific group of hosts.

There are several export modes, which support different export formats.

| Flag      | Description                                                             | Formats                                 |
| --------- | ----------------------------------------------------------------------- | --------------------------------------- |
| `-hosts`  | Export hosts, mapping each one to a list of groups.                     | `json`, `yaml`, `yaml-list`, `yaml-csv` |
| `-groups` | Export groups, mapping each one to a list of hosts.                     | `json`, `yaml`, `yaml-list`, `yaml-csv` |
| `-attrs`  | Export hosts, mapping each one to a list of dictionaries of attributes. | `json`, `yaml`, `yaml-flow`             |
| `-tree`   | Export the raw inventory tree.                                          | `json`, `yaml`                          |

The default format is always `yaml`.

The `-attrs` mode exports a list of dictionaries of attributes for each host. If a host has multiple TXT records or multiple elements in a comma-separated list in the `ROLE` or `SRV` attribute, the attribute list for this host in the `-attrs` output will contain multiple dictionaries: one for each detected attribute "set".

### Examples:
```
$ dns-inventory -hosts -format yaml-list
...
"app01.infra.local": ["all", "all_app", "all_app_tomcat", "all_host", ...]
...

$ dns-inventory -hosts -format yaml-csv
...
"app01.infra.local": "all,all_app,all_app_tomcat,all_host,..."
...

$ dns-inventory -attrs -format yaml-flow
...
"app01.infra.local": [{"OS": "linux", "ENV": "dev", "ROLE": "app", "SRV": "tomcat_backend_auth", "VARS": "key1=value1,key2=value2"}]
...
```

## Roadmap

- [ ] Implement key-value stores support (etcd, Consul, etc.).
- [ ] Support using `ansible-dns-inventory` as a library.
