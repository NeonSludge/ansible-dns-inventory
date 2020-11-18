package config

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type (
	// DNS server configuration.
	DNS struct {
		// DNS server address.
		Address string
		// Network timeout for DNS requests.
		Timeout string
		// DNS zone list.
		Zones []string
		// Enable no-transfer data retrieval mode.
		NoTx bool
		// A host whose TXT records contain inventory data.
		NoTxHost string
		// Separator between a hostname and an attribute string in a TXT record.
		NoTxSeparator string
	}

	// TXT attribute parsing configuration
	Parse struct {
		// Separator between k/v pairs found in TXT records.
		KvSeparator string
		// Separator between a key and a value.
		KvEquals string
		// Separator between elements of an Ansible group name.
		KeySeparator string
		// Key name of the attribute containing the host operating system identifier.
		KeyOs string
		// Key name of the attribute containing the host environment identifier.
		KeyEnv string
		// Key name of the attribute containing the host role identifier.
		KeyRole string
		// Key name of the attribute containing the host service identifier.
		KeySrv string
	}
)

// load DNS server configuration.
func (c *DNS) load() {
	c.Address = viper.GetString("dns.server")
	c.Timeout = viper.GetString("dns.timeout")
	c.Zones = viper.GetStringSlice("dns.zones")
	c.NoTx = viper.GetBool("dns.notransfer.enabled")
	c.NoTxHost = viper.GetString("dns.notransfer.host")
	c.NoTxSeparator = viper.GetString("dns.notransfer.separator")
}

// load TXT attribute parsing configuration
func (c *Parse) load() {
	c.KvSeparator = viper.GetString("txt.kv.separator")
	c.KvEquals = viper.GetString("txt.kv.equalsign")
	c.KeySeparator = viper.GetString("txt.keys.separator")
	c.KeyOs = viper.GetString("txt.keys.os")
	c.KeyEnv = viper.GetString("txt.keys.env")
	c.KeyRole = viper.GetString("txt.keys.role")
	c.KeySrv = viper.GetString("txt.keys.srv")
}

func Init() {
	// Load YAML configuration.
	path, ok := os.LookupEnv("ADI_CONFIG_FILE")
	if ok {
		// Load a specific config file.
		viper.SetConfigFile(path)
	} else {
		// Try to find the config file in standard loctions.
		viper.SetConfigName("ansible-dns-inventory")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.ansible")
		viper.AddConfigPath("/etc/ansible")
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(errors.Wrap(err, "failed to read config file"))
		}
	}

	// Setup environment variables handling.
	viper.SetEnvPrefix("adi")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set defaults.
	viper.SetDefault("dns.server", "127.0.0.1:53")
	viper.SetDefault("dns.timeout", "30s")
	viper.SetDefault("dns.zones", []string{"server.local."})

	viper.SetDefault("dns.notransfer.enabled", false)
	viper.SetDefault("dns.notransfer.host", "ansible-dns-inventory")
	viper.SetDefault("dns.notransfer.separator", ":")

	viper.SetDefault("txt.kv.separator", ";")
	viper.SetDefault("txt.kv.equalsign", "=")

	viper.SetDefault("txt.keys.separator", "_")
	viper.SetDefault("txt.keys.os", "OS")
	viper.SetDefault("txt.keys.env", "ENV")
	viper.SetDefault("txt.keys.role", "ROLE")
	viper.SetDefault("txt.keys.srv", "SRV")
}

func NewDNS() *DNS {
	cfg := &DNS{}
	cfg.load()

	return cfg
}

func NewParse() *Parse {
	cfg := &Parse{}
	cfg.load()

	return cfg
}
