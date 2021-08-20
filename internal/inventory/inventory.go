package inventory

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"gopkg.in/validator.v2"
)

const (
	// DNS TXT record type.
	dnsRrTxtType uint16 = 16
	// Number of the field that contains the TXT record value.
	dnsRrTxtField int = 1
)

// safeAttr validates host attributes.
func safeAttr(v interface{}, param string) error {
	value := reflect.ValueOf(v)
	if value.Kind() != reflect.String {
		return errors.New("safeAttr() can only validate strings")
	}

	re := "^[A-Za-z0-9"

	// Deprecated: using '-' in group names.
	if txtKeysSeparator == "-" {
		re += "\\_"
	}

	switch param {
	case "srv":
		re += "\\,\\" + txtKeysSeparator + "]*$"
	case "list":
		re += "\\," + "]*$"
	case "vars":
		re = "^[[:print:]]*$"
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

// ImportHosts loads a map of hosts and their attributes into the inventory tree.
func (i *Inventory) ImportHosts(hosts map[string][]*HostAttributes) {
	i.Tree.importHosts(hosts, i.Config.GetString("txt.keys.separator"))
}

// SortChildren sorts inventory tree nodes (groups) by name recursively.
func (i *Inventory) SortChildren() {
	i.Tree.sortChildren()
}

// ExportHosts exports the inventory tree into a map of hosts and groups they belong to.
func (i *Inventory) ExportHosts(hosts map[string][]string) {
	i.Tree.exportHosts(hosts)
}

// ExportGroups exports the inventory tree into a map of groups and hosts they contain.
func (i *Inventory) ExportGroups(groups map[string][]string) {
	i.Tree.exportGroups(groups)
}

// ExportInventory exports the inventory tree into a map ready to be marshalled into a JSON representation of a dynamic Ansible inventory.
func (i *Inventory) ExportInventory(inventory map[string]*AnsibleGroup) {
	i.Tree.exportInventory(inventory)
}

// GetAllRecords acquires DNS records from a remote DNS server.
func (i *Inventory) GetAllRecords() []dns.RR {
	cfg := i.Config
	records := make([]dns.RR, 0)

	for _, zone := range cfg.GetStringSlice("dns.zones") {
		var rrs []dns.RR
		var err error

		if cfg.GetBool("dns.notransfer.enabled") {
			rrs, err = i.GetRecords(zone)
		} else {
			rrs, err = i.TransferZone(zone)
		}

		if err != nil {
			log.Printf("[%s] skipping zone: %v", zone, err)
			continue
		}

		records = append(records, rrs...)
	}

	return records
}

// TransferZone performs a DNS zone transfer (AXFR).
func (i *Inventory) TransferZone(zone string) ([]dns.RR, error) {
	cfg := i.Config
	records := make([]dns.RR, 0)

	t, err := time.ParseDuration(cfg.GetString("dns.timeout"))
	if err != nil {
		return records, errors.Wrap(err, "zone transfer failed")
	}
	tx := &dns.Transfer{
		DialTimeout:  t,
		ReadTimeout:  t,
		WriteTimeout: t,
	}

	msg := new(dns.Msg)
	msg.SetAxfr(dns.Fqdn(zone))

	if cfg.GetBool("dns.tsig.enabled") {
		tx.TsigSecret = map[string]string{cfg.GetString("dns.tsig.key"): cfg.GetString("dns.tsig.secret")}
		msg.SetTsig(cfg.GetString("dns.tsig.key"), cfg.GetString("dns.tsig.algo"), 300, time.Now().Unix())
	}

	// Perform the transfer.
	c, err := tx.In(msg, cfg.GetString("dns.server"))
	if err != nil {
		return records, errors.Wrap(err, "zone transfer failed")
	}

	// Process transferred records. Ignore anything that is not a TXT recordd. Ignore the special inventory record as well.
	for e := range c {
		for _, rr := range e.RR {
			if rr.Header().Rrtype == dnsRrTxtType && rr.Header().Name != dns.Fqdn(cfg.GetString("dns.notransfer.host")+"."+zone) {
				records = append(records, rr)
			}
		}
	}
	if len(records) == 0 {
		return records, errors.Wrap(fmt.Errorf("no TXT records found: %s", zone), "zone transfer failed")
	}

	return records, nil
}

// GetRecords performs a DNS query for TXT records of a specific host.
func (i *Inventory) GetRecords(host string) ([]dns.RR, error) {
	cfg := i.Config
	records := make([]dns.RR, 0)
	var name string

	if len(host) > 0 {
		name = fmt.Sprintf("%s.%s", cfg.GetString("dns.notransfer.host"), dns.Fqdn(host))
	} else {
		name = dns.Fqdn(cfg.GetString("dns.notransfer.host"))
	}

	t, err := time.ParseDuration(cfg.GetString("dns.timeout"))
	if err != nil {
		return records, errors.Wrap(err, "record loading failed")
	}
	client := &dns.Client{
		Timeout: t,
	}

	msg := new(dns.Msg)
	msg.SetQuestion(name, dns.TypeTXT)

	rx, _, err := client.Exchange(msg, cfg.GetString("dns.server"))
	if err != nil {
		return records, errors.Wrap(err, "record loading failed")
	} else if len(rx.Answer) == 0 {
		return records, errors.Wrap(fmt.Errorf("not found: %s", name), "record loading failed")
	}
	records = rx.Answer

	return records, nil
}

// GetHostRecords acquires DNS TXT records of a specific host by performing a DNS query for that host or by parsing the no-transfer host records.
func (i *Inventory) GetHostRecords(host string) ([]dns.RR, error) {
	cfg := i.Config
	records := make([]dns.RR, 0)
	var err error

	if cfg.GetBool("dns.notransfer.enabled") {
		// No-transfer mode is enabled.
		var zone string
		var rrs []dns.RR

		// Determine which zone we are working with.
		for _, z := range cfg.GetStringSlice("dns.zones") {
			if strings.HasSuffix(dns.Fqdn(host), dns.Fqdn(z)) {
				zone = z
				break
			}
		}

		// Get no-transfer host records.
		rrs, err = i.GetRecords(zone)
		if err != nil {
			return records, err
		}

		// Filter out the irrelevant records.
		for _, rr := range rrs {
			name := strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), cfg.GetString("dns.notransfer.separator"))[0], ".")
			if host == name {
				records = append(records, rr)
			}
		}
	} else {
		// No-transfer mode is disabled, no special logic is needed.
		records, err = i.GetRecords(host)
		if err != nil {
			return records, err
		}
	}

	return records, nil
}

// ParseRecords parses TXT records and maps hosts to lists of their attributes.
func (i *Inventory) ParseRecords(records []dns.RR) map[string][]*HostAttributes {
	cfg := i.Config
	hosts := make(map[string][]*HostAttributes)

	for _, rr := range records {
		var name string
		var attrs *HostAttributes
		var err error

		if cfg.GetBool("dns.notransfer.enabled") {
			name = strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), cfg.GetString("dns.notransfer.separator"))[0], ".")
			attrs, err = i.ParseAttributes(strings.Split(dns.Field(rr, dnsRrTxtField), cfg.GetString("dns.notransfer.separator"))[1])
		} else {
			name = strings.TrimSuffix(rr.Header().Name, ".")
			attrs, err = i.ParseAttributes(dns.Field(rr, dnsRrTxtField))
		}

		if err != nil {
			log.Printf("[%s] skipping host: %v", name, err)
			continue
		}

		for _, role := range strings.Split(attrs.Role, ",") {
			for _, srv := range strings.Split(attrs.Srv, ",") {
				hosts[name] = append(hosts[name], &HostAttributes{
					OS:   attrs.OS,
					Env:  attrs.Env,
					Role: role,
					Srv:  srv,
					Vars: attrs.Vars,
				})
			}
		}
	}

	return hosts
}

// ParseAttributes parses host attributes.
func (i *Inventory) ParseAttributes(raw string) (*HostAttributes, error) {
	cfg := i.Config
	attrs := &HostAttributes{}
	items := strings.Split(raw, cfg.GetString("txt.kv.separator"))

	for _, item := range items {
		kv := strings.Split(item, cfg.GetString("txt.kv.equalsign"))
		switch kv[0] {
		case cfg.GetString("txt.keys.os"):
			attrs.OS = kv[1]
		case cfg.GetString("txt.keys.env"):
			attrs.Env = kv[1]
		case cfg.GetString("txt.keys.role"):
			attrs.Role = kv[1]
		case cfg.GetString("txt.keys.srv"):
			attrs.Srv = kv[1]
		case cfg.GetString("txt.keys.vars"):
			attrs.Vars = strings.Join(kv[1:], cfg.GetString("txt.kv.equalsign"))
		}
	}

	if err := validator.Validate(attrs); err != nil {
		return attrs, errors.Wrap(err, "attribute validation error")
	}

	return attrs, nil
}

// ParseVariables returns the JSON encoding of all host variables found in v.
func (i *Inventory) ParseVariables(a []*HostAttributes) ([]byte, error) {
	cfg := i.Config
	vars := make(map[string]string)
	var bytes []byte
	var err error

	for _, attrs := range a {
		if len(attrs.Vars) > 0 {
			pairs := strings.Split(attrs.Vars, cfg.GetString("txt.vars.separator"))

			for _, pair := range pairs {
				kv := strings.Split(pair, cfg.GetString("txt.vars.equalsign"))
				vars[kv[0]] = kv[1]
			}
		}
	}

	bytes, err = json.Marshal(vars)
	if err != nil {
		return bytes, err
	}

	return bytes, nil
}

// New creates an empty instance of a DNS inventory.
func New() (*Inventory, error) {
	// Process configuration
	cfg, err := newConfig()
	if err != nil {
		return nil, errors.Wrap(err, "configuration initialization failure")
	}

	// Setup struct validators.
	if err := validator.SetValidationFunc("safe", safeAttr); err != nil {
		return nil, errors.Wrap(err, "validator initialization error")
	}

	i := &Inventory{
		Config: cfg,
		Tree:   newTree(),
	}

	return i, nil
}
