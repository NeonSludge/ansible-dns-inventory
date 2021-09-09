package types

type (
	// Inventory configuration.
	InventoryConfig struct {
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
	InventoryDatasource interface {
		// GetAllRecords returns all host records.
		GetAllRecords() ([]*InventoryDatasourceRecord, error)
		// GetHostRecords returns all records for a specific host.
		GetHostRecords(host string) ([]*InventoryDatasourceRecord, error)
		// Close closes datasource clients and performs other housekeeping.
		Close()
	}

	// Inventory datasource record.
	InventoryDatasourceRecord struct {
		// Host name.
		Hostname string
		// Host attributes.
		Attributes string
	}
)
