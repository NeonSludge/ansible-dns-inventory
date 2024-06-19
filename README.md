# ansible-dns-inventory

[![Go Report Card](https://goreportcard.com/badge/github.com/NeonSludge/ansible-dns-inventory)](https://goreportcard.com/report/github.com/NeonSludge/ansible-dns-inventory)
[![Go Reference](https://pkg.go.dev/badge/github.com/NeonSludge/ansible-dns-inventory.svg)](https://pkg.go.dev/github.com/NeonSludge/ansible-dns-inventory)

A CLI tool (and a library) that processes sets of host attributes stored as DNS TXT records or key/value pairs in etcd to create a tree-like inventory of your infrastructure. It can be used directly as an Ansible [dynamic inventory script](https://docs.ansible.com/ansible/latest/user_guide/intro_dynamic_inventory.html) or export the inventory in several helpful formats.

## Features

- Files and environment variables are supported as configuration sources. 
- DNS and etcd are available as data sources.
- **(DNS data source)** two modes of operation: zone transfers and regular DNS queries.
- **(DNS data source)** TSIG support for zone transfers.
- **(Etcd data source)** authentication and mTLS support.
- **(Etcd data source)** importing host records from a YAML file.
- Unlimited number and length of inventory tree branches.
- Predictable and stable inventory structure.
- Multiple records per host supported.
- Optional custom Ansible variables in host records (see caveats in the 'Host variables' section).
- Can be used as a library.

## Usage

```txt
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
  -import string
    	import host records from file
  -list
    	produce a JSON inventory for Ansible
  -tree
    	export raw inventory tree
  -version
    	display ansible-dns-inventory version and build info
```

## Prerequisites

### DNS data source

1. Allow zone transfers (AXFR) from your DNS server to the host that is going to be running the `dns-inventory` utility and setup TSIG parameters in the configuration file (if needed) or use the no-transfer mode (the `dns.notransfer.enabled` parameter).
2. Add one or more properly formatted DNS TXT records either for the managed hosts themselves or for a special host (the `dns.notransfer.host` parameter) if you're using the no-transfer mode.
3. Set other relevant parameters in the configuration file or via environment variables.

### Etcd data source

1. Add one or more properly formatted key/value pairs for all managed hosts.
2. Set other relevant parameters in the configuration file or via environment variables.

## Configuration file

`ansible-dns-inventory` can use a YAML configuration file, a set of environment variables or both as its configuration source.

It will try to load the file specified in the `ADI_CONFIG_FILE` environment variable if it is defined.
If this variable is not defined or has an empty value, it looks for an `ansible-dns-inventory.yaml` file inside these directories (in this specific order):

1. current working directory
2. `<home directory>/.ansible/`
3. `/etc/ansible/`

`ansible-dns-inventory` will panic if a configuration file was found but there was a problem reading it.
If no configuration file was found, it will fall back to using default values and environment variables.

Every parameter can also be overriden by a corresponding environment variable.
There is a [template](config/ansible-dns-inventory.yaml) in this repository that lists descriptions, environment variable names and default values for all available parameters.

### Example of a config file

```yaml
datasource: dns
dns:
  server: "10.100.100.1:53"
  timeout: "120s"
  zones:
    - server.local.
    - infra.local.
etcd:
  endpoints:
    - 10.100.100.1:2379
    - 10.100.100.2:2379
    - 10.100.100.3:2379
  tls:
    insecure: true
txt:
  kv:
    separator: "|"
  keys:
    env: "PRJ"

```

## Host records

### DNS data source

There are two ways to add a host to the inventory:

1. Create a DNS TXT record for this host and format it properly, specifying host attributes as a set of key/value pairs. One host can have an unlimited number of TXT records: all of them will be parsed by `ansible-dns-inventory`.
2. Enable the no-transfer mode, add a TXT record for the special host (`ansible-dns-inventory.your.domain` by default) and format it properly, referencing the host you want to add to your inventory and specifying its attributes as a set of key/value pairs. Again, one host can have any number of records here.

Here is an example of using both of these ways:

#### Example of a DNS TXT record (regular mode)

| Host                  | TXT record                                                                       |
| --------------------- | -------------------------------------------------------------------------------- |
| `app01.infra.local`   | `OS=linux;ENV=dev;ROLE=app;SRV=tomcat_backend_auth;VARS=key1=value1,key2=value2` |

#### Example of a DNS TXT record (no-transfer mode)

| Host                                | TXT record                                                                                         |
| ----------------------------------- | -------------------------------------------------------------------------------------------------- |
| `ansible-dns-inventory.infra.local` | `app01.infra.local:OS=linux;ENV=dev;ROLE=app;SRV=tomcat_backend_auth;VARS=key1=value1,key2=value2` |

The separator between the hostname and the attribute string in the no-transfer mode is customizable (the `dns.notransfer.separator` parameter).

### Etcd data source

There is only one way of adding host records to an etcd data source.
You create a key/value pair where the value is formatted the same way as with the DNS data source and the key name must be constructed by strictly following this scheme:

`<prefix>/<zone>/<hostname>/<index>`

...where:

- `<prefix>` is the same as the `etcd.prefix` configuration parameter value
- `<zone>` is one of the zones listed in the `etcd.zones` parameter
- `<hostname>` is the FQDN of a host
- `<index>` starts at 0 and is incremented for each additional record belonging to the same host.

#### Example of a key/value pair

| Key                                                  | Value                                                                            |
| ---------------------------------------------------- | -------------------------------------------------------------------------------- |
| `ANSIBLE_INVENTORY/infra.local./app01.infra.local/0` | `OS=linux;ENV=dev;ROLE=app;SRV=tomcat_backend_auth;VARS=key1=value1,key2=value2` |



### Host attributes (default keys)

| Key  | Description                                                                                                                                                 |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| OS   | Operating system identifier. Required.                                                                                                                      |
| ENV  | Environment identifier. Required.                                                                                                                           |
| ROLE | Host role identifier(s). Required. Can be a comma-delimited list.                                                                                           |
| SRV  | Host service identifier(s). This will be split further using the `txt.keys.separator` to produce a hierarchy of groups. Required. Can also be a comma-delimited list. |
| VARS | Optional host variables.                                                                                                                                    |

All keys and separators are customizable via `ansible-dns-inventory`'s config file.
Values are validated and can only contain numbers and letters of the Latin alphabet, except for the service identifier(s) which can also contain the `txt.keys.separator` symbol.

### Host variables

`ansible-dns-inventory` supports passing additional host variables to Ansible via the `VARS` attribute. This feature is disabled by default, you can enable it by setting the `txt.vars.enabled` parameter to `true`.
This is meant to be used in cases where storing some Ansible host variables directly in TXT records could be a good idea. For example, you might want to put variables like `ansible_user` there.

WARNING: This feature adds an additional DNS request for every host in your inventory so be careful when using it with large inventories.
The no-transfer mode may particularly suffer a perfomance hit if host variables are used.

## Inventory structure

In general, if you have a single TXT record for a `HOST` and this record has all 4 required attributes set then this `HOST` will end up in this hierarchy of groups:

```txt
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

```txt
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

### Examples

```txt
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

## Import mode

Some `ansible-dns-inventory` datasources support importing host records from a YAML file. These currently include:
- etcd datasource

To populate one of these datasources with host records, first create a YAML file with the same structure as the `-attrs` export mode output:
```
# cat import.yaml
app01.infra.local:
- ENV: dev
  OS: linux
  ROLE: app
  SRV: tomcat_backend_auth
  VARS: ansible_host=10.0.0.1
app02.infra.local:
- ENV: dev
  OS: linux
  ROLE: app
  SRV: tomcat_backend_auth
  VARS: ansible_host=10.0.0.2
```   

Then run `ansible-dns-inventory` in the import mode:
```
dns-inventory -import ./import.yaml
```

WARNING: while only default host attribute keys (`OS/ENV/ROLE/SRV/VARS`) are supported in the input file itself, the actual records will use your custom keys if set in the configuration.

## Roadmap

- [x] Implement key-value stores support (etcd, Consul, etc.).
- [x] Support using `ansible-dns-inventory` as a library.
- [!] Implement import mode for some of the datasources. (implemented for the etcd datasource)
- [ ] Support more datasource types.
