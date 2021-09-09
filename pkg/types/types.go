package types

type (
	// Inventory configuration.
	InventoryConfig struct {
		Datasource string `mapstructure:"datasource"`
		DNS        struct {
			Server     string   `mapstructure:"server"`
			Timeout    string   `mapstructure:"timeout"`
			Zones      []string `mapstructure:"zones"`
			Notransfer struct {
				Enabled   bool   `mapstructure:"enabled"`
				Host      string `mapstructure:"host"`
				Separator string `mapstructure:"separator"`
			} `mapstructure:"notransfer"`
			Tsig struct {
				Enabled bool   `mapstructure:"enabled"`
				Key     string `mapstructure:"key"`
				Secret  string `mapstructure:"secret"`
				Algo    string `mapstructure:"algo"`
			} `mapstructure:"tsig"`
		} `mapstructure:"dns"`
		Etcd struct {
			Endpoints []string `mapstructure:"endpoints"`
			Timeout   string   `mapstructure:"timeout"`
			Prefix    string   `mapstructure:"prefix"`
			Zones     []string `mapstructure:"zones"`
		} `mapstructure:"etcd"`
		Txt struct {
			Kv struct {
				Separator string `mapstructure:"separator"`
				Equalsign string `mapstructure:"equalsign"`
			} `mapstructure:"kv"`
			Vars struct {
				Enabled   bool   `mapstructure:"enabled"`
				Separator string `mapstructure:"separator"`
				Equalsign string `mapstructure:"equalsign"`
			} `mapstructure:"vars"`
			Keys struct {
				Separator string `mapstructure:"separator"`
				Os        string `mapstructure:"os"`
				Env       string `mapstructure:"env"`
				Role      string `mapstructure:"role"`
				Srv       string `mapstructure:"srv"`
				Vars      string `mapstructure:"vars"`
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
