package datasource

import (
	"context"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	etcdv3 "go.etcd.io/etcd/client/v3"
	etcdns "go.etcd.io/etcd/client/v3/namespace"

	"github.com/NeonSludge/ansible-dns-inventory/pkg/types"
)

// New creates a datasource base on the configuration.
func New(cfg types.Config) (types.Datasource, error) {
	var ds types.Datasource

	// Select datasource implementation.
	switch cfg.GetString("datasource") {
	case "dns":
		t := cfg.GetDuration("dns.timeout")
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
	case "etcd":
		t := cfg.GetDuration("etcd.timeout")
		c, err := etcdv3.New(etcdv3.Config{
			Endpoints:   cfg.GetStringSlice("etcd.endpoints"),
			DialTimeout: t,
		})
		if err != nil {
			return nil, errors.Wrap(err, "etcd datasource initialization failure")
		}

		ctx, cnc := context.WithTimeout(context.Background(), t)

		// Set etcd namespace.
		ns := cfg.GetString("etcd.prefix")
		c.KV = etcdns.NewKV(c.KV, ns+"/")
		c.Watcher = etcdns.NewWatcher(c.Watcher, ns+"/")
		c.Lease = etcdns.NewLease(c.Lease, ns+"/")

		ds = &Etcd{
			Client:  c,
			Context: ctx,
			Cancel:  cnc,
			Config:  cfg,
		}
	default:
		return nil, errors.Errorf("unknown datasource type: %s", cfg.GetString("datasource"))
	}

	return ds, nil
}
