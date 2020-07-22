package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"gopkg.in/validator.v2"
)

const (
	// DNS TXT record type.
	dnsRrTxtType uint16 = 16
	// Number of the field that contains the TXT record value.
	dnsRrTxtField int = 1
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

	// Inventory tree node. Represents an Ansible group.
	TreeNode struct {
		// Group name.
		Name string
		// Group children.
		Children []*TreeNode
		// Hosts belonging to this group.
		Hosts map[string]bool
	}

	// A JSON inventory representation of an Ansible group.
	InventoryGroup struct {
		// Group chilren.
		Children []string `json:"children,omitempty"`
		// Hosts belonging to this group.
		Hosts []string `json:"hosts,omitempty"`
	}
)

// Validate host attributes.
func validateAttribute(v interface{}, param string) error {
	value := reflect.ValueOf(v)
	if value.Kind() != reflect.String {
		return errors.New("ansiblename only validates strings")
	}

	separator := viper.GetString("txt.keys.separator")
	re := "^[A-Za-z0-9"

	// Deprecated: using '-' in group names.
	if separator == "-" {
		re += "\\_"
	}

	switch param {
	case "srv":
		re += "\\,\\" + separator + "]*$"
	case "list":
		re += "\\," + "]*$"
	default:
		re += "]*$"
	}

	pattern, err := regexp.Compile(re)
	if err != nil {
		return errors.Wrap(err, "regex compilation error")
	}

	if !pattern.MatchString(value.String()) {
		return fmt.Errorf("string '%s' is not a valid Ansible group name segment (expr: %s)", value.String(), re)
	}

	return nil
}

// Load a list of hosts into the inventory tree, starting from this node.
func (n *TreeNode) loadHosts(hosts map[string]*TXTAttrs) {
	separator := viper.GetString("txt.keys.separator")

	for host, attrs := range hosts {
		// Automatically create pseudo-groups for the "all" environment.
		envs := make(map[string]bool)
		envs[attrs.Env] = true
		envs["all"] = true

		for env := range envs {
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
					for i, s := range strings.Split(srv, separator) {
						if len(s) > 0 && (i == 0 || env != "all" || attrs.Env == "all") {
							group := fmt.Sprintf("%s%s%s", srvGroup, separator, s)
							n.addGroup(srvGroup, group)
							srvGroup = group
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
	if parent != name {
		if g := n.findByName(name); g == nil {
			// Add the group only if it doesn't exist.
			if pg := n.findByName(parent); pg != nil {
				// If the parent group is found, add the group as a child.
				pg.Children = append(pg.Children, &TreeNode{Name: name, Hosts: make(map[string]bool)})
			} else {
				// If the parent group is not found, add the group as a child to the current node.
				n.Children = append(n.Children, &TreeNode{Name: name, Hosts: make(map[string]bool)})
			}
		}
	}
}

// Add a host to a group in the inventory tree.
func (n *TreeNode) addHost(group string, name string) {
	if g := n.findByName(group); g != nil {
		// If the group is found, add the host.
		g.Hosts[name] = true
	} else {
		// If the group is not found, add the host to the current node.
		n.Hosts[name] = true
	}
}

// Export the inventory tree to a map ready to be marshalled into a JSON representation of an Ansible inventory, starting from this node.
func (n *TreeNode) exportInventory(inventory map[string]*InventoryGroup) {
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
	inventory[n.Name] = &InventoryGroup{Children: children, Hosts: hosts}

	// Process other nodes recursively.
	if len(n.Children) > 0 {
		for _, child := range n.Children {
			child.exportInventory(inventory)
		}
	}
}

// Perform a DNS zone transfer (AXFR), return the results.
func transferZone(domain string, server string) ([]dns.RR, error) {
	records := make([]dns.RR, 0)
	notxName := viper.GetString("dns.notransfer.host")

	timeout, err := time.ParseDuration(viper.GetString("dns.timeout"))
	if err != nil {
		return records, errors.Wrap(err, "zone transfer failed")
	}
	tx := &dns.Transfer{
		DialTimeout:  timeout,
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}

	msg := new(dns.Msg)
	msg.SetAxfr(dns.Fqdn(domain))

	// Perform the transfer.
	c, err := tx.In(msg, server)
	if err != nil {
		return records, errors.Wrap(err, "zone transfer failed")
	}

	// Process transferred records. Ignore anything that is not a TXT recordd. Ignore the special inventory record as well.
	for e := range c {
		for _, rr := range e.RR {
			if rr.Header().Rrtype == dnsRrTxtType && rr.Header().Name != dns.Fqdn(notxName+"."+domain) {
				records = append(records, rr)
			}
		}
	}
	if len(records) == 0 {
		return records, errors.Wrap(fmt.Errorf("no TXT records found: %s", domain), "zone transfer failed")
	}

	return records, nil
}

func getInventoryRecord(host string, domain string, server string) ([]dns.RR, error) {
	records := make([]dns.RR, 0)
	name := fmt.Sprintf("%s.%s", host, dns.Fqdn(domain))

	timeout, err := time.ParseDuration(viper.GetString("dns.timeout"))
	if err != nil {
		return records, errors.Wrap(err, "inventory record loading failed")
	}
	client := &dns.Client{
		Timeout: timeout,
	}

	msg := new(dns.Msg)
	msg.SetQuestion(name, dns.TypeTXT)

	rx, _, err := client.Exchange(msg, server)
	if err != nil {
		return records, errors.Wrap(err, "inventory record loading failed")
	} else if len(rx.Answer) == 0 {
		return records, errors.Wrap(fmt.Errorf("not found: %s", name), "inventory record loading failed")
	}
	records = rx.Answer

	return records, nil
}

// Parse zone transfer results and create a list of hosts and their attributes.
func parseTXTRecords(records []dns.RR) map[string]*TXTAttrs {
	hosts := make(map[string]*TXTAttrs)
	notx := viper.GetBool("dns.notransfer.enabled")
	separator := viper.GetString("dns.notransfer.separator")

	for _, rr := range records {
		var name string
		var attrs *TXTAttrs
		var err error

		if notx {
			name = strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), separator)[0], ".")
			attrs, err = parseAttributes(strings.Split(dns.Field(rr, dnsRrTxtField), separator)[1])
		} else {
			name = strings.TrimSuffix(rr.Header().Name, ".")
			attrs, err = parseAttributes(dns.Field(rr, dnsRrTxtField))
		}

		if err != nil {
			log.Printf("[%s] skipping host: %v", name, err)
			continue
		}

		_, ok := hosts[name] // First host record wins.
		if !ok {
			hosts[name] = attrs
		}
	}

	return hosts
}

// Parse host attributes.
func parseAttributes(raw string) (*TXTAttrs, error) {
	separator := viper.GetString("txt.kv.separator")
	equalsign := viper.GetString("txt.kv.equalsign")

	keyOS := viper.GetString("txt.keys.os")
	keyEnv := viper.GetString("txt.keys.env")
	keyRole := viper.GetString("txt.keys.role")
	keySrv := viper.GetString("txt.keys.srv")

	attrs := &TXTAttrs{}
	items := strings.Split(raw, separator)

	for _, item := range items {
		kv := strings.Split(item, equalsign)
		switch kv[0] {
		case keyOS:
			attrs.OS = kv[1]
		case keyEnv:
			attrs.Env = kv[1]
		case keyRole:
			attrs.Role = kv[1]
		case keySrv:
			attrs.Srv = kv[1]
		}
	}

	if err := validator.Validate(attrs); err != nil {
		return attrs, errors.Wrap(err, "attribute validation error")
	}

	return attrs, nil
}

func init() {
	log.SetOutput(os.Stderr)

	// Load YAML configuration.
	viper.SetConfigName("ansible-dns-inventory")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.ansible")
	viper.AddConfigPath("/etc/ansible")

	if err := viper.ReadInConfig(); err != nil {
		panic(errors.Wrap(err, "failed to read config file"))
	}

	// Set defaults.
	viper.SetDefault("dns.server", "127.0.0.1:53")
	viper.SetDefault("dns.timeout", "30s")
	viper.SetDefault("dns.zones", []string{"server.local."})

	viper.SetDefault("dns.notransfer.enabled", false)
	viper.SetDefault("dns.notransfer.host", "ansible-dns-inventory")
	viper.SetDefault("dns.notransfer.separator", ":")

	viper.SetDefault("txt.kv.separator", ";")
	viper.SetDefault("txt.kv.equalsign", "=")

	viper.SetDefault("txt.keys.separator", "_")
	viper.SetDefault("txt.keys.os", "OS")
	viper.SetDefault("txt.keys.env", "ENV")
	viper.SetDefault("txt.keys.role", "ROLE")
	viper.SetDefault("txt.keys.srv", "SRV")

	if err := validator.SetValidationFunc("safe", validateAttribute); err != nil {
		panic(errors.Wrap(err, "'safe' validator initialization error"))
	}
}

func main() {
	listFlag := flag.Bool("list", false, "list hosts")
	hostFlag := flag.Bool("host", false, "a stub for Ansible")
	flag.Parse()

	if *listFlag {
		// Initialize the inventory tree.
		tree := &TreeNode{Name: "all", Hosts: make(map[string]bool)}
		server := viper.GetString("dns.server")
		notx := viper.GetBool("dns.notransfer.enabled")
		notxName := viper.GetString("dns.notransfer.host")

		// Transfer all of the zones, load results into the inventory tree.
		for _, zone := range viper.GetStringSlice("dns.zones") {
			var records []dns.RR
			var err error

			if notx {
				records, err = getInventoryRecord(notxName, zone, server)
			} else {
				records, err = transferZone(zone, server)
			}

			if err != nil {
				log.Printf("[%s] skipping zone: %v", zone, err)
				continue
			}

			tree.loadHosts(parseTXTRecords(records))
		}

		if len(tree.Children) == 0 {
			log.Fatalln("empty inventory tree")
		}

		// Export the tree into a map.
		inventory := make(map[string]*InventoryGroup)
		tree.exportInventory(inventory)

		// Marshal the map into a JSON representation of an Ansible inventory.
		jsonInventory, err := json.Marshal(inventory)
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Println(string(jsonInventory))
	} else if *hostFlag {
		fmt.Println("{}")
	}
}
