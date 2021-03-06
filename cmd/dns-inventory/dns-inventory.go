package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/NeonSludge/ansible-dns-inventory/internal/build"
	"github.com/NeonSludge/ansible-dns-inventory/internal/config"
	"github.com/NeonSludge/ansible-dns-inventory/internal/dns"
	"github.com/NeonSludge/ansible-dns-inventory/internal/tree"
	"github.com/NeonSludge/ansible-dns-inventory/internal/types"
	"github.com/NeonSludge/ansible-dns-inventory/internal/util"
)

func main() {
	// Setup logging.
	log.SetOutput(os.Stderr)

	// Parse flags.
	listFlag := flag.Bool("list", false, "produce a JSON inventory for Ansible")
	hostsFlag := flag.Bool("hosts", false, "export hosts")
	attrsFlag := flag.Bool("attrs", false, "export host attributes")
	groupsFlag := flag.Bool("groups", false, "export groups")
	treeFlag := flag.Bool("tree", false, "export raw inventory tree")
	formatFlag := flag.String("format", "yaml", "select export format, if available")
	hostFlag := flag.String("host", "", "a stub for Ansible")
	versionFlag := flag.Bool("version", false, "display ansible-dns-inventory version and build info")
	flag.Parse()

	// Initialize and load configuration.
	cfg := config.New()

	if len(*hostFlag) == 0 {
		// Acquire TXT records.
		records := dns.GetAllRecords(cfg)
		if len(records) == 0 {
			log.Fatal("empty TXT records list")
		}

		// Parse TXT records.
		hosts := dns.ParseRecords(records, cfg)

		// Initialize the inventory tree.
		inventory := tree.New()

		// Load DNS records into the inventory tree.
		inventory.ImportHosts(hosts, cfg)
		inventory.SortChildren()

		// Export the inventory tree in various formats.
		var bytes []byte
		var err error
		switch {
		case *versionFlag:
			fmt.Println("version:", build.Version)
			fmt.Println("build time:", build.Time)
		case *listFlag:
			export := make(map[string]*types.InventoryGroup)

			// Export the inventory tree into a map.
			inventory.ExportInventory(export)

			// Marshal the map into a JSON representation of an Ansible inventory.
			bytes, err = util.Marshal(export, "json", cfg)
		case *attrsFlag:
			bytes, err = util.Marshal(hosts, *formatFlag, cfg)
		case *treeFlag:
			bytes, err = util.Marshal(inventory, *formatFlag, cfg)
		default:
			export := make(map[string][]string)

			// Export the inventory tree into a map.
			switch {
			case *hostsFlag:
				inventory.ExportHosts(export)
			case *groupsFlag:
				inventory.ExportGroups(export)
			}

			bytes, err = util.Marshal(export, *formatFlag, cfg)
		}

		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(bytes))
	} else if len(*hostFlag) > 0 && cfg.VarsEnabled {
		// Acquire host TXT records.
		records, err := dns.GetHostRecords(cfg, *hostFlag)
		if err != nil {
			log.Fatal(err)
		}

		// Parse host TXT records.
		attrs := dns.ParseRecords(records, cfg)[*hostFlag]

		// Parse host variables.
		bytes, err := dns.ParseVariables(attrs, cfg)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(bytes))
	} else {
		fmt.Println("{}")
	}
}
