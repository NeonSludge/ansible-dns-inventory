package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
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
	// Ansible root group name.
	ansibleRootGroup string = "all"
)

type (
	// DNS server configuration.
	DNSServerConfig struct {
		// DNS server address.
		Address string
		// Network timeout for DNS requests.
		Timeout string
		// DNS zone list.
		Zones []string
		// Enable no-transfer data retrieval mode.
		NoTx bool
		// A host whose TXT records contain inventory data.
		NoTxHost string
		// Separator between a hostname and an attribute string in a TXT record.
		NoTxSeparator string
	}

	// TXT attribute parsing configuration
	TXTParseConfig struct {
		// Separator between k/v pairs found in TXT records.
		KvSeparator string
		// Separator between a key and a value.
		KvEquals string
		// Separator between elements of an Ansible group name.
		KeySeparator string
		// Key name of the attribute containing the host operating system identifier.
		KeyOs string
		// Key name of the attribute containing the host environment identifier.
		KeyEnv string
		// Key name of the attribute containing the host role identifier.
		KeyRole string
		// Key name of the attribute containing the host service identifier.
		KeySrv string
	}

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
		// Group Parent
		Parent *TreeNode
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

// Load DNS server configuration.
func (c *DNSServerConfig) load() {
	c.Address = viper.GetString("dns.server")
	c.Timeout = viper.GetString("dns.timeout")
	c.Zones = viper.GetStringSlice("dns.zones")
	c.NoTx = viper.GetBool("dns.notransfer.enabled")
	c.NoTxHost = viper.GetString("dns.notransfer.host")
	c.NoTxSeparator = viper.GetString("dns.notransfer.separator")
}

// Load TXT attribute parsing configuration
func (c *TXTParseConfig) load() {
	c.KvSeparator = viper.GetString("txt.kv.separator")
	c.KvEquals = viper.GetString("txt.kv.equalsign")
	c.KeySeparator = viper.GetString("txt.keys.separator")
	c.KeyOs = viper.GetString("txt.keys.os")
	c.KeyEnv = viper.GetString("txt.keys.env")
	c.KeyRole = viper.GetString("txt.keys.role")
	c.KeySrv = viper.GetString("txt.keys.srv")
}

// Validate host attributes.
func safeAttr(v interface{}, param string) error {
	value := reflect.ValueOf(v)
	if value.Kind() != reflect.String {
		return errors.New("safeAttr() can only validate strings")
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
		return fmt.Errorf("string '%s' is not a valid host attribute value (expr: %s)", value.String(), re)
	}

	return nil
}

// Load a list of hosts into the inventory tree, using this node as root.
func (n *TreeNode) importHosts(hosts map[string]*TXTAttrs, pc *TXTParseConfig) {
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
					envNode := n.addChild(env)

					// Role: root>environment>role
					roleGroup := fmt.Sprintf("%s%s%s", env, sep, role)
					roleGroupNode := envNode.addChild(roleGroup)

					// Service: root>environment>role>service[1]>...>service[N].
					srvGroup := roleGroup
					srvGroupNode := roleGroupNode
					for i, s := range strings.Split(srv, sep) {
						if len(s) > 0 && (i == 0 || env != ansibleRootGroup || attrs.Env == ansibleRootGroup) {
							group := fmt.Sprintf("%s%s%s", srvGroup, sep, s)
							node := srvGroupNode.addChild(group)
							srvGroup = group
							srvGroupNode = node
						}
					}

					// The last service group holds the host.
					srvGroupNode.addHost(host)

					// Host: root>environment>host
					hostGroupNode := envNode.addChild(fmt.Sprintf("%s%shost", env, sep))

					// OS: root>environment>host>os
					osGroupNode := hostGroupNode.addChild(fmt.Sprintf("%s%shost%s%s", env, sep, sep, attrs.OS))

					// The OS group holds the host.
					osGroupNode.addHost(host)
				}
			}
		}
	}
}

// Collect all ancestor nodes, starting from this node.
func (n *TreeNode) getAncestors() []*TreeNode {
	ancestors := make([]*TreeNode, 0)

	if len(n.Parent.Name) > 0 {
		// Add our parent.
		ancestors = append(ancestors, n.Parent)

		// Add ancestors.
		a := n.Parent.getAncestors()
		ancestors = append(ancestors, a...)
	}

	return ancestors
}

// Add a child of this node if it doesn't exist and return a pointer to the child.
func (n *TreeNode) addChild(name string) *TreeNode {
	if n.Name == name {
		return n
	}

	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}

	node := &TreeNode{Name: name, Parent: n, Hosts: make(map[string]bool)}
	n.Children = append(n.Children, node)

	return node
}

// Add a host to this node.
func (n *TreeNode) addHost(host string) {
	n.Hosts[host] = true
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

// Export the inventory tree to a map of hosts and groups the belong to, starting from this node.
func (n *TreeNode) exportHosts(hosts map[string][]string) {
	// Collect a list of unique group names for every host owned by this node.
	for host := range n.Hosts {
		collected := make(map[string]bool)
		result := make([]string, 0)

		// Add current node name.
		collected[n.Name] = true

		// Add all parent node names.
		parents := n.getAncestors()
		for _, parent := range parents {
			collected[parent.Name] = true
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

		// Add host to map.
		hosts[host] = result
	}

	// Process other nodes recursively.
	if len(n.Children) > 0 {
		for _, child := range n.Children {
			child.exportHosts(hosts)
		}
	}
}

// Export the inventory tree to a map of groups and hosts they own, starting from this node.
func (n *TreeNode) exportGroups(groups map[string][]string) {
	// Collect node hosts.
	hosts := make([]string, 0, len(n.Hosts))
	for host := range n.Hosts {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)

	// Add group to map
	groups[n.Name] = hosts

	// Process other nodes recursively.
	if len(n.Children) > 0 {
		for _, child := range n.Children {
			child.exportGroups(groups)
		}
	}
}

// Convert a map of strings into newline-delimited YAML.
func mapStrToNDYAML(m map[string][]string, mode string) ([]byte, error) {
	buf := new(bytes.Buffer)

	for key, value := range m {
		var yaml string

		switch mode {
		case "list":
			value = mapStr(value, strconv.Quote)
			yaml = fmt.Sprintf("[%s]", strings.Join(value, ","))
		default:
			yaml = fmt.Sprintf("\"%s\"", strings.Join(value, ","))
		}
		if _, err := buf.WriteString(fmt.Sprintf("\"%s\": %s\n", key, yaml)); err != nil {
			return buf.Bytes(), err
		}
	}

	return buf.Bytes(), nil
}

func mapAttrToNDYAML(m map[string]*TXTAttrs, pc *TXTParseConfig) ([]byte, error) {
	buf := new(bytes.Buffer)

	for key, value := range m {
		yaml := fmt.Sprintf("{\"%s\": \"%s\", \"%s\": \"%s\", \"%s\": \"%s\", \"%s\": \"%s\"}", pc.KeyOs, value.OS, pc.KeyEnv, value.Env, pc.KeyRole, value.Role, pc.KeySrv, value.Srv)

		if _, err := buf.WriteString(fmt.Sprintf("\"%s\": %s\n", key, yaml)); err != nil {
			return buf.Bytes(), err
		}
	}

	return buf.Bytes(), nil
}

// Acquire DNS records from a remote DNS server.
func getTXTRecords(c *DNSServerConfig) []dns.RR {
	records := make([]dns.RR, 0)

	for _, zone := range c.Zones {
		var rrs []dns.RR
		var err error

		if c.NoTx {
			rrs, err = getInventoryRecord(c.Address, zone, c.NoTxHost, c.Timeout)
		} else {
			rrs, err = transferZone(c.Address, zone, c.NoTxHost, c.Timeout)
		}

		if err != nil {
			log.Printf("[%s] skipping zone: %v", zone, err)
			continue
		}

		records = append(records, rrs...)
	}

	return records
}

// Perform a DNS zone transfer (AXFR), return the results.
func transferZone(server string, domain string, notxName string, timeout string) ([]dns.RR, error) {
	records := make([]dns.RR, 0)

	t, err := time.ParseDuration(timeout)
	if err != nil {
		return records, errors.Wrap(err, "zone transfer failed")
	}
	tx := &dns.Transfer{
		DialTimeout:  t,
		ReadTimeout:  t,
		WriteTimeout: t,
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

// Acquire TXT records of a special host (no-transfer mode).
func getInventoryRecord(server string, domain string, host string, timeout string) ([]dns.RR, error) {
	records := make([]dns.RR, 0)
	name := fmt.Sprintf("%s.%s", host, dns.Fqdn(domain))

	t, err := time.ParseDuration(timeout)
	if err != nil {
		return records, errors.Wrap(err, "inventory record loading failed")
	}
	client := &dns.Client{
		Timeout: t,
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

// Parse zone transfer results and create a map of hosts and their attributes.
func parseTXTRecords(records []dns.RR, dc *DNSServerConfig, pc *TXTParseConfig) map[string]*TXTAttrs {
	hosts := make(map[string]*TXTAttrs)

	for _, rr := range records {
		var name string
		var attrs *TXTAttrs
		var err error

		if dc.NoTx {
			name = strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), dc.NoTxSeparator)[0], ".")
			attrs, err = parseAttributes(strings.Split(dns.Field(rr, dnsRrTxtField), dc.NoTxSeparator)[1], pc)
		} else {
			name = strings.TrimSuffix(rr.Header().Name, ".")
			attrs, err = parseAttributes(dns.Field(rr, dnsRrTxtField), pc)
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
func parseAttributes(raw string, pc *TXTParseConfig) (*TXTAttrs, error) {
	attrs := &TXTAttrs{}
	items := strings.Split(raw, pc.KvSeparator)

	for _, item := range items {
		kv := strings.Split(item, pc.KvEquals)
		switch kv[0] {
		case pc.KeyOs:
			attrs.OS = kv[1]
		case pc.KeyEnv:
			attrs.Env = kv[1]
		case pc.KeyRole:
			attrs.Role = kv[1]
		case pc.KeySrv:
			attrs.Srv = kv[1]
		}
	}

	if err := validator.Validate(attrs); err != nil {
		return attrs, errors.Wrap(err, "attribute validation error")
	}

	return attrs, nil
}

// Apply a function to all elements in a slice of strings.
func mapStr(values []string, f func(string) string) []string {
	result := make([]string, len(values))

	for i, value := range values {
		result[i] = f(value)
	}

	return result
}

func init() {
	log.SetOutput(os.Stderr)

	// Load YAML configuration.
	path, ok := os.LookupEnv("ADI_CONFIG_FILE")
	if ok {
		// Load a specific config file.
		viper.SetConfigFile(path)
	} else {
		// Try to find the config file in standard loctions.
		viper.SetConfigName("ansible-dns-inventory")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.ansible")
		viper.AddConfigPath("/etc/ansible")
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(errors.Wrap(err, "failed to read config file"))
		}
	}

	// Setup environment variables handling.
	viper.SetEnvPrefix("adi")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

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

	// Setup validators.
	if err := validator.SetValidationFunc("safe", safeAttr); err != nil {
		panic(errors.Wrap(err, "validator initialization error"))
	}
}

func main() {
	listFlag := flag.Bool("list", false, "produce a JSON inventory for Ansible")
	hostsFlag := flag.Bool("hosts", false, "export hosts")
	attrsFlag := flag.Bool("attrs", false, "export host attributes")
	groupsFlag := flag.Bool("groups", false, "export groups")
	formatFlag := flag.String("format", "yaml", "select export format")
	hostFlag := flag.Bool("host", false, "a stub for Ansible")
	flag.Parse()

	if !*hostFlag {
		// Initialize and load configuration.
		dnsConfig := &DNSServerConfig{}
		parseConfig := &TXTParseConfig{}
		dnsConfig.load()
		parseConfig.load()

		// Acquire TXT records.
		records := getTXTRecords(dnsConfig)
		if len(records) == 0 {
			log.Fatal("empty TXT records list")
		}

		// Initialize the inventory tree.
		tree := &TreeNode{Name: ansibleRootGroup, Parent: &TreeNode{}, Children: make([]*TreeNode, 0), Hosts: make(map[string]bool)}

		// Load DNS records into the inventory tree.
		hosts := parseTXTRecords(records, dnsConfig, parseConfig)
		tree.importHosts(hosts, parseConfig)

		// Export the inventory tree in various formats.
		var bytes []byte
		var err error
		switch {
		case *listFlag:
			export := make(map[string]*InventoryGroup)

			// Export the inventory tree into a map.
			tree.exportInventory(export)

			// Marshal the map into a JSON representation of an Ansible inventory.
			bytes, err = json.Marshal(export)
		case *attrsFlag:
			bytes, err = mapAttrToNDYAML(hosts, parseConfig)
		default:
			export := make(map[string][]string)

			// Export the inventory tree into a map.
			if *hostsFlag {
				tree.exportHosts(export)
			} else if *groupsFlag {
				tree.exportGroups(export)
			}

			switch *formatFlag {
			case "json":
				bytes, err = json.Marshal(export)
			case "yaml-csv":
				bytes, err = mapStrToNDYAML(export, "csv")
			case "yaml-list":
				bytes, err = mapStrToNDYAML(export, "list")
			default:
				bytes, err = mapStrToNDYAML(export, "csv")
			}
		}

		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(bytes))
	} else {
		fmt.Println("{}")
	}
}
