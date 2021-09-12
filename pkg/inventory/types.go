package inventory

import (
	"encoding/json"
)

type (
	Inventory struct {
		// Inventory configuration.
		Config *Config
		// Inventory datasource.
		Datasource Datasource
		// Inventory logger.
		Logger Logger
		// Inventory tree.
		Tree *Node
	}

	Config struct {
		// A logger for the inventory.
		// By default, the global zap.SugaredLogger is used.
		Logger Logger
		// Datasource type.
		// Currently supported: dns, etcd.
		Datasource string `mapstructure:"datasource"`
		// DNS datasource configuration.
		DNS struct {
			// DNS server address.
			Server string `mapstructure:"server"`
			// Network timeout for DNS requests.
			Timeout string `mapstructure:"timeout"`
			// DNS zone list.
			Zones []string `mapstructure:"zones"`
			// No-transfer mode configuration.
			Notransfer struct {
				// Enable no-transfer data retrieval mode.
				Enabled bool `mapstructure:"enabled"`
				// A host whose TXT records contain inventory data.
				Host string `mapstructure:"host"`
				// Separator between a hostname and an attribute string in a TXT record.
				Separator string `mapstructure:"separator"`
			} `mapstructure:"notransfer"`
			// TSIG parameters (used only with zone transfer requests).
			Tsig struct {
				// Enable TSIG.
				Enabled bool `mapstructure:"enabled"`
				// TSIG key name.
				Key string `mapstructure:"key"`
				// TSIG secret (base64-encoded).
				Secret string `mapstructure:"secret"`
				// TSIG algorithm.
				// Allowed values: 'hmac-sha1', hmac-sha224, 'hmac-sha256', 'hmac-sha384', 'hmac-sha512'. 'hmac-sha256' is used if an invalid value is specified.
				Algo string `mapstructure:"algo"`
			} `mapstructure:"tsig"`
		} `mapstructure:"dns"`
		// Etcd datasource configuration.
		Etcd struct {
			// Etcd cluster endpoints.
			Endpoints []string `mapstructure:"endpoints"`
			// Network timeout for etcd requests.
			Timeout string `mapstructure:"timeout"`
			// Etcd k/v path prefix.
			Prefix string `mapstructure:"prefix"`
			// Etcd host zone list.
			Zones []string `mapstructure:"zones"`
		} `mapstructure:"etcd"`
		// Host records parsing configuration.
		Txt struct {
			// Key/value pair parsing configuration.
			Kv struct {
				// Separator between k/v pairs found in TXT records.
				Separator string `mapstructure:"separator"`
				// Separator between a key and a value.
				Equalsign string `mapstructure:"equalsign"`
			} `mapstructure:"kv"`
			// Host variables parsing configuration.
			Vars struct {
				// Enable host variables support.
				Enabled bool `mapstructure:"enabled"`
				// Separator between k/v pairs found in the host variables attribute.
				Separator string `mapstructure:"separator"`
				// Separator between a key and a value.
				Equalsign string `mapstructure:"equalsign"`
			} `mapstructure:"vars"`
			// Host attributes parsing configuration.
			Keys struct {
				// Separator between elements of an Ansible group name.
				Separator string `mapstructure:"separator"`
				// Key name of the attribute containing the host operating system identifier.
				Os string `mapstructure:"os"`
				// Key name of the attribute containing the host environment identifier.
				Env string `mapstructure:"env"`
				// Key name of the attribute containing the host role identifier.
				Role string `mapstructure:"role"`
				// Key name of the attribute containing the host service identifier.
				Srv string `mapstructure:"srv"`
				// Key name of the attribute containing the host variables.
				Vars string `mapstructure:"vars"`
			} `mapstructure:"keys"`
		} `mapstructure:"txt"`
	}

	// Inventory datasource
	Datasource interface {
		// GetAllRecords returns all host records.
		GetAllRecords() ([]*DatasourceRecord, error)
		// GetHostRecords returns all records for a specific host.
		GetHostRecords(host string) ([]*DatasourceRecord, error)
		// Close closes datasource clients and performs other housekeeping.
		Close()
	}

	// Inventory datasource record.
	DatasourceRecord struct {
		// Host name.
		Hostname string
		// Host attributes.
		Attributes string
	}

	// Inventory logger.
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
		OS string `validate:"nonzero,safe"`
		// Host environment identifier.
		Env string `validate:"nonzero,safe"`
		// Host role identifier.
		Role string `validate:"nonzero,safe=list"`
		// Host service identifier.
		Srv string `validate:"safe=srv"`
		// Host variables
		Vars string `validate:"safe=vars"`
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

// MarshalJSON implements a custom JSON Marshaller for host attributes.
func (a *HostAttributes) MarshalJSON() ([]byte, error) {
	attrs := make(map[string]string)

	attrs[ADIHostAttributeNames["OS"]] = a.OS
	attrs[ADIHostAttributeNames["ENV"]] = a.Env
	attrs[ADIHostAttributeNames["ROLE"]] = a.Role
	attrs[ADIHostAttributeNames["SRV"]] = a.Srv
	attrs[ADIHostAttributeNames["VARS"]] = a.Vars

	return json.Marshal(attrs)
}

// MarshalYAML implements a custom YAML Marshaller for host attributes.
func (a *HostAttributes) MarshalYAML() (interface{}, error) {
	attrs := make(map[string]string)

	attrs[ADIHostAttributeNames["OS"]] = a.OS
	attrs[ADIHostAttributeNames["ENV"]] = a.Env
	attrs[ADIHostAttributeNames["ROLE"]] = a.Role
	attrs[ADIHostAttributeNames["SRV"]] = a.Srv
	attrs[ADIHostAttributeNames["VARS"]] = a.Vars

	return attrs, nil
}
