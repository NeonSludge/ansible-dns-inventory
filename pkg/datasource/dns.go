package datasource

import (
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/pkg/errors"

	"github.com/NeonSludge/ansible-dns-inventory/pkg/types"
)

const (
	// DNS TXT record type.
	dnsRrTxtType uint16 = 16
	// Number of the field that contains the TXT record value.
	dnsRrTxtField int = 1
)

type (
	// A DNS datasource implementation.
	DNS struct {
		// DNS client.
		Client *dns.Client
		// DNS zone transfer parameters.
		Transfer *dns.Transfer
		// Inventory configuration.
		Config *types.InventoryConfig
		// Inventory logger.
		Logger types.InventoryLogger
	}
)

// Process a single DNS resource record.
func (d *DNS) processRecord(rr dns.RR) *types.InventoryDatasourceRecord {
	cfg := d.Config
	var name, attrs string

	if cfg.DNS.Notransfer.Enabled {
		name = strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), cfg.DNS.Notransfer.Separator)[0], ".")
		attrs = strings.Split(dns.Field(rr, dnsRrTxtField), cfg.DNS.Notransfer.Separator)[1]
	} else {
		name = strings.TrimSuffix(rr.Header().Name, ".")
		attrs = dns.Field(rr, dnsRrTxtField)
	}

	return &types.InventoryDatasourceRecord{
		Hostname:   name,
		Attributes: attrs,
	}
}

// Process several DNS resource records.
func (d *DNS) processRecords(rrs []dns.RR) []*types.InventoryDatasourceRecord {
	records := make([]*types.InventoryDatasourceRecord, 0)

	for _, rr := range rrs {
		records = append(records, d.processRecord(rr))
	}

	return records
}

// Produce a fully qualified host name for use in DNS requests.
func (d *DNS) makeFQDN(host string, zone string) string {
	name := strings.TrimPrefix(host, ".")
	domain := strings.TrimPrefix(zone, ".")

	if len(domain) == 0 {
		return dns.Fqdn(name)
	}

	return strings.TrimPrefix(dns.Fqdn(name+"."+domain), ".")
}

// getZone acquires TXT records for all hosts in a specific zone.
func (d *DNS) getZone(zone string) ([]dns.RR, error) {
	cfg := d.Config
	records := make([]dns.RR, 0)

	msg := new(dns.Msg)
	msg.SetAxfr(dns.Fqdn(zone))

	if cfg.DNS.Tsig.Enabled {
		d.Transfer.TsigSecret = map[string]string{cfg.DNS.Tsig.Key: cfg.DNS.Tsig.Secret}
		msg.SetTsig(cfg.DNS.Tsig.Key, cfg.DNS.Tsig.Algo, 300, time.Now().Unix())
	}

	// Perform the transfer.
	c, err := d.Transfer.In(msg, cfg.DNS.Server)
	if err != nil {
		return nil, errors.Wrap(err, "zone transfer failed")
	}

	// Process transferred records. Ignore anything that is not a TXT recordd. Ignore the special inventory record as well.
	for e := range c {
		for _, rr := range e.RR {
			if rr.Header().Rrtype == dnsRrTxtType && rr.Header().Name != d.makeFQDN(cfg.DNS.Notransfer.Host, zone) {
				records = append(records, rr)
			}
		}
	}

	return records, nil
}

// getHost acquires all TXT records for a specific host.
func (d *DNS) getHost(host string) ([]dns.RR, error) {
	cfg := d.Config
	msg := new(dns.Msg)
	msg.SetQuestion(host, dns.TypeTXT)

	rx, _, err := d.Client.Exchange(msg, cfg.DNS.Server)
	if err != nil {
		return nil, errors.Wrap(err, "dns request failed")
	}

	return rx.Answer, nil
}

// GetAllRecords acquires all available host records.
func (d *DNS) GetAllRecords() ([]*types.InventoryDatasourceRecord, error) {
	cfg := d.Config
	log := d.Logger
	records := make([]*types.InventoryDatasourceRecord, 0)

	for _, zone := range cfg.DNS.Zones {
		var rrs []dns.RR
		var err error

		if cfg.DNS.Notransfer.Enabled {
			rrs, err = d.getHost(d.makeFQDN(cfg.DNS.Notransfer.Host, zone))
		} else {
			rrs, err = d.getZone(d.makeFQDN("", zone))
		}
		if err != nil {
			log.Warnf("[%s] skipping zone: %v", zone, err)
			continue
		}

		records = append(records, d.processRecords(rrs)...)
	}

	return records, nil
}

// GetHostRecords acquires all available records for a specific host.
func (d *DNS) GetHostRecords(host string) ([]*types.InventoryDatasourceRecord, error) {
	cfg := d.Config
	records := make([]*types.InventoryDatasourceRecord, 0)
	var err error

	if cfg.DNS.Notransfer.Enabled {
		// No-transfer mode is enabled.
		var zone string
		var rrs []dns.RR

		// Determine which zone we are working with.
		for _, z := range cfg.DNS.Zones {
			if strings.HasSuffix(dns.Fqdn(host), dns.Fqdn(z)) {
				zone = z
				break
			}
		}

		if len(zone) == 0 {
			return nil, errors.New("failed to determine zone from hostname")
		}

		// Get no-transfer host records.
		rrs, err = d.getHost(d.makeFQDN(cfg.DNS.Notransfer.Host, zone))
		if err != nil {
			return nil, err
		}

		// Filter out the irrelevant records.
		for _, rr := range rrs {
			name := strings.TrimSuffix(strings.Split(dns.Field(rr, dnsRrTxtField), cfg.DNS.Notransfer.Separator)[0], ".")
			if host == name {
				records = append(records, d.processRecord(rr))
			}
		}
	} else {
		// No-transfer mode is disabled, no special logic is needed.
		rrs, err := d.getHost(d.makeFQDN(host, ""))
		if err != nil {
			return nil, err
		}

		records = append(records, d.processRecords(rrs)...)
	}

	return records, nil
}

// Close datasource and perform housekeeping.
func (d *DNS) Close() {}
