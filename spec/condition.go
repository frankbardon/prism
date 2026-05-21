package spec

import (
	"encoding/json"
	"fmt"
)

// Condition is the per-channel conditional encoding clause. JSON form
// accepts either a single test object or an ordered array of test
// objects evaluated in order; the first match wins. The "otherwise"
// branch is supplied by the channel's own `value` (or `field`/`type`)
// at the surrounding ChannelCommon level.
//
// See `.planning/tier1-01-condition-encodings-plan.md` and
// docs/src/concepts/encoding.md (Conditions).
type Condition struct {
	Single *ConditionTest  `json:"-"`
	Multi  []ConditionTest `json:"-"`
}

// ConditionTest is one entry in a condition list. Exactly one of
// {Selection, Test} must be set (enforced by validate rule
// PRISM_SPEC_025/026). Exactly one of {Value, Field} must be set —
// PRISM_SPEC_027 — except that a selection-form entry with no Value
// inherits the channel's own field binding implicitly.
type ConditionTest struct {
	Selection string `json:"selection,omitempty"`
	Test      string `json:"test,omitempty"`
	Field     string `json:"field,omitempty"`
	Type      string `json:"type,omitempty"`
	Value     any    `json:"value,omitempty"`
	Scale     *Scale `json:"scale,omitempty"`
}

// MarshalJSON emits a single object or an array depending on which
// field is populated.
func (c Condition) MarshalJSON() ([]byte, error) {
	if c.Multi != nil {
		return json.Marshal(c.Multi)
	}
	if c.Single != nil {
		return json.Marshal(c.Single)
	}
	return []byte("null"), nil
}

// UnmarshalJSON accepts either an array (Multi) or an object (Single)
// of ConditionTest entries.
func (c *Condition) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	first := firstNonSpace(data)
	switch first {
	case '[':
		var arr []ConditionTest
		if err := json.Unmarshal(data, &arr); err != nil {
			return fmt.Errorf("condition: %w", err)
		}
		c.Multi = arr
		return nil
	case '{':
		var single ConditionTest
		if err := json.Unmarshal(data, &single); err != nil {
			return fmt.Errorf("condition: %w", err)
		}
		c.Single = &single
		return nil
	default:
		return fmt.Errorf("condition: expected object or array, got %q", string(first))
	}
}

// Entries returns the condition list in iteration order regardless of
// whether the spec used the single-object or array form.
func (c *Condition) Entries() []ConditionTest {
	if c == nil {
		return nil
	}
	if c.Multi != nil {
		return c.Multi
	}
	if c.Single != nil {
		return []ConditionTest{*c.Single}
	}
	return nil
}

func firstNonSpace(data []byte) byte {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return b
		}
	}
	return 0
}
