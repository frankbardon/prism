package spec

import (
	"encoding/json"
	"fmt"
)

// MarshalJSON emits either the bare type-string shorthand or the full mark
// definition object.
func (m Mark) MarshalJSON() ([]byte, error) {
	if m.Def != nil {
		return json.Marshal(m.Def)
	}
	if m.Shorthand != "" {
		return json.Marshal(m.Shorthand)
	}
	return []byte("null"), nil
}

// UnmarshalJSON accepts either a string ("bar", "line", ...) or a full
// MarkDef object. The discriminator is the JSON token kind.
func (m *Mark) UnmarshalJSON(data []byte) error {
	trimmed := trimSpace(data)
	if len(trimmed) == 0 {
		return fmt.Errorf("mark: empty input")
	}
	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("mark shorthand: %w", err)
		}
		m.Shorthand = s
		return nil
	case '{':
		var def MarkDef
		if err := strictUnmarshal(data, &def); err != nil {
			return fmt.Errorf("mark def: %w", err)
		}
		m.Def = &def
		return nil
	default:
		return fmt.Errorf("mark: expected string or object, got %s", string(trimmed))
	}
}

func trimSpace(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t' || b[0] == '\n' || b[0] == '\r') {
		b = b[1:]
	}
	return b
}
