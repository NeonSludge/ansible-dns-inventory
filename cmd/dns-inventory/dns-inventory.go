package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/NeonSludge/ansible-dns-inventory/internal/build"
	"github.com/NeonSludge/ansible-dns-inventory/internal/inventory"
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

	// Initialize a new inventory.
	dnsInventory, err := inventory.New()
	if err != nil {
		log.Fatal(err)
	}

	if len(*hostFlag) == 0 {
		// Acquire TXT records.
		records := dnsInventory.GetAllRecords()
		if len(records) == 0 {
			log.Fatal("empty TXT records list")
		}

		// Parse TXT records.
		hosts := dnsInventory.ParseRecords(records)

		// Load DNS records into the inventory tree.
		dnsInventory.ImportHosts(hosts)
		dnsInventory.SortChildren()

		// Export the inventory tree in various formats.
		var bytes []byte
		var err error
		switch {
		case *versionFlag:
			fmt.Println("version:", build.Version)
			fmt.Println("build time:", build.Time)
		case *listFlag:
			export := make(map[string]*inventory.AnsibleGroup)

			// Export the inventory tree into a map.
			dnsInventory.ExportInventory(export)

			// Marshal the map into a JSON representation of an Ansible inventory.
			bytes, err = util.Marshal(export, "json", dnsInventory.Config)
		case *attrsFlag:
			bytes, err = util.Marshal(hosts, *formatFlag, dnsInventory.Config)
		case *treeFlag:
			bytes, err = util.Marshal(dnsInventory.Tree, *formatFlag, dnsInventory.Config)
		default:
			export := make(map[string][]string)

			// Export the inventory tree into a map.
			switch {
			case *hostsFlag:
				dnsInventory.ExportHosts(export)
			case *groupsFlag:
				dnsInventory.ExportGroups(export)
			}

			bytes, err = util.Marshal(export, *formatFlag, dnsInventory.Config)
		}

		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(bytes))
	} else if len(*hostFlag) > 0 && dnsInventory.Config.GetBool("txt.vars.enabled") {
		// Acquire host TXT records.
		records, err := dnsInventory.GetHostRecords(*hostFlag)
		if err != nil {
			log.Fatal(err)
		}

		// Parse host TXT records.
		attrs := dnsInventory.ParseRecords(records)[*hostFlag]

		// Parse host variables.
		bytes, err := dnsInventory.ParseVariables(attrs)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(string(bytes))
	} else {
		fmt.Println("{}")
	}
}
