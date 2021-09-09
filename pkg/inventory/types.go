package inventory

import (
	"encoding/json"

	"github.com/NeonSludge/ansible-dns-inventory/pkg/types"
)

var ADIHostAttributeNames map[string]string
var ADITxtKeysSeparator string

type (
	Inventory struct {
		// Inventory configuration.
		Config *types.InventoryConfig
		// Inventory datasource.
		Datasource types.InventoryDatasource
		// Inventory tree.
		Tree *Node
	}

	// HostAttributes represents host attributes found in TXT records.
	HostAttributes struct {
		// Host operating system identifier.
		OS string `validate:"nonzero,safe"`
		// Host environment identifier.
		Env string `validate:"nonzero,safe"`
		// Host role identifier.
		Role string `validate:"nonzero,safe=list"`
		// Host service identifier.
		Srv string `validate:"safe=srv"`
		// Host variables
		Vars string `validate:"safe=vars"`
	}

	// AnsibleGroup is an Ansible group ready to be marshalled into a JSON representation.
	AnsibleGroup struct {
		// Group chilren.
		Children []string `json:"children,omitempty"`
		// Hosts belonging to this group.
		Hosts []string `json:"hosts,omitempty"`
	}

	// Node represents and inventory tree node.
	Node struct {
		// Group name.
		Name string
		// Group Parent
		Parent *Node `json:"-" yaml:"-"`
		// Group children.
		Children []*Node
		// Hosts belonging to this group.
		Hosts map[string]bool
	}

	// ExportNode represents an inventory tree node for the tree export mode.
	ExportNode struct {
		// Group name.
		Name string `json:"name" yaml:"name"`
		// Group children.
		Children []*Node `json:"children" yaml:"children"`
		// Hosts belonging to this group.
		Hosts []string `json:"hosts" yaml:"hosts"`
	}
)

// MarshalJSON implements a custom JSON Marshaller for host attributes.
func (a *HostAttributes) MarshalJSON() ([]byte, error) {
	attrs := make(map[string]string)

	attrs[ADIHostAttributeNames["OS"]] = a.OS
	attrs[ADIHostAttributeNames["ENV"]] = a.Env
	attrs[ADIHostAttributeNames["ROLE"]] = a.Role
	attrs[ADIHostAttributeNames["SRV"]] = a.Srv
	attrs[ADIHostAttributeNames["VARS"]] = a.Vars

	return json.Marshal(attrs)
}

// MarshalYAML implements a custom YAML Marshaller for host attributes.
func (a *HostAttributes) MarshalYAML() (interface{}, error) {
	attrs := make(map[string]string)

	attrs[ADIHostAttributeNames["OS"]] = a.OS
	attrs[ADIHostAttributeNames["ENV"]] = a.Env
	attrs[ADIHostAttributeNames["ROLE"]] = a.Role
	attrs[ADIHostAttributeNames["SRV"]] = a.Srv
	attrs[ADIHostAttributeNames["VARS"]] = a.Vars

	return attrs, nil
}
