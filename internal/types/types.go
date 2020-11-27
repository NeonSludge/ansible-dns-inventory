package types

import (
	"encoding/json"

	"github.com/spf13/viper"
)

type (
	// Host attributes found in its TXT record.
	TXTAttrs struct {
		// Host operating system identifier.
		OS string `validate:"nonzero,safe"`
		// Host environment identifier.
		Env string `validate:"nonzero,safe"`
		// Host role identifier.
		Role string `validate:"nonzero,safe=list"`
		// Host service identifier.
		Srv string `validate:"safe=srv"`
	}

	// A JSON inventory representation of an Ansible group.
	InventoryGroup struct {
		// Group chilren.
		Children []string `json:"children,omitempty"`
		// Hosts belonging to this group.
		Hosts []string `json:"hosts,omitempty"`
	}
)

func (a *TXTAttrs) MarshalJSON() ([]byte, error) {
	attrs := make(map[string]string)

	attrs[viper.GetString("txt.keys.os")] = a.OS
	attrs[viper.GetString("txt.keys.env")] = a.Env
	attrs[viper.GetString("txt.keys.role")] = a.Role
	attrs[viper.GetString("txt.keys.srv")] = a.Srv

	return json.Marshal(attrs)
}

func (a *TXTAttrs) MarshalYAML() (interface{}, error) {
	attrs := make(map[string]string)

	attrs[viper.GetString("txt.keys.os")] = a.OS
	attrs[viper.GetString("txt.keys.env")] = a.Env
	attrs[viper.GetString("txt.keys.role")] = a.Role
	attrs[viper.GetString("txt.keys.srv")] = a.Srv

	return attrs, nil
}
