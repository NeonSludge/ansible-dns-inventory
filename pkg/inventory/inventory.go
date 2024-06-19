package inventory

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/creasty/defaults"
	"github.com/go-playground/validator/v10"
	"github.com/go-playground/validator/v10/non-standard/validators"
	"github.com/pkg/errors"

	"github.com/NeonSludge/ansible-dns-inventory/internal/logger"
)

const (
	adiSafeListRegexString              = "^[A-Za-z0-9\\,]*$"
	adiSafeListWithSeparatorRegexString = "^[A-Za-z0-9\\,\\-\\_]*$"
)

var (
	adiHostAttributeNames map[string]string

	adiSafeListRegex              = regexp.MustCompile(adiSafeListRegexString)
	adiSafeListWithSeparatorRegex = regexp.MustCompile(adiSafeListWithSeparatorRegexString)
)

// isSafeList validates if the field's value is a valid attribute list.
func isSafeList(fl validator.FieldLevel) bool {
	return adiSafeListRegex.MatchString(fl.Field().String())
}

// isSafeList validates if the field's value is a valid attribute list with separators that are allowed in Ansible group names.
func isSafeListWithSeparator(fl validator.FieldLevel) bool {
	return adiSafeListWithSeparatorRegex.MatchString(fl.Field().String())
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

	if err := i.Validator.Struct(attrs); err != nil {
		return nil, errors.Wrap(err, "attribute validation error")
	}

	return attrs, nil
}

// RenderAttributes constructs a string representation of the HostAttributes struct.
func (i *Inventory) RenderAttributes(attributes *HostAttributes) (string, error) {
	cfg := i.Config

	attrString := strings.Builder{}

	if err := i.Validator.Struct(attributes); err != nil {
		return "", errors.Wrap(err, "attribute validation error")
	}

	attrs := [][]string{{cfg.Txt.Keys.Os, attributes.OS}, {cfg.Txt.Keys.Env, attributes.Env}, {cfg.Txt.Keys.Role, attributes.Role}, {cfg.Txt.Keys.Srv, attributes.Srv}, {cfg.Txt.Keys.Vars, attributes.Vars}}

	for i, attr := range attrs {
		attrString.WriteString(attr[0])
		attrString.WriteString(cfg.Txt.Kv.Equalsign)
		attrString.WriteString(attr[1])

		if i != len(attrs)-1 {
			attrString.WriteString(cfg.Txt.Kv.Separator)
		}
	}

	return attrString.String(), nil
}

// PublishHosts publishes host records via the datasource.
func (i *Inventory) PublishHosts(hosts map[string][]*HostAttributes) error {
	log := i.Logger

	records := []*DatasourceRecord{}

	for hostname, attrsList := range hosts {
		for _, attrs := range attrsList {
			if attrString, err := i.RenderAttributes(attrs); err == nil {
				records = append(records, &DatasourceRecord{
					Hostname:   hostname,
					Attributes: attrString,
				})
			} else {
				log.Warnf("[%s] skipping host record: %v", hostname, err)
				continue
			}
		}
	}

	return i.Datasource.PublishRecords(records)
}

// New creates an instance of the DNS inventory with user-supplied configuration.
func New(cfg *Config) (*Inventory, error) {
	// Setup package global state
	adiHostAttributeNames = make(map[string]string)
	adiHostAttributeNames["OS"] = cfg.Txt.Keys.Os
	adiHostAttributeNames["ENV"] = cfg.Txt.Keys.Env
	adiHostAttributeNames["ROLE"] = cfg.Txt.Keys.Role
	adiHostAttributeNames["SRV"] = cfg.Txt.Keys.Srv
	adiHostAttributeNames["VARS"] = cfg.Txt.Keys.Vars

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

	// Initialize struct validator.
	validator := validator.New()
	validator.RegisterValidation("notblank", validators.NotBlank)
	validator.RegisterValidation("safelist", isSafeList)
	validator.RegisterValidation("safelistsep", isSafeListWithSeparator)

	i := &Inventory{
		Config:    cfg,
		Logger:    cfg.Logger,
		Validator: validator,

		Datasource: ds,
		Tree:       NewTree(),
	}

	return i, nil
}

// NewDefault creates an instance of the DNS inventory with the default configuration.
func NewDefault() (*Inventory, error) {
	cfg := &Config{}

	if err := defaults.Set(cfg); err != nil {
		return nil, errors.Wrap(err, "defaults initialization failure")
	}

	return New(cfg)
}
