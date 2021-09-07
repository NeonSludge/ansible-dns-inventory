package datasource

import (
	"context"
	"strconv"
	"strings"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/mvccpb"
	etcdv3 "go.etcd.io/etcd/client/v3"

	"github.com/NeonSludge/ansible-dns-inventory/pkg/types"
)

type (
	// An etcd datasource implementation.
	Etcd struct {
		// Etcd client.
		Client *etcdv3.Client
		// Etcd request context.
		Context context.Context
		// Etcd request context cancel function.
		Cancel context.CancelFunc
		// Inventory configuration.
		Config types.Config
	}
)

// Process several k/v pairs.
func (e *Etcd) processKVs(kvs []*mvccpb.KeyValue) []*types.Record {
	var name string
	records := make([]*types.Record, 0)

	// Host attribute sets
	sets := make(map[int]string)

	for _, kv := range kvs {
		key := strings.Split(string(kv.Key), "/")
		value := string(kv.Value)

		// Determine which set of host attributes we are working with.
		num, err := strconv.Atoi(key[2])
		if err != nil {
			// log...
			continue
		}

		// Set hostname.
		if len(name) == 0 {
			name = key[1]
		}

		// Populate this set of host attributes.
		sets[num] = value
	}

	for _, set := range sets {
		records = append(records, &types.Record{
			Hostname:   name,
			Attributes: set,
		})
	}

	return records
}

// GetAllRecords acquires all available host records.
func (e *Etcd) GetAllRecords() ([]*types.Record, error) {
	records := make([]*types.Record, 0)

	for _, zone := range e.Config.GetStringSlice("etcd.zones") {
		kvs, err := e.GetZone(zone)
		if err != nil {
			//  log...
			continue
		}

		records = append(records, e.processKVs(kvs)...)
	}

	return records, nil
}

// GetHostRecords acquires all available records for a specific host.
func (e *Etcd) GetHostRecords(host string) ([]*types.Record, error) {
	var zone string

	// Determine which zone we are working with.
	for _, z := range e.Config.GetStringSlice("etcd.zones") {
		if strings.HasSuffix(dns.Fqdn(host), dns.Fqdn(z)) {
			zone = z
			break
		}
	}

	if len(zone) == 0 {
		return nil, errors.New("failed to determine zone from hostname")
	}

	kvs, err := e.GetRecords(host, zone)
	if err != nil {
		return nil, err
	}

	return e.processKVs(kvs), nil
}

// GetZone acquires all records in a specific zone.
func (e *Etcd) GetZone(zone string) ([]*mvccpb.KeyValue, error) {
	return e.GetRecords("", zone)
}

// GetRecords acquires all records for a specific host.
func (e *Etcd) GetRecords(host string, zone string) ([]*mvccpb.KeyValue, error) {
	resp, err := e.Client.Get(e.Context, zone+"/"+host, etcdv3.WithPrefix())
	if err != nil {
		return nil, errors.Wrap(err, "etcd request failure")
	}

	if len(resp.Kvs) == 0 {
		return nil, errors.New("no etcd records found")
	}

	return resp.Kvs, nil
}

// Close datasource and perform housekeeping.
func (e *Etcd) Close() {
	e.Cancel()
	e.Client.Close()
}
