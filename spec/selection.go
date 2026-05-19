package spec

import (
	"encoding/json"
	"fmt"
)

// Selection is point or interval; discriminated by the "type" field.
type Selection struct {
	Point    *PointSelection    `json:"-"`
	Interval *IntervalSelection `json:"-"`
}

// PointSelection captures discrete marks.
type PointSelection struct {
	Type      string   `json:"type"`
	Fields    []string `json:"fields,omitempty"`
	Encodings []string `json:"encodings,omitempty"`
	Toggle    any      `json:"toggle,omitempty"`
	Nearest   *bool    `json:"nearest,omitempty"`
	Empty     string   `json:"empty,omitempty"`
	On        string   `json:"on,omitempty"`
}

// IntervalSelection captures a continuous range over one or more channels.
type IntervalSelection struct {
	Type      string            `json:"type"`
	Encodings []string          `json:"encodings,omitempty"`
	Mark      *IntervalMarkProp `json:"mark,omitempty"`
	Translate any               `json:"translate,omitempty"`
	Zoom      any               `json:"zoom,omitempty"`
	On        string            `json:"on,omitempty"`
}

// IntervalMarkProp is the brush rectangle styling.
type IntervalMarkProp struct {
	Fill        string   `json:"fill,omitempty"`
	FillOpacity *float64 `json:"fill_opacity,omitempty"`
	Stroke      string   `json:"stroke,omitempty"`
	StrokeWidth *float64 `json:"stroke_width,omitempty"`
}

// MarshalJSON emits the underlying selection variant.
func (s Selection) MarshalJSON() ([]byte, error) {
	if s.Point != nil {
		return json.Marshal(s.Point)
	}
	if s.Interval != nil {
		return json.Marshal(s.Interval)
	}
	return []byte("null"), nil
}

// UnmarshalJSON discriminates on the "type" field.
func (s *Selection) UnmarshalJSON(data []byte) error {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return fmt.Errorf("selection: %w", err)
	}
	switch probe.Type {
	case "point":
		var p PointSelection
		if err := json.Unmarshal(data, &p); err != nil {
			return fmt.Errorf("selection point: %w", err)
		}
		s.Point = &p
	case "interval":
		var i IntervalSelection
		if err := json.Unmarshal(data, &i); err != nil {
			return fmt.Errorf("selection interval: %w", err)
		}
		s.Interval = &i
	default:
		return fmt.Errorf("selection: unknown type %q (expected point|interval)", probe.Type)
	}
	return nil
}
