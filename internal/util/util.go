package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/NeonSludge/ansible-dns-inventory/internal/config"
	"github.com/NeonSludge/ansible-dns-inventory/internal/types"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"gopkg.in/yaml.v2"
)

// Validate host attributes.
func SafeAttr(v interface{}, param string) error {
	value := reflect.ValueOf(v)
	if value.Kind() != reflect.String {
		return errors.New("safeAttr() can only validate strings")
	}

	separator := viper.GetString("txt.keys.separator")
	re := "^[A-Za-z0-9"

	// Deprecated: using '-' in group names.
	if separator == "-" {
		re += "\\_"
	}

	switch param {
	case "srv":
		re += "\\,\\" + separator + "]*$"
	case "list":
		re += "\\," + "]*$"
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

// Marshal returns the JSON or YAML encoding of v.
func Marshal(v interface{}, format string, pc *config.Parse) ([]byte, error) {
	var bytes []byte
	var err error

	switch format {
	case "yaml":
		bytes, err = yaml.Marshal(v)
	case "json":
		bytes, err = json.Marshal(v)
	default:
		bytes, err = marshalYAMLFlow(v, format, pc)
	}

	if err != nil {
		return bytes, errors.Wrapf(err, "marshalling error")
	}

	return bytes, nil
}

// marshalYAMLFlow returns the flow-style YAML encoding of v which can be a map[string][]string or a map[string]*types.TXTAttrs.
// It supports two formats of marshalling the values in the map: as a YAML list (format=yaml-list) and as a CSV string (format=yaml-csv).
// TODO: deal with yaml.Marshal's issues with flow-style encoding and switch to using that instead of this hack.
func marshalYAMLFlow(v interface{}, format string, pc *config.Parse) ([]byte, error) {
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
	case map[string]*types.TXTAttrs:
		for key, value := range v {
			var yaml string

			switch format {
			case "yaml-flow":
				yaml = fmt.Sprintf("{\"%s\": \"%s\", \"%s\": \"%s\", \"%s\": \"%s\", \"%s\": \"%s\"}", pc.KeyOs, value.OS, pc.KeyEnv, value.Env, pc.KeyRole, value.Role, pc.KeySrv, value.Srv)
			default:
				return buf.Bytes(), fmt.Errorf("unsupported format: %s", format)
			}

			if _, err := buf.WriteString(fmt.Sprintf("\"%s\": %s\n", key, yaml)); err != nil {
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
