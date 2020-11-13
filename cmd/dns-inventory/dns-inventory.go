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

// Validate host attributes.
func validateAttr(v interface{}, param string) error {
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
func (n *TreeNode) importHosts(hosts map[string]*TXTAttrs) {
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
					// Insert the first node into the tree.
					envNode := n.addChild(env)

					// Add
					roleGroup := fmt.Sprintf("%s%s%s", env, separator, role)
					roleGroupNode := envNode.addChild(roleGroup)

					// Add service groups.
					srvGroup := roleGroup
					srvGroupNode := roleGroupNode
					for i, s := range strings.Split(srv, separator) {
						if len(s) > 0 && (i == 0 || env != "all" || attrs.Env == "all") {
							group := fmt.Sprintf("%s%s%s", srvGroup, separator, s)
							node := srvGroupNode.addChild(group)
							srvGroup = group
							srvGroupNode = node
						}
					}

					// Add the host itself to the last service group.
					srvGroupNode.addHost(host)

					// Add OS-based groups.
					hostGroup := fmt.Sprintf("%s%shost", env, separator)
					osGroup := fmt.Sprintf("%s%shost%s%s", env, separator, separator, attrs.OS)
					hostGroupNode := envNode.addChild(hostGroup)
					osGroupNode := hostGroupNode.addChild(osGroup)

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

// Add a child of this node if it doesn't exist.
func (n *TreeNode) addChild(name string) *TreeNode {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}

	node := &TreeNode{Name: name, Parent: n, Hosts: make(map[string]bool)}
	n.Children = append(n.Children, node)

	return node
}

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

// Export the inventory to a map ready to be marshalled into a YAML file that maps hosts to groups they belong to, starting from this node.
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

		hosts[host] = result
	}

	// Process other nodes recursively.
	if len(n.Children) > 0 {
		for _, child := range n.Children {
			child.exportHosts(hosts)
		}
	}
}

// Convert a hosts map into newline-delimited YAML.
func hostsToNDYAML(hosts map[string][]string, mode string) ([]byte, error) {
	buf := new(bytes.Buffer)

	for host, groups := range hosts {
		var groupsYAML string

		switch mode {
		case "list":
			groups = Map(groups, strconv.Quote)
			groupsYAML = fmt.Sprintf("[%s]", strings.Join(groups, ","))
		default:
			groupsYAML = fmt.Sprintf("\"%s\"", strings.Join(groups, ","))
		}
		if _, err := buf.WriteString(fmt.Sprintf("\"%s\": %s\n", host, groupsYAML)); err != nil {
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

// Parse zone transfer results and create a list of hosts and their attributes.
func parseTXTRecords(records []dns.RR, notx bool, notxSplit string) map[string]*TXTAttrs {
	hosts := make(map[string]*TXTAttrs)

	for _, rr := range records {
		var name string
		var attrs *TXTAttrs
		var err error

		if notx {
			name = strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), notxSplit)[0], ".")
			attrs, err = parseAttributes(strings.Split(dns.Field(rr, dnsRrTxtField), notxSplit)[1])
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

// Apply a function to all elements in a slice of strings.
func Map(vs []string, f func(string) string) []string {
	vsmap := make([]string, len(vs))

	for i, v := range vs {
		vsmap[i] = f(v)
	}

	return vsmap
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
	if err := validator.SetValidationFunc("safe", validateAttr); err != nil {
		panic(errors.Wrap(err, "validator initialization error"))
	}
}

func main() {
	listFlag := flag.Bool("list", false, "list hosts")
	exportFlag := flag.Bool("export", false, "export hosts and the groups they belong to")
	formatFlag := flag.String("format", "yaml", "export format")
	hostFlag := flag.Bool("host", false, "a stub for Ansible")
	flag.Parse()

	if *listFlag || *exportFlag {
		// Initialize and load DNS server configuration.
		config := &DNSServerConfig{}
		config.load()

		// Acquire TXT records.
		records := getTXTRecords(config)
		if len(records) == 0 {
			log.Fatal("empty TXT records list")
		}

		// Initialize the inventory tree.
		tree := &TreeNode{Name: "all", Parent: &TreeNode{}, Children: make([]*TreeNode, 0), Hosts: make(map[string]bool)}

		// Load DNS records into the inventory tree.
		tree.importHosts(parseTXTRecords(records, config.NoTx, config.NoTxSeparator))

		if !*exportFlag {
			// Export the inventory tree into a map.
			inventory := make(map[string]*InventoryGroup)
			tree.exportInventory(inventory)

			// Marshal the map into a JSON representation of an Ansible inventory.
			jsonInventory, err := json.Marshal(inventory)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(string(jsonInventory))
		} else {
			// Export the inventory tree into a map.
			hosts := make(map[string][]string)
			tree.exportHosts(hosts)

			var export []byte
			var err error
			switch *formatFlag {
			case "json":
				export, err = json.Marshal(hosts)
			case "yaml-csv":
				export, err = hostsToNDYAML(hosts, "csv")
			case "yaml-list":
				export, err = hostsToNDYAML(hosts, "list")
			default:
				export, err = hostsToNDYAML(hosts, "csv")
			}

			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(string(export))
		}
	} else if *hostFlag {
		fmt.Println("{}")
	}
}
