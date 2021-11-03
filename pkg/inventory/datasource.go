package inventory

import (
	"github.com/pkg/errors"
)

// NewDatasource creates a datasource based on the inventory configuration.
func NewDatasource(cfg *Config) (Datasource, error) {
	// Select datasource implementation.
	switch cfg.Datasource {
	case DNSDatasourceType:
		return NewDNSDatasource(cfg)
	case EtcdDatasourceType:
		return NewEtcdDatasource(cfg)
	default:
		return nil, errors.Errorf("unknown datasource type: %s", cfg.Datasource)
	}
}
