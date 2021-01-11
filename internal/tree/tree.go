package tree

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/NeonSludge/ansible-dns-inventory/internal/config"
	"github.com/NeonSludge/ansible-dns-inventory/internal/types"
)

const (
	// Ansible root group name.
	ansibleRootGroup string = "all"
)

type (
	// Node represents an inventory tree node.
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

// MarshalJSON implements a custom JSON Marshaller for tree nodes.
func (n *Node) MarshalJSON() ([]byte, error) {
	// Collect node hosts.
	hosts := make([]string, 0, len(n.Hosts))
	for host := range n.Hosts {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	return json.Marshal(&ExportNode{
		Name:     n.Name,
		Children: n.Children,
		Hosts:    hosts,
	})
}

// MarshalYAML implements a custom YAML Marshaller for tree nodes.
func (n *Node) MarshalYAML() (interface{}, error) {
	// Collect node hosts.
	hosts := make([]string, 0, len(n.Hosts))
	for host := range n.Hosts {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	return &ExportNode{
		Name:     n.Name,
		Children: n.Children,
		Hosts:    hosts,
	}, nil
}

// ImportHosts loads a map of hosts and their attributes into the inventory tree, using this node as root.
func (n *Node) ImportHosts(hosts map[string][]*types.Attributes, cfg *config.Main) {
	sep := cfg.KeySeparator

	for host, attrs := range hosts {
		for _, attr := range attrs {
			// Create an environment list for this host. Add the root environment, if necessary.
			envs := make(map[string]bool)
			envs[attr.Env] = true
			envs[ansibleRootGroup] = true

			// Iterate the environments.
			for env := range envs {
				// Environment: root>environment
				envNode := n.AddChild(env)

				// Role: root>environment>role
				groupName := fmt.Sprintf("%s%s%s", env, sep, attr.Role)
				groupNode := envNode.AddChild(groupName)

				// Service: root>environment>role>service[1]>...>service[N].
				for i, srv := range strings.Split(attr.Srv, sep) {
					if len(srv) > 0 && (i == 0 || env != ansibleRootGroup || attr.Env == ansibleRootGroup) {
						groupName = fmt.Sprintf("%s%s%s", groupName, sep, srv)
						groupNode = groupNode.AddChild(groupName)
					}
				}

				// The last service group holds the host.
				groupNode.AddHost(host)

				// Special groups: [root_]<environment>_host, [root_]<environment>_host_<os>
				envNode.AddChild(fmt.Sprintf("%s%shost", env, sep)).AddChild(fmt.Sprintf("%s%shost%s%s", env, sep, sep, attr.OS)).AddHost(host)
			}
		}
	}
}

// GetAncestors returns all ancestor nodes, starting from this node.
func (n *Node) GetAncestors() []*Node {
	ancestors := make([]*Node, 0)

	if len(n.Parent.Name) > 0 {
		// Add our parent.
		ancestors = append(ancestors, n.Parent)

		// Add ancestors.
		a := n.Parent.GetAncestors()
		ancestors = append(ancestors, a...)
	}

	return ancestors
}

// GetAllHosts returns all hosts from descendant groups, starting from this node.
func (n *Node) GetAllHosts() map[string]bool {
	result := make(map[string]bool)

	// Add our own hosts.
	if len(n.Hosts) > 0 {
		for host := range n.Hosts {
			result[host] = true
		}
	}

	// Add hosts of our descendants.
	if len(n.Children) > 0 {
		for _, child := range n.Children {
			for host := range child.GetAllHosts() {
				result[host] = true
			}
		}
	}

	return result
}

// AddChild adds a child to this node if it doesn't exist and return a pointer to the child.
func (n *Node) AddChild(name string) *Node {
	if n.Name == name {
		return n
	}

	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}

	node := &Node{Name: name, Parent: n, Hosts: make(map[string]bool)}
	n.Children = append(n.Children, node)

	return node
}

// AddHost adds a host to this node.
func (n *Node) AddHost(host string) {
	n.Hosts[host] = true
}

// SortChildren sorts children by name recursively, starting from this node.
func (n *Node) SortChildren() {
	if len(n.Children) > 0 {
		sort.Slice(n.Children, func(i, j int) bool { return n.Children[i].Name < n.Children[j].Name })

		for _, child := range n.Children {
			child.SortChildren()
		}
	}
}

// ExportInventory exports the inventory tree into a map ready to be marshalled into a JSON representation of an Ansible inventory, starting from this node.
func (n *Node) ExportInventory(inventory map[string]*types.InventoryGroup) {
	// Collect node children.
	children := make([]string, 0, len(n.Children))
	for _, child := range n.Children {
		children = append(children, child.Name)
	}

	// Collect node hosts.
	hosts := make([]string, 0, len(n.Hosts))
	for host := range n.Hosts {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	// Put this node into the map.
	inventory[n.Name] = &types.InventoryGroup{Children: children, Hosts: hosts}

	// Process other nodes recursively.
	if len(n.Children) > 0 {
		for _, child := range n.Children {
			child.ExportInventory(inventory)
		}
	}
}

// ExportHosts exports the inventory tree into a map of hosts and groups they belong to, starting from this node.
func (n *Node) ExportHosts(hosts map[string][]string) {
	// Collect a list of unique group names for every host owned by this node.
	for host := range n.Hosts {
		collected := make(map[string]bool)
		result := make([]string, 0)

		// Add current node name.
		collected[n.Name] = true

		// Add all parent node names.
		ancestors := n.GetAncestors()
		for _, ancestor := range ancestors {
			collected[ancestor.Name] = true
		}

		// Get current list for host.
		current := hosts[host]
		for _, name := range current {
			collected[name] = true
		}

		// Compile the final result.
		for name := range collected {
			result = append(result, name)
		}
		sort.Strings(result)

		// Add host to map.
		hosts[host] = result
	}

	// Process other nodes recursively.
	if len(n.Children) > 0 {
		for _, child := range n.Children {
			child.ExportHosts(hosts)
		}
	}
}

// ExportGroups exports the inventory tree into a map of groups and hosts they contain, starting from this node.
func (n *Node) ExportGroups(groups map[string][]string) {
	hosts := make([]string, 0)

	// Get all hosts that this group contains.
	for host := range n.GetAllHosts() {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	// Add group to map
	groups[n.Name] = hosts

	// Process other nodes recursively.
	if len(n.Children) > 0 {
		for _, child := range n.Children {
			child.ExportGroups(groups)
		}
	}
}

// New returns an empty inventory tree
func New() *Node {
	return &Node{Name: ansibleRootGroup, Parent: &Node{}, Children: make([]*Node, 0), Hosts: make(map[string]bool)}
}
