package config

import (
	"os"
	"strings"

	"github.com/creasty/defaults"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/NeonSludge/ansible-dns-inventory/pkg/inventory"
)

const (
	adiEnvPrefix = "ADI"
)

func configKeys() []string {
	return []string{
		"datasource",
		"dns.server",
		"dns.timeout",
		"dns.zones",
		"dns.notransfer.enabled",
		"dns.notransfer.host",
		"dns.notransfer.separator",
		"dns.tsig.enabled",
		"dns.tsig.key",
		"dns.tsig.secret",
		"dns.tsig.algo",
		"etcd.endpoints",
		"etcd.timeout",
		"etcd.prefix",
		"etcd.zones",
		"etcd.auth.username",
		"etcd.auth.password",
		"etcd.tls.enabled",
		"etcd.tls.insecure",
		"etcd.tls.ca.path",
		"etcd.tls.ca.pem",
		"etcd.tls.certificate.path",
		"etcd.tls.certificate.pem",
		"etcd.tls.key.path",
		"etcd.tls.key.pem",
		"etcd.import.clear",
		"etcd.import.batch",
		"txt.kv.separator",
		"txt.kv.equalsign",
		"txt.vars.enabled",
		"txt.vars.separator",
		"txt.vars.equalsign",
		"txt.keys.separator",
		"txt.keys.os",
		"txt.keys.env",
		"txt.keys.role",
		"txt.keys.srv",
		"txt.keys.vars",
		"filter.enabled",
	}
}

// tsigAlgo processes user-supplied TSIG algorithm names.
func tsigAlgo(algo string) string {
	switch algo {
	case "hmac-sha1", "hmac-sha224", "hmac-sha256", "hmac-sha384", "hmac-sha512":
		return algo + "."
	default:
		return "hmac-sha256."
	}
}

// Load reads the configuration with Viper.
func Load() (*inventory.Config, error) {
	v := viper.New()

	// Load YAML configuration.
	path, ok := os.LookupEnv("ADI_CONFIG_FILE")
	if ok {
		// Load a specific config file.
		v.SetConfigFile(path)
	} else {
		// Try to find the config file in standard locations.
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.Wrap(err, "failed to determine user's home directory")
		}

		v.SetConfigName("ansible-dns-inventory")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath(home + "/.ansible")
		v.AddConfigPath("/etc/ansible")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, errors.Wrap(err, "failed to read config file")
		}
	}

	// Setup environment variables handling.
	v.SetEnvPrefix(adiEnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind environment variables to configuration keys.
	for _, key := range configKeys() {
		if err := v.BindEnv(key); err != nil {
			return nil, errors.Wrap(err, "failed to bind environment variables")
		}
	}

	// Process user-supplied TSIG algorithm name.
	v.Set("dns.tsig.algo", tsigAlgo(v.GetString("dns.tsig.algo")))

	cfg := &inventory.Config{}

	if err := defaults.Set(cfg); err != nil {
		return nil, errors.Wrap(err, "defaults initialization failure")
	}

	// Unmarshal Viper configuration to an instance of inventory.Config.
	if err := v.Unmarshal(cfg); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal configuration")
	}

	return cfg, nil
}
