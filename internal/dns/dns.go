package dns

import (
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

// Acquire DNS records from a remote DNS server.
func GetTXTRecords(c *config.DNS) []dns.RR {
	records := make([]dns.RR, 0)

	for _, zone := range c.Zones {
		var rrs []dns.RR
		var err error

		if c.NoTx {
			rrs, err = GetInventoryRecord(c.Address, zone, c.NoTxHost, c.Timeout)
		} else {
			rrs, err = TransferZone(c.Address, zone, c.NoTxHost, c.Timeout)
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

// Acquire TXT records of a special host (no-transfer mode).
func GetInventoryRecord(server string, domain string, host string, timeout string) ([]dns.RR, error) {
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
func ParseTXTRecords(records []dns.RR, dc *config.DNS, pc *config.Parse) map[string]*types.TXTAttrs {
	hosts := make(map[string]*types.TXTAttrs)

	for _, rr := range records {
		var name string
		var attrs *types.TXTAttrs
		var err error

		if dc.NoTx {
			name = strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), dc.NoTxSeparator)[0], ".")
			attrs, err = ParseAttributes(strings.Split(dns.Field(rr, dnsRrTxtField), dc.NoTxSeparator)[1], pc)
		} else {
			name = strings.TrimSuffix(rr.Header().Name, ".")
			attrs, err = ParseAttributes(dns.Field(rr, dnsRrTxtField), pc)
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
func ParseAttributes(raw string, pc *config.Parse) (*types.TXTAttrs, error) {
	attrs := &types.TXTAttrs{}
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

	// Setup struct validators.
	if err := validator.SetValidationFunc("safe", util.SafeAttr); err != nil {
		return attrs, errors.Wrap(err, "validator initialization error")
	}

	if err := validator.Validate(attrs); err != nil {
		return attrs, errors.Wrap(err, "attribute validation error")
	}

	return attrs, nil
}
