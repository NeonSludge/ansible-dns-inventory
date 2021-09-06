package inventory

import (
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/validator.v2"

	"github.com/NeonSludge/ansible-dns-inventory/pkg/config"
	"github.com/NeonSludge/ansible-dns-inventory/pkg/datasource"
)

// safeAttr validates host attributes.
func safeAttr(v interface{}, param string) error {
	value := reflect.ValueOf(v)
	if value.Kind() != reflect.String {
		return errors.New("safeAttr() can only validate strings")
	}

	re := "^[A-Za-z0-9"

	// Deprecated: using '-' in group names.
	if config.ADITxtKeysSeparator == "-" {
		re += "\\_"
	}

	switch param {
	case "srv":
		re += "\\,\\" + config.ADITxtKeysSeparator + "]*$"
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
	i.Tree.ImportHosts(hosts, i.Config.GetString("txt.keys.separator"))
	i.Tree.SortChildren()
}

// ExportHosts exports the inventory tree into a map of hosts and groups they belong to.
func (i *Inventory) ExportHosts(hosts map[string][]string) {
	i.Tree.ExportHosts(hosts)
}

// ExportGroups exports the inventory tree into a map of groups and hosts they contain.
func (i *Inventory) ExportGroups(groups map[string][]string) {
	i.Tree.ExportGroups(groups)
}

// ExportInventory exports the inventory tree into a map ready to be marshalled into a JSON representation of a dynamic Ansible inventory.
func (i *Inventory) ExportInventory(inventory map[string]*AnsibleGroup) {
	i.Tree.ExportInventory(inventory)
}

// GetHostVariables acquires a map of host variables specified via the 'VARS' attribute.
func (i *Inventory) GetHostVariables(host string) (map[string]string, error) {
	variables := make(map[string]string)

	records, err := i.Datasource.GetHostRecords(host)
	if err != nil {
		return variables, errors.Wrap(err, "host record loading failure")
	}

	for _, r := range records {
		attrs, err := i.ParseAttributes(r.Attributes)
		if err != nil {
			log.Printf("[%s] skipping host record: %v", r.Hostname, err)
			continue
		}

		if len(attrs.Vars) > 0 {
			pairs := strings.Split(attrs.Vars, i.Config.GetString("txt.vars.separator"))

			for _, p := range pairs {
				kv := strings.Split(p, i.Config.GetString("txt.vars.equalsign"))
				variables[kv[0]] = kv[1]
			}
		}
	}

	return variables, nil
}

// GetHosts acquires a map of all hosts and their attributes.
func (i *Inventory) GetHosts() (map[string][]*HostAttributes, error) {
	hosts := make(map[string][]*HostAttributes)

	records, err := i.Datasource.GetAllRecords()
	if err != nil {
		return hosts, errors.Wrap(err, "record loading failure")
	}

	if len(records) == 0 {
		return hosts, errors.New("no TXT records found")
	}

	for _, r := range records {
		attrs, err := i.ParseAttributes(r.Attributes)
		if err != nil {
			log.Printf("[%s] skipping host record: %v", r.Hostname, err)
			continue
		}

		for _, role := range strings.Split(attrs.Role, ",") {
			for _, srv := range strings.Split(attrs.Srv, ",") {
				hosts[r.Hostname] = append(hosts[r.Hostname], &HostAttributes{
					OS:   attrs.OS,
					Env:  attrs.Env,
					Role: role,
					Srv:  srv,
					Vars: attrs.Vars,
				})
			}
		}
	}

	return hosts, nil
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

// New creates an instance of the DNS inventory.
func New() (*Inventory, error) {
	// Process configuration
	cfg, err := config.New()
	if err != nil {
		return nil, errors.Wrap(err, "configuration initialization failure")
	}

	ds, err := datasource.New(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "datasource initialization failure")
	}

	// Setup struct validators.
	if err := validator.SetValidationFunc("safe", safeAttr); err != nil {
		return nil, errors.Wrap(err, "validator initialization failure")
	}

	i := &Inventory{
		Config:     cfg,
		Datasource: ds,
		Tree:       InitTree(),
	}

	return i, nil
}
