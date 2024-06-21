package inventory

import (
	"time"

	"github.com/go-playground/validator/v10"
)

type (
	// Inventory implements a dynamic inventory for Ansible.
	Inventory struct {
		// Inventory configuration.
		Config *Config
		// Inventory logger.
		Logger Logger
		// Inventory validator.
		Validator *validator.Validate
		// Inventory datasource.
		Datasource Datasource
		// Inventory tree.
		Tree *Node
	}

	// Config represents the main inventory configuration.
	Config struct {
		// Datasource type.
		// Currently supported: dns, etcd.
		Datasource string `mapstructure:"datasource" default:"dns"`
		// DNS datasource configuration.
		DNS struct {
			// DNS server address.
			Server string `mapstructure:"server" default:"127.0.0.1:53"`
			// Network timeout for DNS requests.
			Timeout time.Duration `mapstructure:"timeout" default:"30s"`
			// DNS zone list.
			Zones []string `mapstructure:"zones" default:"[\"server.local.\"]"`
			// No-transfer mode configuration.
			Notransfer struct {
				// Enable no-transfer data retrieval mode.
				Enabled bool `mapstructure:"enabled" default:"false"`
				// A host whose TXT records contain inventory data.
				Host string `mapstructure:"host" default:"ansible-dns-inventory"`
				// Separator between a hostname and an attribute string in a TXT record.
				Separator string `mapstructure:"separator" default:":"`
			} `mapstructure:"notransfer"`
			// TSIG parameters (used only with zone transfer requests).
			Tsig struct {
				// Enable TSIG.
				Enabled bool `mapstructure:"enabled" default:"false"`
				// TSIG key name.
				Key string `mapstructure:"key" default:"axfr."`
				// TSIG secret (base64-encoded).
				Secret string `mapstructure:"secret" default:"c2VjcmV0Cg=="`
				// TSIG algorithm.
				// Allowed values: 'hmac-sha1', hmac-sha224, 'hmac-sha256', 'hmac-sha384', 'hmac-sha512'. 'hmac-sha256' is used if an invalid value is specified.
				Algo string `mapstructure:"algo" default:"hmac-sha256."`
			} `mapstructure:"tsig"`
		} `mapstructure:"dns"`
		// Etcd datasource configuration.
		Etcd struct {
			// Etcd cluster endpoints.
			Endpoints []string `mapstructure:"endpoints" default:"[\"127.0.0.1:2379\"]"`
			// Network timeout for etcd requests.
			Timeout time.Duration `mapstructure:"timeout" default:"30s"`
			// Etcd k/v path prefix.
			Prefix string `mapstructure:"prefix" default:"ANSIBLE_INVENTORY"`
			// Etcd host zone list.
			Zones []string `mapstructure:"zones" default:"[\"server.local.\"]"`
			// Etcd authentication configuration.
			Auth struct {
				// Username for authentication.
				Username string `mapstructure:"username" default:""`
				// Password for authentication.
				Password string `mapstructure:"password" default:""`
			} `mapstructure:"auth"`
			// Etcd TLS configuration.
			TLS struct {
				// Enable TLS.
				Enabled bool `mapstructure:"enabled" default:"true"`
				// Skip verification of the etcd server's certificate chain and host name.
				Insecure bool `mapstructure:"insecure" default:"false"`
				// Trusted CA bundle.
				CA struct {
					Path string `mapstructure:"path" default:""`
					PEM  string `mapstructure:"pem" default:""`
				} `mapstructure:"ca"`
				// User certificate.
				Certificate struct {
					Path string `mapstructure:"path" default:""`
					PEM  string `mapstructure:"pem" default:""`
				} `mapstructure:"certificate"`
				// User private key.
				Key struct {
					Path string `mapstructure:"path" default:""`
					PEM  string `mapstructure:"pem" default:""`
				} `mapstructure:"key"`
			} `mapstructure:"tls"`
			// Etcd datasource import mode configuration.
			Import struct {
				// Clear all existing host records before importing records from file.
				Clear bool `mapstructure:"clear" default:"true"`
				// Batch size used when pushing host records to etcd.
				// Should not exceed the maximum number of operations permitted in a etcd transaction (max-txn-ops).
				Batch int `mapstructure:"batch" default:"128"`
			} `mapstructure:"import"`
		} `mapstructure:"etcd"`
		// Host records parsing configuration.
		Txt struct {
			// Key/value pair parsing configuration.
			Kv struct {
				// Separator between k/v pairs found in TXT records.
				Separator string `mapstructure:"separator" default:";"`
				// Separator between a key and a value.
				Equalsign string `mapstructure:"equalsign" default:"="`
			} `mapstructure:"kv"`
			// Host variables parsing configuration.
			Vars struct {
				// Enable host variables support.
				Enabled bool `mapstructure:"enabled" default:"false"`
				// Separator between k/v pairs found in the host variables attribute.
				Separator string `mapstructure:"separator" default:","`
				// Separator between a key and a value.
				Equalsign string `mapstructure:"equalsign" default:"="`
			} `mapstructure:"vars"`
			// Host attributes parsing configuration.
			Keys struct {
				// Separator between elements of an Ansible group name.
				Separator string `mapstructure:"separator" default:"_"`
				// Key name of the attribute containing the host operating system identifier.
				Os string `mapstructure:"os" default:"OS"`
				// Key name of the attribute containing the host environment identifier.
				Env string `mapstructure:"env" default:"ENV"`
				// Key name of the attribute containing the host role identifier.
				Role string `mapstructure:"role" default:"ROLE"`
				// Key name of the attribute containing the host service identifier.
				Srv string `mapstructure:"srv" default:"SRV"`
				// Key name of the attribute containing the host variables.
				Vars string `mapstructure:"vars" default:"VARS"`
			} `mapstructure:"keys"`
		} `mapstructure:"txt"`
	}

	// Datasource provides an interface for all supported datasources.
	Datasource interface {
		// GetAllRecords returns all host records.
		GetAllRecords() ([]*DatasourceRecord, error)
		// GetHostRecords returns all records for a specific host.
		GetHostRecords(host string) ([]*DatasourceRecord, error)
		// PublishRecords writes host records to the datasource.
		PublishRecords(records []*DatasourceRecord) error
		// Close closes datasource clients and performs other housekeeping.
		Close()
	}

	// DatasourceRecord represents a single host record returned by a datasource.
	DatasourceRecord struct {
		// Host name.
		Hostname string
		// Host attributes.
		Attributes string
	}

	// Logger provides a logging interface for the inventory and its datasources.
	Logger interface {
		Info(args ...interface{})
		Infof(template string, args ...interface{})
		Warn(args ...interface{})
		Warnf(template string, args ...interface{})
		Error(args ...interface{})
		Errorf(template string, args ...interface{})
		Fatal(args ...interface{})
		Fatalf(template string, args ...interface{})
		Debug(args ...interface{})
		Debugf(template string, args ...interface{})
	}

	// HostAttributes represents host attributes found in TXT records.
	HostAttributes struct {
		// Host operating system identifier.
		OS string `validate:"required,notblank,alphanum" yaml:"OS"`
		// Host environment identifier.
		Env string `validate:"required,notblank,alphanum" yaml:"ENV"`
		// Host role identifier.
		Role string `validate:"required,notblank,safelist" yaml:"ROLE"`
		// Host service identifier.
		Srv string `validate:"safelistsep" yaml:"SRV"`
		// Host variables
		Vars string `validate:"printascii" yaml:"VARS"`
	}

	// AnsibleGroup is an Ansible group ready to be marshalled into a JSON representation.
	AnsibleGroup struct {
		// Group chilren.
		Children []string `json:"children,omitempty"`
		// Hosts belonging to this group.
		Hosts []string `json:"hosts,omitempty"`
	}

	// Node represents and inventory tree node.
	Node struct {
		// Group name.
		Name string
		// Group Parent
		Parent *Node `json:"-" yaml:"-"`
		// Group children.
		Children []*Node
		// Hosts belonging to this group.
		Hosts map[string]bool
	}

	// ExportNode represents an inventory tree node for the tree export mode.
	ExportNode struct {
		// Group name.
		Name string `json:"name" yaml:"name"`
		// Group children.
		Children []*Node `json:"children" yaml:"children"`
		// Hosts belonging to this group.
		Hosts []string `json:"hosts" yaml:"hosts"`
	}
)
