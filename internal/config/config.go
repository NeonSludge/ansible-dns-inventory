package config

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type (
	// Main contains configuration options of ansible-dns-inventory.
	Main struct {
		// DNS server address.
		Address string
		// Network timeout for DNS requests.
		Timeout string
		// DNS zone list.
		Zones []string
		// TSIG parameters.
		TSIG TSIGParameters
		// (no-transfer mode) Enable no-transfer data retrieval mode.
		NoTx bool
		// (no-transfer mode) A host whose TXT records contain inventory data.
		NoTxHost string
		// (no-transfer mode) Separator between a hostname and an attribute string in a TXT record.
		NoTxSeparator string
		// (TXT records parsing) Separator between k/v pairs.
		KvSeparator string
		// (TXT records parsing) Separator between a key and a value.
		KvEquals string
		// (Host variables parsing) Enable host variables support.
		VarsEnabled bool
		// (Host variables parsing) Separator between k/v pairs.
		VarsSeparator string
		// (Host variables parsing) Separator between a key and a value.
		VarsEquals string
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
		// Key name of the attribute containing the host variables.
		KeyVars string
	}

	// TSIGParameters contains TSIG parameters to use during zone transfers.
	TSIGParameters struct {
		// Enable TSIG.
		Enabled bool
		// TSIG key name.
		Key string
		// TSIG secret (base64-encoded).
		Secret string
		// TSIG algorithm. Allowed values: 'hmac-sha1', 'hmac-sha256', 'hmac-sha512'.
		Algo string
	}
)

func (c *Main) load() {
	c.Address = viper.GetString("dns.server")
	c.Timeout = viper.GetString("dns.timeout")
	c.Zones = viper.GetStringSlice("dns.zones")
	c.NoTx = viper.GetBool("dns.notransfer.enabled")
	c.NoTxHost = viper.GetString("dns.notransfer.host")
	c.NoTxSeparator = viper.GetString("dns.notransfer.separator")
	c.KvSeparator = viper.GetString("txt.kv.separator")
	c.KvEquals = viper.GetString("txt.kv.equalsign")
	c.VarsEnabled = viper.GetBool("txt.vars.enabled")
	c.VarsSeparator = viper.GetString("txt.vars.separator")
	c.VarsEquals = viper.GetString("txt.vars.equalsign")
	c.KeySeparator = viper.GetString("txt.keys.separator")
	c.KeyOs = viper.GetString("txt.keys.os")
	c.KeyEnv = viper.GetString("txt.keys.env")
	c.KeyRole = viper.GetString("txt.keys.role")
	c.KeySrv = viper.GetString("txt.keys.srv")
	c.KeyVars = viper.GetString("txt.keys.vars")

	c.TSIG.Enabled = viper.GetBool("dns.tsig.enabled")
	c.TSIG.Key = viper.GetString("dns.tsig.key")
	c.TSIG.Secret = viper.GetString("dns.tsig.secret")
	c.TSIG.Algo = tsigAlgo(viper.GetString("dns.tsig.algo"))

}

// tsigAlgo processes user-supplied TSIG algorithm names.
func tsigAlgo(algo string) string {
	switch algo {
	case "hmac-sha1", "hmac-sha224", "hmac-sha256", "hmac-sha384", "hmac-sha512":
		return algo + "."
	case "hmac-md5":
		return "hmac-md5.sig-alg.reg.int."
	default:
		return "hmac-sha256."
	}
}

// New initializes and loads the configuration.
func New() *Main {
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

	viper.SetDefault("dns.tsig.enabled", false)
	viper.SetDefault("dns.tsig.key", "axfr.")
	viper.SetDefault("dns.tsig.secret", "c2VjcmV0Cg==")
	viper.SetDefault("dns.tsig.algo", "hmac-sha256")

	viper.SetDefault("txt.kv.separator", ";")
	viper.SetDefault("txt.kv.equalsign", "=")

	viper.SetDefault("txt.vars.enabled", false)
	viper.SetDefault("txt.vars.separator", ",")
	viper.SetDefault("txt.vars.equalsign", "=")

	viper.SetDefault("txt.keys.separator", "_")
	viper.SetDefault("txt.keys.os", "OS")
	viper.SetDefault("txt.keys.env", "ENV")
	viper.SetDefault("txt.keys.role", "ROLE")
	viper.SetDefault("txt.keys.srv", "SRV")
	viper.SetDefault("txt.keys.vars", "VARS")

	cfg := &Main{}
	cfg.load()

	return cfg
}
