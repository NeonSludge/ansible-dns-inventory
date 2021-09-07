package types

import "time"

type (
	// Config represents a configuration object.
	Config interface {
		// GetString returns a 'string' configuration parameter value.
		GetString(key string) string
		// GetStringSlice returns a '[]string' configuration parameter value.
		GetStringSlice(key string) []string
		// GetBool returns a 'bool' configuration parameter value.
		GetBool(key string) bool
		// GetInt returns an 'int' configuration parameter value.
		GetInt(key string) int
		// GetDuration returns a 'time.Duration' configuration parameter value.
		GetDuration(key string) time.Duration
	}

	Datasource interface {
		// GetAllRecords returns all host records.
		GetAllRecords() ([]*Record, error)
		// GetHostRecords returns all records for a specific host.
		GetHostRecords(host string) ([]*Record, error)
		// Close closes datasource clients and performs other housekeeping.
		Close()
	}

	Record struct {
		// Host name.
		Hostname string
		// Host attributes.
		Attributes string
	}
)
