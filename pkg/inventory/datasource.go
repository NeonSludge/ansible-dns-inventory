package inventory

import (
	"github.com/pkg/errors"
)

// NewDatasource creates a datasource based on the inventory configuration.
func NewDatasource(cfg *Config, log Logger) (Datasource, error) {
	// Select datasource implementation.
	switch cfg.Datasource {
	case DNSDatasourceType:
		return NewDNSDatasource(cfg, log)
	case EtcdDatasourceType:
		return NewEtcdDatasource(cfg, log)
	default:
		return nil, errors.Errorf("unknown datasource type: %s", cfg.Datasource)
	}
}
