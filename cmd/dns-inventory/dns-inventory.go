package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

const (
	// DNS TXT record type.
	dnsRrTxtType uint16 = 16
	// Number of the field that contains the TXT record value.
	dnsRrTxtField int = 1
)

type (
	// DNS zone transfer result.
	DNSZone struct {
		// DNS records that were received during the transfer.
		Records []dns.RR
	}

	// Host attributes found in its TXT record.
	TXTAttrs struct {
		// Host operating system identifier.
		OS string
		// Host environment identifier.
		Env string
		// Host role identifier.
		Role string
		// Host service identifier.
		Srv string
	}

	// Inventory tree node. Represents an Ansible group.
	TreeNode struct {
		// Group name.
		Name string
		// Group children.
		Children []*TreeNode
		// Hosts belonging to this group.
		Hosts []string
	}

	// A JSON inventory representation of an Ansible group.
	InventoryGroup struct {
		// Group chilren.
		Children []string `json:"children,omitempty"`
		// Hosts belonging to this group.
		Hosts []string `json:"hosts,omitempty"`
	}
)

// Load a list of hosts into the inventory tree, starting from this node.
func (n *TreeNode) loadHosts(hosts map[string]*TXTAttrs) {
	separator := viper.GetString("txt.keys.separator")

	for host, attrs := range hosts {
		// Automatically create pseudo-groups for the "all" environment.
		for _, env := range []string{attrs.Env, "all"} {
			// A host can have several roles.
			for _, role := range strings.Split(attrs.Role, ",") {
				// A host can have several services.
				for _, srv := range strings.Split(attrs.Srv, ",") {
					// Add the environment and role groups
					roleGroup := fmt.Sprintf("%s%s%s", env, separator, role)

					n.addGroup(n.Name, env)
					n.addGroup(env, roleGroup)

					// Add service groups.
					srvGroup := roleGroup
					for _, s := range strings.Split(srv, separator) {
						if len(s) > 0 {
							group := fmt.Sprintf("%s%s%s", srvGroup, separator, s)
							n.addGroup(srvGroup, group)
							srvGroup = fmt.Sprintf("%s%s%s", srvGroup, separator, s)
						}
					}

					// Add the host itself to the last service group.
					n.addHost(srvGroup, host)
				}
			}

			// Add OS-based groups.
			hostGroup := fmt.Sprintf("%s%shost", env, separator)
			osGroup := fmt.Sprintf("%s%shost%s%s", env, separator, separator, attrs.OS)
			n.addGroup(env, hostGroup)
			n.addGroup(hostGroup, osGroup)
			n.addHost(osGroup, host)
		}
	}
}

// Find an inventory tree node by its name, starting from this node.
func (n *TreeNode) findByName(name string) *TreeNode {
	if n.Name == name {
		// Node found.
		return n
	}

	// Process other nodes recursively.
	if len(n.Children) > 0 {
		for _, child := range n.Children {
			if g := child.findByName(name); g != nil {
				// Node found.
				return g
			}
		}
	}

	// Node not found.
	return nil
}

// Add a group to the inventory tree as a child of the specified parent.
func (n *TreeNode) addGroup(parent string, name string) {
	if g := n.findByName(name); g == nil {
		// Add the group only if it doesn't exist.
		if pg := n.findByName(parent); pg != nil {
			// If the parent group is found, add the group as a child.
			pg.Children = append(pg.Children, &TreeNode{Name: name})
		} else {
			// If the parent group is not found, add the group as a child to the current node.
			n.Children = append(n.Children, &TreeNode{Name: name})
		}
	}
}

// Add a host to a group in the inventory tree.
func (n *TreeNode) addHost(group string, name string) {
	if g := n.findByName(group); g != nil {
		// If the group is found, add the host.
		g.Hosts = append(g.Hosts, name)
	} else {
		// If the group is not found, add the host to the current node.
		n.Hosts = append(n.Hosts, name)
	}
}

// Export the inventory tree to a map ready to be marshalled into a JSON representation of an Ansible inventory, starting from this node.
func (n *TreeNode) exportInventory(inventory map[string]*InventoryGroup) {
	// Collect children of this node.
	children := []string{}
	for _, child := range n.Children {
		children = append(children, child.Name)
	}

	// Put this node into the map.
	inventory[n.Name] = &InventoryGroup{Children: children, Hosts: n.Hosts}

	// Process other nodes recursively.
	if len(n.Children) > 0 {
		for _, child := range n.Children {
			child.exportInventory(inventory)
		}
	}
}

// Perform a DNS zone transfer (AXFR), return the results.
func transferZone(domain string, server string) (*DNSZone, error) {
	zone := &DNSZone{}

	timeout, err := time.ParseDuration(viper.GetString("dns.timeout"))
	if err != nil {
		return zone, errors.Wrap(err, "zone transfer failed")
	}
	tx := &dns.Transfer{
		DialTimeout:  timeout,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}

	msg := new(dns.Msg)
	msg.SetAxfr(domain)

	// Perform the transfer.
	c, err := tx.In(msg, server)
	if err != nil {
		return zone, errors.Wrap(err, "zone transfer failed")
	}

	// Process transferred records. Ignore anything that is not a TXT record.
	for e := range c {
		for _, rr := range e.RR {
			if rr.Header().Rrtype == dnsRrTxtType {
				zone.Records = append(zone.Records, rr)
			}
		}
	}

	return zone, nil
}

// Parse zone transfer results and create a list of hosts and their attributes.
func makeHosts(records []dns.RR) map[string]*TXTAttrs {
	hosts := make(map[string]*TXTAttrs)
	for _, rr := range records {
		name := strings.TrimSuffix(rr.Header().Name, ".")
		_, ok := hosts[name] // First host record wins.
		if !ok {
			txt := parseTXT(dns.Field(rr, dnsRrTxtField))
			hosts[name] = txt
		}
	}

	return hosts
}

// Parse a raw TXT record
func parseTXT(raw string) *TXTAttrs {
	txt := &TXTAttrs{}
	items := strings.Split(raw, viper.GetString("txt.kv.separator"))

	for _, item := range items {
		kv := strings.Split(item, viper.GetString("txt.kv.equalsign"))
		if len(kv[1]) > 0 {
			// Skip keys if they are unknown or their values are empty.
			switch kv[0] {
			case viper.GetString("txt.keys.os"):
				txt.OS = kv[1]
			case viper.GetString("txt.keys.env"):
				txt.Env = kv[1]
			case viper.GetString("txt.keys.role"):
				txt.Role = kv[1]
			case viper.GetString("txt.keys.srv"):
				txt.Srv = kv[1]
			}
		}
	}

	return txt
}

func init() {
	// Load YAML configuration.
	viper.SetConfigName("ansible-dns-inventory")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.ansible")
	viper.AddConfigPath("/etc/ansible")

	err := viper.ReadInConfig()
	if err != nil {
		panic(errors.Wrap(err, "failed to read config file"))
	}

	// Set defaults.
	viper.SetDefault("dns.server", "127.0.0.1:53")
	viper.SetDefault("dns.timeout", "30s")
	viper.SetDefault("dns.zones", []string{"server.local."})

	viper.SetDefault("txt.kv.separator", ";")
	viper.SetDefault("txt.kv.equalsign", "=")

	viper.SetDefault("txt.keys.separator", "_")

	viper.SetDefault("txt.keys.os", "OS")
	viper.SetDefault("txt.keys.env", "ENV")
	viper.SetDefault("txt.keys.role", "ROLE")
	viper.SetDefault("txt.keys.srv", "SRV")
}

func main() {
	listFlag := flag.Bool("list", false, "list hosts")
	hostFlag := flag.Bool("host", false, "a stub for Ansible")
	flag.Parse()

	if *listFlag {
		// Initialize the inventory tree.
		tree := &TreeNode{Name: "all"}

		// Transfer all of the zones, load results into the inventory tree.
		for _, zone := range viper.GetStringSlice("dns.zones") {
			dnsZone, err := transferZone(zone, viper.GetString("dns.server"))
			if err != nil {
				panic(err)
			}

			tree.loadHosts(makeHosts(dnsZone.Records))
		}

		// Export the tree into a map.
		inventory := make(map[string]*InventoryGroup)
		tree.exportInventory(inventory)

		// Marshal the map into a JSON representation of an Ansible inventory.
		jsonInventory, err := json.Marshal(inventory)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(jsonInventory))
	} else if *hostFlag {
		fmt.Println("{}")
	}
}
