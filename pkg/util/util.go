package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/NeonSludge/ansible-dns-inventory/pkg/inventory"
	"github.com/NeonSludge/ansible-dns-inventory/pkg/types"
)

// Marshal returns the JSON or YAML encoding of v.
func Marshal(v interface{}, format string, cfg types.Config) ([]byte, error) {
	var bytes []byte
	var err error

	switch format {
	case "yaml":
		bytes, err = yaml.Marshal(v)
	case "json":
		bytes, err = json.Marshal(v)
	default:
		bytes, err = marshalYAMLFlow(v, format, cfg)
	}

	if err != nil {
		return bytes, errors.Wrap(err, "marshalling error")
	}

	return bytes, nil
}

// marshalYAMLFlow returns the flow-style YAML encoding of v which can be a map[string][]string or a map[string]*types.TXTAttrs.
// It supports two formats of marshalling the values in the map: as a YAML list (format=yaml-list) and as a CSV string (format=yaml-csv).
// TODO: deal with yaml.Marshal's issues with flow-style encoding and switch to using that instead of this hack.
func marshalYAMLFlow(v interface{}, format string, cfg types.Config) ([]byte, error) {
	buf := new(bytes.Buffer)

	switch v := v.(type) {
	case map[string][]string:
		for key, value := range v {
			var yaml string

			switch format {
			case "yaml-list":
				value = mapStr(value, strconv.Quote)
				yaml = fmt.Sprintf("[%s]", strings.Join(value, ","))
			case "yaml-csv":
				yaml = fmt.Sprintf("\"%s\"", strings.Join(value, ","))
			default:
				return buf.Bytes(), fmt.Errorf("unsupported format: %s", format)
			}

			if _, err := buf.WriteString(fmt.Sprintf("\"%s\": %s\n", key, yaml)); err != nil {
				return buf.Bytes(), err
			}
		}
	case map[string][]*inventory.HostAttributes:
		for key, value := range v {
			var yaml []string
			for _, attrs := range value {
				switch format {
				case "yaml-flow":
					yaml = append(yaml, fmt.Sprintf("{\"%s\": \"%s\", \"%s\": \"%s\", \"%s\": \"%s\", \"%s\": \"%s\", \"%s\": \"%s\"}", cfg.GetString("txt.keys.os"), attrs.OS, cfg.GetString("txt.keys.env"), attrs.Env, cfg.GetString("txt.keys.role"), attrs.Role, cfg.GetString("txt.keys.srv"), attrs.Srv, cfg.GetString("txt.keys.vars"), attrs.Vars))
				default:
					return buf.Bytes(), fmt.Errorf("unsupported format: %s", format)
				}
			}

			if _, err := buf.WriteString(fmt.Sprintf("\"%s\": [%s]\n", key, strings.Join(yaml, ","))); err != nil {
				return buf.Bytes(), err
			}
		}
	default:
		return buf.Bytes(), fmt.Errorf("unsupported format: %s", format)
	}

	return buf.Bytes(), nil
}

// Apply a function to all elements in a slice of strings.
func mapStr(values []string, f func(string) string) []string {
	result := make([]string, len(values))

	for i, value := range values {
		result[i] = f(value)
	}

	return result
}
