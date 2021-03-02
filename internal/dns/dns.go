package dns

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/NeonSludge/ansible-dns-inventory/internal/config"
	"github.com/NeonSludge/ansible-dns-inventory/internal/types"
	"github.com/NeonSludge/ansible-dns-inventory/internal/util"
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

func init() {
	// Setup struct validators.
	if err := validator.SetValidationFunc("safe", util.SafeAttr); err != nil {
		panic(errors.Wrap(err, "validator initialization error"))
	}
}

// GetAllRecords acquires DNS records from a remote DNS server.
func GetAllRecords(cfg *config.Main) []dns.RR {
	records := make([]dns.RR, 0)

	for _, zone := range cfg.Zones {
		var rrs []dns.RR
		var err error

		if cfg.NoTx {
			rrs, err = GetRecords(cfg.Address, zone, cfg.NoTxHost, cfg.Timeout)
		} else {
			rrs, err = TransferZone(cfg.Address, zone, cfg.NoTxHost, cfg.Timeout)
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
func TransferZone(server string, domain string, notxName string, timeout string) ([]dns.RR, error) {
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

// GetRecords performs a DNS query for TXT records of a specific host.
func GetRecords(server string, domain string, host string, timeout string) ([]dns.RR, error) {
	records := make([]dns.RR, 0)
	var name string

	if len(domain) > 0 {
		name = fmt.Sprintf("%s.%s", host, dns.Fqdn(domain))
	} else {
		name = dns.Fqdn(host)
	}

	t, err := time.ParseDuration(timeout)
	if err != nil {
		return records, errors.Wrap(err, "record loading failed")
	}
	client := &dns.Client{
		Timeout: t,
	}

	msg := new(dns.Msg)
	msg.SetQuestion(name, dns.TypeTXT)

	rx, _, err := client.Exchange(msg, server)
	if err != nil {
		return records, errors.Wrap(err, "record loading failed")
	} else if len(rx.Answer) == 0 {
		return records, errors.Wrap(fmt.Errorf("not found: %s", name), "record loading failed")
	}
	records = rx.Answer

	return records, nil
}

// GetHostRecords acquires DNS TXT records of a specific host by performing a DNS query for that host or by parsing the no-transfer host records.
func GetHostRecords(cfg *config.Main, host string) ([]dns.RR, error) {
	records := make([]dns.RR, 0)
	var err error

	if cfg.NoTx {
		// No-transfer mode is enabled.
		var zone string
		var rrs []dns.RR

		// Determine which zone we are working with.
		for _, z := range cfg.Zones {
			if strings.HasSuffix(dns.Fqdn(host), dns.Fqdn(z)) {
				zone = z
				break
			}
		}

		// Get no-transfer host records.
		rrs, err = GetRecords(cfg.Address, zone, cfg.NoTxHost, cfg.Timeout)
		if err != nil {
			return records, err
		}

		// Filter out the irrelevant records.
		for _, rr := range rrs {
			name := strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), cfg.NoTxSeparator)[0], ".")
			if host == name {
				records = append(records, rr)
			}
		}
	} else {
		// No-transfer mode is disabled, no special logic is needed.
		records, err = GetRecords(cfg.Address, "", host, cfg.Timeout)
		if err != nil {
			return records, err
		}
	}

	return records, nil
}

// ParseRecords parses TXT records and maps hosts to lists of their attributes.
func ParseRecords(records []dns.RR, cfg *config.Main) map[string][]*types.Attributes {
	hosts := make(map[string][]*types.Attributes)

	for _, rr := range records {
		var name string
		var attrs *types.Attributes
		var err error

		if cfg.NoTx {
			name = strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), cfg.NoTxSeparator)[0], ".")
			attrs, err = ParseAttributes(strings.Split(dns.Field(rr, dnsRrTxtField), cfg.NoTxSeparator)[1], cfg)
		} else {
			name = strings.TrimSuffix(rr.Header().Name, ".")
			attrs, err = ParseAttributes(dns.Field(rr, dnsRrTxtField), cfg)
		}

		if err != nil {
			log.Printf("[%s] skipping host: %v", name, err)
			continue
		}

		for _, role := range strings.Split(attrs.Role, ",") {
			for _, srv := range strings.Split(attrs.Srv, ",") {
				hosts[name] = append(hosts[name], &types.Attributes{
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
func ParseAttributes(raw string, cfg *config.Main) (*types.Attributes, error) {
	attrs := &types.Attributes{}
	items := strings.Split(raw, cfg.KvSeparator)

	for _, item := range items {
		kv := strings.Split(item, cfg.KvEquals)
		switch kv[0] {
		case cfg.KeyOs:
			attrs.OS = kv[1]
		case cfg.KeyEnv:
			attrs.Env = kv[1]
		case cfg.KeyRole:
			attrs.Role = kv[1]
		case cfg.KeySrv:
			attrs.Srv = kv[1]
		case cfg.KeyVars:
			attrs.Vars = strings.Join(kv[1:], cfg.KvEquals)
		}
	}

	if err := validator.Validate(attrs); err != nil {
		return attrs, errors.Wrap(err, "attribute validation error")
	}

	return attrs, nil
}

// ParseVariables returns the JSON encoding of all host variables found in v.
func ParseVariables(a []*types.Attributes, cfg *config.Main) ([]byte, error) {
	vars := make(map[string]string)
	var bytes []byte
	var err error

	for _, attrs := range a {
		if len(attrs.Vars) > 0 {
			pairs := strings.Split(attrs.Vars, cfg.VarsSeparator)

			for _, pair := range pairs {
				kv := strings.Split(pair, cfg.VarsEquals)
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
