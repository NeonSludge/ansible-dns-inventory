package inventory

import (
	"context"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/mvccpb"
	etcdv3 "go.etcd.io/etcd/client/v3"
)

type (
	// An etcd datasource implementation.
	EtcdDatasource struct {
		// Etcd client.
		Client *etcdv3.Client
		// Etcd request context.
		Context context.Context
		// Etcd request context cancel function.
		Cancel context.CancelFunc
		// Inventory configuration.
		Config *Config
		// Inventory logger.
		Logger Logger
	}
)

// Process several k/v pairs.
func (e *EtcdDatasource) processKVs(kvs []*mvccpb.KeyValue) []*DatasourceRecord {
	log := e.Logger
	var name string
	records := make([]*DatasourceRecord, 0)

	// Host attribute sets
	sets := make(map[int]string)

	for _, kv := range kvs {
		key := strings.Split(string(kv.Key), "/")
		value := string(kv.Value)

		// Determine which set of host attributes we are working with.
		num, err := strconv.Atoi(key[2])
		if err != nil {
			log.Warnf("[%s] skipping host attributes set: %v", key[1], err)
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
		records = append(records, &DatasourceRecord{
			Hostname:   name,
			Attributes: set,
		})
	}

	return records
}

// getPrefix acquires all key/value records for a specific prefix.
func (e *EtcdDatasource) getPrefix(prefix string) ([]*mvccpb.KeyValue, error) {
	resp, err := e.Client.Get(e.Context, prefix, etcdv3.WithPrefix())
	if err != nil {
		return nil, errors.Wrap(err, "etcd request failure")
	}

	return resp.Kvs, nil
}

// GetAllRecords acquires all available host records.
func (e *EtcdDatasource) GetAllRecords() ([]*DatasourceRecord, error) {
	cfg := e.Config
	log := e.Logger
	records := make([]*DatasourceRecord, 0)

	for _, zone := range cfg.Etcd.Zones {
		kvs, err := e.getPrefix(zone)
		if err != nil {
			log.Warnf("[%s] skipping zone: %v", zone, err)
			continue
		}

		records = append(records, e.processKVs(kvs)...)
	}

	return records, nil
}

// GetHostRecords acquires all available records for a specific host.
func (e *EtcdDatasource) GetHostRecords(host string) ([]*DatasourceRecord, error) {
	cfg := e.Config
	var zone string

	// Determine which zone we are working with.
	for _, z := range cfg.Etcd.Zones {
		if strings.HasSuffix(host, z) {
			zone = z
			break
		}
	}

	if len(zone) == 0 {
		return nil, errors.New("failed to determine zone from hostname")
	}

	prefix := zone + "/" + host
	kvs, err := e.getPrefix(prefix)
	if err != nil {
		return nil, err
	}

	return e.processKVs(kvs), nil
}

// Close datasource and perform housekeeping.
func (e *EtcdDatasource) Close() {
	e.Cancel()
	e.Client.Close()
}
