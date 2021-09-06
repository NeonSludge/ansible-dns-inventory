package datasource

import (
	"time"

	"github.com/miekg/dns"
	"github.com/pkg/errors"

	"github.com/NeonSludge/ansible-dns-inventory/pkg/types"
)

// New creates a datasource base on the configuration.
func New(cfg types.Config) (types.Datasource, error) {
	var ds types.Datasource

	switch cfg.GetString("datasource") {
	case "dns":
		t, err := time.ParseDuration(cfg.GetString("dns.timeout"))
		if err != nil {
			return nil, errors.Wrap(err, "dns datasource failure")
		}

		ds = &DNS{
			Client: &dns.Client{
				Timeout: t,
			},
			Transfer: &dns.Transfer{
				DialTimeout:  t,
				ReadTimeout:  t,
				WriteTimeout: t,
			},
			Config: cfg,
		}
	default:
		return nil, errors.New("unknown datasource type")
	}

	return ds, nil
}
