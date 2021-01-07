package types

import (
	"encoding/json"

	"github.com/spf13/viper"
)

type (
	// Attributes represents host attributes found in TXT records.
	Attributes struct {
		// Host operating system identifier.
		OS string `validate:"nonzero,safe"`
		// Host environment identifier.
		Env string `validate:"nonzero,safe"`
		// Host role identifier.
		Role string `validate:"nonzero,safe=list"`
		// Host service identifier.
		Srv string `validate:"safe=srv"`
	}

	// InventoryGroup is an Ansible group ready to be marshalled into a JSON representation.
	InventoryGroup struct {
		// Group chilren.
		Children []string `json:"children,omitempty"`
		// Hosts belonging to this group.
		Hosts []string `json:"hosts,omitempty"`
	}
)

// MarshalJSON implements a custom JSON Marshaller for host attributes.
func (a *Attributes) MarshalJSON() ([]byte, error) {
	attrs := make(map[string]string)

	attrs[viper.GetString("txt.keys.os")] = a.OS
	attrs[viper.GetString("txt.keys.env")] = a.Env
	attrs[viper.GetString("txt.keys.role")] = a.Role
	attrs[viper.GetString("txt.keys.srv")] = a.Srv

	return json.Marshal(attrs)
}

// MarshalYAML implements a custom YAML Marshaller for host attributes.
func (a *Attributes) MarshalYAML() (interface{}, error) {
	attrs := make(map[string]string)

	attrs[viper.GetString("txt.keys.os")] = a.OS
	attrs[viper.GetString("txt.keys.env")] = a.Env
	attrs[viper.GetString("txt.keys.role")] = a.Role
	attrs[viper.GetString("txt.keys.srv")] = a.Srv

	return attrs, nil
}
