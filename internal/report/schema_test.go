package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

func TestReportPassesPublishedJSONSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", "report-v0.1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	report := validReport()
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	if err := validateSchema(schema, schema, value, "$"); err != nil {
		t.Fatal(err)
	}
}

func validateSchema(root, schema, value any, path string) error {
	s, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	if ref, ok := s["$ref"].(string); ok {
		const prefix = "#/$defs/"
		if len(ref) <= len(prefix) || ref[:len(prefix)] != prefix {
			return fmt.Errorf("%s unsupported ref %q", path, ref)
		}
		defs := root.(map[string]any)["$defs"].(map[string]any)
		return validateSchema(root, defs[ref[len(prefix):]], value, path)
	}
	if constant, ok := s["const"]; ok && !reflect.DeepEqual(constant, value) {
		return fmt.Errorf("%s must equal %v", path, constant)
	}
	if options, ok := s["enum"].([]any); ok {
		matched := false
		for _, option := range options {
			matched = matched || reflect.DeepEqual(option, value)
		}
		if !matched {
			return fmt.Errorf("%s value %v is outside enum", path, value)
		}
	}
	if kind, ok := s["type"].(string); ok {
		switch kind {
		case "object":
			object, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("%s is not object", path)
			}
			if required, ok := s["required"].([]any); ok {
				for _, key := range required {
					if _, exists := object[key.(string)]; !exists {
						return fmt.Errorf("%s missing %s", path, key)
					}
				}
			}
			properties, _ := s["properties"].(map[string]any)
			if additional, ok := s["additionalProperties"].(bool); ok && !additional {
				for key := range object {
					if _, exists := properties[key]; !exists {
						return fmt.Errorf("%s has unexpected property %s", path, key)
					}
				}
			}
			for key, sub := range properties {
				if child, exists := object[key]; exists {
					if err := validateSchema(root, sub, child, path+"."+key); err != nil {
						return err
					}
				}
			}
		case "array":
			array, ok := value.([]any)
			if !ok {
				return fmt.Errorf("%s is not array", path)
			}
			if itemSchema, exists := s["items"]; exists {
				for i, item := range array {
					if err := validateSchema(root, itemSchema, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
						return err
					}
				}
			}
		case "string":
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("%s is not string", path)
			}
			if minimum, ok := s["minLength"].(float64); ok && len(text) < int(minimum) {
				return fmt.Errorf("%s is too short", path)
			}
			if pattern, ok := s["pattern"].(string); ok && !regexp.MustCompile(pattern).MatchString(text) {
				return fmt.Errorf("%s does not match %s", path, pattern)
			}
		case "integer":
			number, ok := value.(float64)
			if !ok || number != float64(int64(number)) {
				return fmt.Errorf("%s is not integer", path)
			}
			if minimum, ok := s["minimum"].(float64); ok && number < minimum {
				return fmt.Errorf("%s is below minimum", path)
			}
		}
	}
	return nil
}
