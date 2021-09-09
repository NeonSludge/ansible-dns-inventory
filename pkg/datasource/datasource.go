package datasource

import (
	"context"
	"time"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	etcdv3 "go.etcd.io/etcd/client/v3"
	etcdns "go.etcd.io/etcd/client/v3/namespace"

	"github.com/NeonSludge/ansible-dns-inventory/pkg/types"
)

// New creates a datasource base on the configuration.
func New(cfg *types.InventoryConfig) (types.InventoryDatasource, error) {
	var ds types.InventoryDatasource

	// Select datasource implementation.
	switch cfg.Datasource {
	case "dns":
		t, err := time.ParseDuration(cfg.DNS.Timeout)
		if err != nil {
			return nil, errors.Wrap(err, "dns datasource initialization failure")
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
	case "etcd":
		t, err := time.ParseDuration(cfg.Etcd.Timeout)
		if err != nil {
			return nil, errors.Wrap(err, "etcd datasource initialization failure")
		}

		c, err := etcdv3.New(etcdv3.Config{
			Endpoints:   cfg.Etcd.Endpoints,
			DialTimeout: t,
		})
		if err != nil {
			return nil, errors.Wrap(err, "etcd datasource initialization failure")
		}

		ctx, cnc := context.WithTimeout(context.Background(), t)

		// Set etcd namespace.
		ns := cfg.Etcd.Prefix
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
		return nil, errors.Errorf("unknown datasource type: %s", cfg.Datasource)
	}

	return ds, nil
}
