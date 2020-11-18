package util

import (
	"bytes"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/NeonSludge/ansible-dns-inventory/internal/config"
	"github.com/NeonSludge/ansible-dns-inventory/internal/types"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
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

func MarshalYAMLFlow(v interface{}, mode string, pc *config.Parse) ([]byte, error) {
	buf := new(bytes.Buffer)

	switch v := v.(type) {
	case map[string][]string:
		for key, value := range v {
			var yaml string

			switch mode {
			case "list":
				value = mapStr(value, strconv.Quote)
				yaml = fmt.Sprintf("[%s]", strings.Join(value, ","))
			default:
				yaml = fmt.Sprintf("\"%s\"", strings.Join(value, ","))
			}

			if _, err := buf.WriteString(fmt.Sprintf("\"%s\": %s\n", key, yaml)); err != nil {
				return buf.Bytes(), err
			}
		}
	case map[string]*types.TXTAttrs:
		for key, value := range v {
			yaml := fmt.Sprintf("{\"%s\": \"%s\", \"%s\": \"%s\", \"%s\": \"%s\", \"%s\": \"%s\"}", pc.KeyOs, value.OS, pc.KeyEnv, value.Env, pc.KeyRole, value.Role, pc.KeySrv, value.Srv)

			if _, err := buf.WriteString(fmt.Sprintf("\"%s\": %s\n", key, yaml)); err != nil {
				return buf.Bytes(), err
			}
		}
	default:
		return buf.Bytes(), fmt.Errorf("MarshalYAMLFlow(): unsupported type: %T", v)
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
