package spec

import (
	"encoding/json"
	"fmt"
)

// Dimension is either a numeric pixel value or a token string ("container",
// "step", etc.).
type Dimension struct {
	Number *float64
	Token  string
}

// MarshalJSON emits either the number or the token.
func (d Dimension) MarshalJSON() ([]byte, error) {
	if d.Number != nil {
		return json.Marshal(*d.Number)
	}
	if d.Token != "" {
		return json.Marshal(d.Token)
	}
	return []byte("null"), nil
}

// UnmarshalJSON accepts a number or string.
func (d *Dimension) UnmarshalJSON(data []byte) error {
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		d.Number = &n
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		d.Token = s
		return nil
	}
	return fmt.Errorf("dimension: expected number or string, got %s", string(data))
}

// Padding is either a uniform pixel value or a per-side map.
type Padding struct {
	Uniform *float64
	Top     *float64
	Right   *float64
	Bottom  *float64
	Left    *float64
}

type paddingObj struct {
	Top    *float64 `json:"top,omitempty"`
	Right  *float64 `json:"right,omitempty"`
	Bottom *float64 `json:"bottom,omitempty"`
	Left   *float64 `json:"left,omitempty"`
}

// MarshalJSON emits a uniform number or a per-side object.
func (p Padding) MarshalJSON() ([]byte, error) {
	if p.Uniform != nil {
		return json.Marshal(*p.Uniform)
	}
	return json.Marshal(paddingObj{Top: p.Top, Right: p.Right, Bottom: p.Bottom, Left: p.Left})
}

// UnmarshalJSON accepts a number or a per-side object.
func (p *Padding) UnmarshalJSON(data []byte) error {
	var n float64
	if err := json.Unmarshal(data, &n); err == nil {
		p.Uniform = &n
		return nil
	}
	var obj paddingObj
	if err := json.Unmarshal(data, &obj); err == nil {
		p.Top = obj.Top
		p.Right = obj.Right
		p.Bottom = obj.Bottom
		p.Left = obj.Left
		return nil
	}
	return fmt.Errorf("padding: expected number or object, got %s", string(data))
}

// TextObj carries text properties when the title/subtitle/text encoding is
// declared in object form rather than as a bare string.
type TextObj struct {
	Text       string  `json:"text"`
	Font       string  `json:"font,omitempty"`
	FontSize   float64 `json:"font_size,omitempty"`
	FontWeight any     `json:"font_weight,omitempty"`
	FontStyle  string  `json:"font_style,omitempty"`
	Color      string  `json:"color,omitempty"`
	Align      string  `json:"align,omitempty"`
	Baseline   string  `json:"baseline,omitempty"`
}

// TextOrTextObj is either a bare string or a rich text object.
type TextOrTextObj struct {
	Text *string
	Obj  *TextObj
}

// MarshalJSON emits the underlying string or object.
func (t TextOrTextObj) MarshalJSON() ([]byte, error) {
	if t.Text != nil {
		return json.Marshal(*t.Text)
	}
	if t.Obj != nil {
		return json.Marshal(t.Obj)
	}
	return []byte("null"), nil
}

// UnmarshalJSON accepts a string or rich text object.
func (t *TextOrTextObj) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		t.Text = &s
		return nil
	}
	var obj TextObj
	if err := json.Unmarshal(data, &obj); err == nil {
		t.Obj = &obj
		return nil
	}
	return fmt.Errorf("text: expected string or object, got %s", string(data))
}

// Config is the inline spec-level config block. Values override registered
// theme defaults but lose to inline spec properties.
type Config struct {
	Background string         `json:"background,omitempty"`
	Padding    *Padding       `json:"padding,omitempty"`
	Font       string         `json:"font,omitempty"`
	FontSize   float64        `json:"font_size,omitempty"`
	Color      string         `json:"color,omitempty"`
	Mark       map[string]any `json:"mark,omitempty"`
	Axis       map[string]any `json:"axis,omitempty"`
	Legend     map[string]any `json:"legend,omitempty"`
	Scale      map[string]any `json:"scale,omitempty"`
	Title      map[string]any `json:"title,omitempty"`
}
