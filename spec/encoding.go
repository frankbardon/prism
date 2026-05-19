package spec

import (
	"encoding/json"
	"fmt"
)

// Encoding is the map of channel name → channel binding for a leaf spec.
type Encoding struct {
	X       *PositionChannel `json:"x,omitempty"`
	Y       *PositionChannel `json:"y,omitempty"`
	X2      *PositionChannel `json:"x2,omitempty"`
	Y2      *PositionChannel `json:"y2,omitempty"`
	Theta   *PositionChannel `json:"theta,omitempty"`
	Radius  *PositionChannel `json:"radius,omitempty"`
	Color   *MarkChannel     `json:"color,omitempty"`
	Fill    *MarkChannel     `json:"fill,omitempty"`
	Stroke  *MarkChannel     `json:"stroke,omitempty"`
	Opacity *MarkChannel     `json:"opacity,omitempty"`
	Size    *MarkChannel     `json:"size,omitempty"`
	Shape   *MarkChannel     `json:"shape,omitempty"`
	Text    *TextChannel     `json:"text,omitempty"`
	Tooltip *TooltipChannel  `json:"tooltip,omitempty"`
	Order   *OrderChannel    `json:"order,omitempty"`
	Detail  *DetailChannel   `json:"detail,omitempty"`
	Row     *FacetChannel    `json:"row,omitempty"`
	Column  *FacetChannel    `json:"column,omitempty"`
}

// ChannelCommon holds the fields shared by every channel class.
type ChannelCommon struct {
	Field     string `json:"field,omitempty"`
	Type      string `json:"type,omitempty"`
	Aggregate string `json:"aggregate,omitempty"`
	Scale     *Scale `json:"scale,omitempty"`
	Title     string `json:"title,omitempty"`
	Format    string `json:"format,omitempty"`
	Bin       any    `json:"bin,omitempty"`
	Sort      any    `json:"sort,omitempty"`
	Value     any    `json:"value,omitempty"`
}

// PositionChannel adds axis + stack to ChannelCommon.
type PositionChannel struct {
	ChannelCommon
	Axis  *Axis `json:"axis,omitempty"`
	Stack any   `json:"stack,omitempty"`
}

// MarkChannel adds legend to ChannelCommon.
type MarkChannel struct {
	ChannelCommon
	Legend *Legend `json:"legend,omitempty"`
}

// TextChannel is a slimmer channel for text marks and tooltips.
type TextChannel struct {
	Field     string `json:"field,omitempty"`
	Type      string `json:"type,omitempty"`
	Aggregate string `json:"aggregate,omitempty"`
	Format    string `json:"format,omitempty"`
	Title     string `json:"title,omitempty"`
	Value     any    `json:"value,omitempty"`
}

// TooltipChannel is either a single text channel or an array.
type TooltipChannel struct {
	Single *TextChannel
	Multi  []TextChannel
}

// MarshalJSON emits the single channel or the array.
func (c TooltipChannel) MarshalJSON() ([]byte, error) {
	if c.Multi != nil {
		return json.Marshal(c.Multi)
	}
	if c.Single != nil {
		return json.Marshal(c.Single)
	}
	return []byte("null"), nil
}

// UnmarshalJSON accepts either form.
func (c *TooltipChannel) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		var arr []TextChannel
		if err := json.Unmarshal(data, &arr); err != nil {
			return fmt.Errorf("tooltip: %w", err)
		}
		c.Multi = arr
		return nil
	}
	var single TextChannel
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("tooltip: %w", err)
	}
	c.Single = &single
	return nil
}

// OrderChannelEntry is one element in an order channel.
type OrderChannelEntry struct {
	Field     string `json:"field,omitempty"`
	Type      string `json:"type,omitempty"`
	Aggregate string `json:"aggregate,omitempty"`
	Sort      string `json:"sort,omitempty"`
}

// OrderChannel is either a single entry or an array.
type OrderChannel struct {
	Single *OrderChannelEntry
	Multi  []OrderChannelEntry
}

// MarshalJSON emits a single entry or array.
func (c OrderChannel) MarshalJSON() ([]byte, error) {
	if c.Multi != nil {
		return json.Marshal(c.Multi)
	}
	if c.Single != nil {
		return json.Marshal(c.Single)
	}
	return []byte("null"), nil
}

// UnmarshalJSON accepts either form.
func (c *OrderChannel) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		var arr []OrderChannelEntry
		if err := json.Unmarshal(data, &arr); err != nil {
			return fmt.Errorf("order: %w", err)
		}
		c.Multi = arr
		return nil
	}
	var single OrderChannelEntry
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("order: %w", err)
	}
	c.Single = &single
	return nil
}

// DetailChannelEntry is one detail-channel element.
type DetailChannelEntry struct {
	Field     string `json:"field,omitempty"`
	Type      string `json:"type,omitempty"`
	Aggregate string `json:"aggregate,omitempty"`
}

// DetailChannel is either a single entry or an array.
type DetailChannel struct {
	Single *DetailChannelEntry
	Multi  []DetailChannelEntry
}

// MarshalJSON emits the underlying form.
func (c DetailChannel) MarshalJSON() ([]byte, error) {
	if c.Multi != nil {
		return json.Marshal(c.Multi)
	}
	if c.Single != nil {
		return json.Marshal(c.Single)
	}
	return []byte("null"), nil
}

// UnmarshalJSON accepts either form.
func (c *DetailChannel) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		var arr []DetailChannelEntry
		if err := json.Unmarshal(data, &arr); err != nil {
			return fmt.Errorf("detail: %w", err)
		}
		c.Multi = arr
		return nil
	}
	var single DetailChannelEntry
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("detail: %w", err)
	}
	c.Single = &single
	return nil
}

// FacetChannel binds a field for row/column facetting.
type FacetChannel struct {
	Field  string            `json:"field,omitempty"`
	Type   string            `json:"type,omitempty"`
	Sort   any               `json:"sort,omitempty"`
	Header *FacetChannelHead `json:"header,omitempty"`
}

// FacetChannelHead carries optional header rendering options.
type FacetChannelHead struct {
	Title  string `json:"title,omitempty"`
	Labels *bool  `json:"labels,omitempty"`
}
