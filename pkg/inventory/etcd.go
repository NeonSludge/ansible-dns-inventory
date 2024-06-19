package inventory

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/mvccpb"
	etcdv3 "go.etcd.io/etcd/client/v3"
	etcdns "go.etcd.io/etcd/client/v3/namespace"
)

const (
	// Etcd datasource type.
	EtcdDatasourceType string = "etcd"
)

type (
	// EtcdDatasource implements an etcd datasource.
	EtcdDatasource struct {
		// Inventory configuration.
		Config *Config
		// Inventory logger.
		Logger Logger
		// Etcd client.
		Client *etcdv3.Client
	}
)

// processKVs processes several k/v pairs.
func (e *EtcdDatasource) processKVs(kvs []*mvccpb.KeyValue) []*DatasourceRecord {
	log := e.Logger
	records := make([]*DatasourceRecord, 0)

	// Sets of attributes for every host.
	hosts := make(map[string]map[int]string)

	for _, kv := range kvs {
		key := strings.Split(string(kv.Key), "/")
		value := string(kv.Value)

		// Determine which set of host attributes we are working with.
		setN, err := strconv.Atoi(key[2])
		if err != nil {
			log.Warnf("[%s] skipping host attributes set: %v", key[1], err)
			continue
		}

		// Populate this set of attributes for this host, overwriting if it already exists.
		if hosts[key[1]] == nil {
			hosts[key[1]] = make(map[int]string)
		}
		hosts[key[1]][setN] = value
	}

	for name, sets := range hosts {
		for _, set := range sets {
			records = append(records, &DatasourceRecord{
				Hostname:   name,
				Attributes: set,
			})
		}
	}

	return records
}

// findZone selects a matching zone from the datasource configuration based on the hostname.
func (e *EtcdDatasource) findZone(host string) (string, error) {
	cfg := e.Config
	var zone string

	// Try finding a matching zone in the configuration.
	for _, z := range cfg.Etcd.Zones {
		if strings.HasSuffix(strings.Trim(host, "."), strings.Trim(z, ".")) {
			zone = z
			break
		}
	}

	if len(zone) == 0 {
		return zone, errors.New("no matching zones found in config file")
	}

	return zone, nil
}

// getPrefix acquires all key/value records for a specific prefix.
func (e *EtcdDatasource) getPrefix(prefix string) ([]*mvccpb.KeyValue, error) {
	cfg := e.Config
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Etcd.Timeout)
	resp, err := e.Client.Get(ctx, prefix, etcdv3.WithPrefix())
	cancel()
	if err != nil {
		return nil, errors.Wrap(err, "etcd request failure")
	}

	return resp.Kvs, nil
}

// putRecord publishes a host record via the datasource.
func (e *EtcdDatasource) putRecord(record *DatasourceRecord, count int) error {
	cfg := e.Config

	zone, err := e.findZone(record.Hostname)
	if err != nil {
		return errors.Wrap(err, "failed to determine zone from hostname")
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Etcd.Timeout)
	_, err = e.Client.Put(ctx, fmt.Sprintf("%s/%s/%d", zone, record.Hostname, count), record.Attributes)
	cancel()
	if err != nil {
		return errors.Wrap(err, "etcd request failure")
	}

	return nil
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
	zone, err := e.findZone(host)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine zone from hostname")
	}

	prefix := zone + "/" + host
	kvs, err := e.getPrefix(prefix)
	if err != nil {
		return nil, err
	}

	return e.processKVs(kvs), nil
}

// PublishRecords writes host records to the datasource.
func (e *EtcdDatasource) PublishRecords(records []*DatasourceRecord) error {
	counts := map[string]int{}

	for _, record := range records {
		if _, ok := counts[record.Hostname]; ok {
			counts[record.Hostname]++
		} else {
			counts[record.Hostname] = 0
		}

		if err := e.putRecord(record, counts[record.Hostname]); err != nil {
			return errors.Wrap(err, "failed to publish a host record")
		}
	}

	return nil
}

// Close shuts down the datasource and performs other housekeeping.
func (e *EtcdDatasource) Close() {
	e.Client.Close()
}

func makeEtcdTLSConfig(cfg *Config) (*tls.Config, error) {
	var tlsCAPool *x509.CertPool
	var tlsKeyPair tls.Certificate
	var err error

	if len(cfg.Etcd.TLS.CA.PEM) > 0 {
		tlsCAPool, err = tlsCAPoolFromPEM(cfg.Etcd.TLS.CA.PEM)
	} else if len(cfg.Etcd.TLS.CA.Path) > 0 {
		tlsCAPool, err = tlsCAPoolFromFile(cfg.Etcd.TLS.CA.Path)
	}

	if err != nil {
		return nil, errors.Wrap(err, "TLS configuration error")
	}

	if len(cfg.Etcd.TLS.Certificate.PEM) > 0 && len(cfg.Etcd.TLS.Key.PEM) > 0 {
		tlsKeyPair, err = tlsKeyPairFromPEM(cfg.Etcd.TLS.Certificate.PEM, cfg.Etcd.TLS.Key.PEM)
	} else if len(cfg.Etcd.TLS.Certificate.Path) > 0 && len(cfg.Etcd.TLS.Key.Path) > 0 {
		tlsKeyPair, err = tlsKeyPairFromFile(cfg.Etcd.TLS.Certificate.Path, cfg.Etcd.TLS.Key.Path)
	}

	if err != nil {
		return nil, errors.Wrap(err, "TLS configuration error")
	}

	return &tls.Config{
		InsecureSkipVerify: cfg.Etcd.TLS.Insecure,
		RootCAs:            tlsCAPool,
		Certificates:       []tls.Certificate{tlsKeyPair},
	}, nil
}

// NewEtcdDatasource creates an etcd datasource.
func NewEtcdDatasource(cfg *Config) (*EtcdDatasource, error) {
	// Etcd client configuration
	clientCfg := etcdv3.Config{
		Endpoints:   cfg.Etcd.Endpoints,
		DialTimeout: cfg.Etcd.Timeout,
		Username:    cfg.Etcd.Auth.Username,
		Password:    cfg.Etcd.Auth.Password,
	}

	// Setup TLS.
	if cfg.Etcd.TLS.Enabled {
		tlsCfg, err := makeEtcdTLSConfig(cfg)
		if err != nil {
			return nil, errors.Wrap(err, "etcd datasource initialization failure")
		}
		clientCfg.TLS = tlsCfg
	}

	// Create etcd client.
	client, err := etcdv3.New(clientCfg)
	if err != nil {
		return nil, errors.Wrap(err, "etcd datasource initialization failure")
	}

	// Set etcd namespace.
	ns := cfg.Etcd.Prefix
	client.KV = etcdns.NewKV(client.KV, ns+"/")
	client.Watcher = etcdns.NewWatcher(client.Watcher, ns+"/")
	client.Lease = etcdns.NewLease(client.Lease, ns+"/")

	return &EtcdDatasource{
		Config: cfg,
		Logger: cfg.Logger,
		Client: client,
	}, nil
}
