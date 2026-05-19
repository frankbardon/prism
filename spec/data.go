package spec

import (
	"encoding/json"
	"fmt"
)

// Data is the data binding: source path, named ref, or inline values.
// Discriminator: presence of "source", "name", or "values" key.
type Data struct {
	Source string `json:"source,omitempty"`
	Format string `json:"format,omitempty"`
	Name   string `json:"name,omitempty"`

	// Inline-only fields.
	Values []map[string]any `json:"values,omitempty"`
	Fields []FieldSpec      `json:"fields,omitempty"`
}

// FieldSpec optionally types an inline dataset column.
type FieldSpec struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// UnmarshalJSON enforces strict decode and picks the variant from the keys
// present.
func (d *Data) UnmarshalJSON(data []byte) error {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("data: %w", err)
	}
	hasSource := keyPresent(probe, "source")
	hasName := keyPresent(probe, "name")
	hasValues := keyPresent(probe, "values")

	type rawData Data
	var r rawData
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("data: %w", err)
	}
	switch {
	case hasSource:
		// data_source variant — Source must be non-empty after unmarshal.
		*d = Data(r)
	case hasValues:
		*d = Data(r)
	case hasName:
		*d = Data(r)
	default:
		return fmt.Errorf("data: must declare one of source, name, or values")
	}
	return nil
}

func keyPresent(m map[string]json.RawMessage, k string) bool {
	_, ok := m[k]
	return ok
}
