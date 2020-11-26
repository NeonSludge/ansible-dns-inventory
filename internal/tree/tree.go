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
	// Inventory tree node. Represents an Ansible group.
	Node struct {
		// Group name.
		Name string
		// Group Parent
		Parent *Node
		// Group children.
		Children []*Node
		// Hosts belonging to this group.
		Hosts map[string]bool
	}

	// Inventory tree node for the tree export mode.
	ExportNode struct {
		// Group name.
		Name string
		// Group Parent
		Parent *Node
		// Group children.
		Children []*Node
		// Hosts belonging to this group.
		Hosts []string
	}
)

// MarshalJSON implements a custom Marshaller for tree nodes.
func (n *Node) MarshalJSON() ([]byte, error) {
	// Collect node hosts.
	hosts := make([]string, 0, len(n.Hosts))
	for host := range n.Hosts {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	return json.Marshal(&ExportNode{
		Name:     n.Name,
		Parent:   n.Parent,
		Children: n.Children,
		Hosts:    hosts,
	})
}

// Load a list of hosts into the inventory tree, using this node as root.
func (n *Node) ImportHosts(hosts map[string]*types.TXTAttrs, pc *config.Parse) {
	sep := pc.KeySeparator

	for host, attrs := range hosts {
		// Create an environment list for this host. Add the root environment, if necessary.
		envs := make(map[string]bool)
		envs[attrs.Env] = true
		envs[ansibleRootGroup] = true

		// Iterate the environments.
		for env := range envs {
			// Iterate the roles.
			for _, role := range strings.Split(attrs.Role, ",") {
				// Iterate the services.
				for _, srv := range strings.Split(attrs.Srv, ",") {
					// Environment: root>environment
					envNode := n.AddChild(env)

					// Role: root>environment>role
					roleGroup := fmt.Sprintf("%s%s%s", env, sep, role)
					roleGroupNode := envNode.AddChild(roleGroup)

					// Service: root>environment>role>service[1]>...>service[N].
					srvGroup := roleGroup
					srvGroupNode := roleGroupNode
					for i, s := range strings.Split(srv, sep) {
						if len(s) > 0 && (i == 0 || env != ansibleRootGroup || attrs.Env == ansibleRootGroup) {
							group := fmt.Sprintf("%s%s%s", srvGroup, sep, s)
							node := srvGroupNode.AddChild(group)
							srvGroup = group
							srvGroupNode = node
						}
					}

					// The last service group holds the host.
					srvGroupNode.AddHost(host)

					// Host: root>environment>host
					hostGroupNode := envNode.AddChild(fmt.Sprintf("%s%shost", env, sep))

					// OS: root>environment>host>os
					osGroupNode := hostGroupNode.AddChild(fmt.Sprintf("%s%shost%s%s", env, sep, sep, attrs.OS))

					// The OS group holds the host.
					osGroupNode.AddHost(host)
				}
			}
		}
	}
}

// Collect all ancestor nodes, starting from this node.
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

// Collect all hosts from descendant groups, starting from this node.
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

// Add a child of this node if it doesn't exist and return a pointer to the child.
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

// Add a host to this node.
func (n *Node) AddHost(host string) {
	n.Hosts[host] = true
}

// Export the inventory tree to a map ready to be marshalled into a JSON representation of an Ansible inventory, starting from this node.
func (n *Node) ExportInventory(inventory map[string]*types.InventoryGroup) {
	// Collect node children.
	children := make([]string, 0, len(n.Children))
	for _, child := range n.Children {
		children = append(children, child.Name)
	}
	sort.Strings(children)

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

// Export the inventory tree to a map of hosts and groups the belong to, starting from this node.
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

// Export the inventory tree to a map of groups and hosts they contain, starting from this node.
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
