package inventory

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/validator.v2"

	"github.com/NeonSludge/ansible-dns-inventory/internal/logger"
)

var adiHostAttributeNames map[string]string
var adiTxtKeysSeparator string

// safeAttr validates host attributes.
func safeAttr(v interface{}, param string) error {
	value := reflect.ValueOf(v)
	if value.Kind() != reflect.String {
		return errors.New("safeAttr() can only validate strings")
	}

	re := "^[A-Za-z0-9"

	// Deprecated: using '-' in group names.
	if adiTxtKeysSeparator == "-" {
		re += "\\_"
	}

	switch param {
	case "srv":
		re += "\\,\\" + adiTxtKeysSeparator + "]*$"
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

// MarshalJSON implements a custom JSON Marshaller for host attributes.
func (a *HostAttributes) MarshalJSON() ([]byte, error) {
	attrs := make(map[string]string)

	attrs[adiHostAttributeNames["OS"]] = a.OS
	attrs[adiHostAttributeNames["ENV"]] = a.Env
	attrs[adiHostAttributeNames["ROLE"]] = a.Role
	attrs[adiHostAttributeNames["SRV"]] = a.Srv
	attrs[adiHostAttributeNames["VARS"]] = a.Vars

	return json.Marshal(attrs)
}

// MarshalYAML implements a custom YAML Marshaller for host attributes.
func (a *HostAttributes) MarshalYAML() (interface{}, error) {
	attrs := make(map[string]string)

	attrs[adiHostAttributeNames["OS"]] = a.OS
	attrs[adiHostAttributeNames["ENV"]] = a.Env
	attrs[adiHostAttributeNames["ROLE"]] = a.Role
	attrs[adiHostAttributeNames["SRV"]] = a.Srv
	attrs[adiHostAttributeNames["VARS"]] = a.Vars

	return attrs, nil
}

// ImportHosts loads a map of hosts and their attributes into the inventory tree.
func (i *Inventory) ImportHosts(hosts map[string][]*HostAttributes) {
	i.Tree.ImportHosts(hosts, i.Config.Txt.Keys.Separator)
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
	cfg := i.Config
	log := i.Logger
	variables := make(map[string]string)

	records, err := i.Datasource.GetHostRecords(host)
	if err != nil {
		return nil, errors.Wrap(err, "host record loading failure")
	}

	for _, r := range records {
		attrs, err := i.ParseAttributes(r.Attributes)
		if err != nil {
			log.Warnf("[%s] skipping host record: %v", r.Hostname, err)
			continue
		}

		if len(attrs.Vars) > 0 {
			pairs := strings.Split(attrs.Vars, cfg.Txt.Vars.Separator)
			for _, p := range pairs {
				kv := strings.Split(p, cfg.Txt.Vars.Equalsign)
				if len(kv) == 2 {
					variables[kv[0]] = kv[1]
				}
			}
		}
	}

	return variables, nil
}

// GetHosts acquires a map of all hosts and their attributes.
func (i *Inventory) GetHosts() (map[string][]*HostAttributes, error) {
	log := i.Logger
	hosts := make(map[string][]*HostAttributes)

	records, err := i.Datasource.GetAllRecords()
	if err != nil {
		return nil, errors.Wrap(err, "record loading failure")
	}

	for _, r := range records {
		attrs, err := i.ParseAttributes(r.Attributes)
		if err != nil {
			log.Warnf("[%s] skipping host record: %v", r.Hostname, err)
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
	items := strings.Split(raw, cfg.Txt.Kv.Separator)

	for _, item := range items {
		kv := strings.SplitN(item, cfg.Txt.Kv.Equalsign, 2)
		switch kv[0] {
		case cfg.Txt.Keys.Os:
			attrs.OS = kv[1]
		case cfg.Txt.Keys.Env:
			attrs.Env = kv[1]
		case cfg.Txt.Keys.Role:
			attrs.Role = kv[1]
		case cfg.Txt.Keys.Srv:
			attrs.Srv = kv[1]
		case cfg.Txt.Keys.Vars:
			attrs.Vars = kv[1]
		}
	}

	if err := validator.Validate(attrs); err != nil {
		return nil, errors.Wrap(err, "attribute validation error")
	}

	return attrs, nil
}

// New creates an instance of the DNS inventory.
func New(cfg *Config) (*Inventory, error) {
	// Setup package global state
	adiHostAttributeNames = make(map[string]string)
	adiHostAttributeNames["OS"] = cfg.Txt.Keys.Os
	adiHostAttributeNames["ENV"] = cfg.Txt.Keys.Env
	adiHostAttributeNames["ROLE"] = cfg.Txt.Keys.Role
	adiHostAttributeNames["SRV"] = cfg.Txt.Keys.Srv
	adiHostAttributeNames["VARS"] = cfg.Txt.Keys.Vars
	adiTxtKeysSeparator = cfg.Txt.Keys.Separator

	// Initialize logger.
	if cfg.Logger == nil {
		l, err := logger.New("info")
		if err != nil {
			return nil, errors.Wrap(err, "logger initialization failure")
		}
		cfg.Logger = l
	}

	// Initialize datasource.
	ds, err := NewDatasource(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "datasource initialization failure")
	}

	// Initialize struct validators.
	if err := validator.SetValidationFunc("safe", safeAttr); err != nil {
		return nil, errors.Wrap(err, "validator initialization failure")
	}

	i := &Inventory{
		Config:     cfg,
		Logger:     cfg.Logger,
		Datasource: ds,
		Tree:       NewTree(),
	}

	return i, nil
}
